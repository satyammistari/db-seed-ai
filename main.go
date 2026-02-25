package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/satyammistari/db-seed-ai/internal/generator"
	"github.com/satyammistari/db-seed-ai/internal/inserter"
	"github.com/satyammistari/db-seed-ai/internal/reporter"
	"github.com/satyammistari/db-seed-ai/internal/schema"
	"github.com/satyammistari/db-seed-ai/internal/tui"
	"github.com/satyammistari/db-seed-ai/internal/validator"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	switch cmd {
	case "ui":
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "UI error: %v\n", err)
			os.Exit(1)
		}
	case "preview":
		runPreview(os.Args[2:])
	case "seed":
		runSeed(os.Args[2:])
	case "validate":
		runValidate(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("db-seed-ai v" + version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `db-seed-ai — generate seed data with local AI

Usage:
  seeddb ui                                    Launch interactive terminal UI
  seeddb preview  --schema <file> [--table <name>] [--rows N] [--model M]
  seeddb seed     --schema <file> --db <conn> [--table <name>] [--rows N] [--dry-run] [--model M] [--batch-size N] [--style S]
  seeddb validate --schema <file> [--rows N]

Commands:
  ui        Launch interactive terminal UI (recommended)
  preview   Show generated rows (no DB)
  seed      Generate and insert into database
  validate  Generate sample and validate constraints
  help      Show this help message
  version   Show version information
`)
}

func loadSchema(path string) ([]*schema.Table, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("schema file: %w", err)
	}
	return schema.ParseFile(string(data))
}

func runPreview(args []string) {
	fs := flag.NewFlagSet("preview", flag.ExitOnError)
	schemaPath := fs.String("schema", "", "Path to .sql schema file")
	tableName := fs.String("table", "", "Only this table (required for preview)")
	rows := fs.Int("rows", 5, "Number of rows")
	model := fs.String("model", "llama3", "Ollama model")
	style := fs.String("style", "realistic", "realistic, minimal, edge-cases")
	_ = fs.Parse(args)

	if *schemaPath == "" || *tableName == "" {
		fmt.Fprintln(os.Stderr, "preview requires --schema and --table")
		fs.PrintDefaults()
		os.Exit(1)
	}

	tables, err := loadSchema(*schemaPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	t := schema.TableByName(tables, *tableName)
	if t == nil {
		fmt.Fprintf(os.Stderr, "table %q not found in schema\n", *tableName)
		os.Exit(1)
	}

	cfg := generator.DefaultConfig()
	cfg.Model = *model
	cfg.Style = generator.Style(*style)

	reporter.Info(fmt.Sprintf("  Asking %s to generate %d rows...\n", cfg.Model, *rows))
	prompt := generator.BuildPrompt(t, *rows, nil, string(cfg.Style), nil)
	raw, err := generator.CallOllama(cfg, prompt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ollama error:", err)
		os.Exit(1)
	}
	colNames := columnNames(t)
	parsed, err := generator.ParseJSONRows(raw, colNames)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Parse error:", err)
		fmt.Fprintln(os.Stderr, "Raw response:", raw)
		os.Exit(1)
	}
	reporter.Info("")
	reporter.Table(colNames, parsed)
	reporter.Info("\n  Dry run — no data was inserted.")
}

func runSeed(args []string) {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	schemaPath := fs.String("schema", "", "Path to .sql schema file")
	dbConn := fs.String("db", "", "Database connection string")
	tableName := fs.String("table", "", "Only this table (default: all)")
	rows := fs.Int("rows", 100, "Rows per table")
	dryRun := fs.Bool("dry-run", false, "Generate but do not insert")
	model := fs.String("model", "llama3", "Ollama model")
	style := fs.String("style", "realistic", "realistic, minimal, edge-cases")
	batchSize := fs.Int("batch-size", 500, "Rows per INSERT batch")
	_ = fs.Parse(args)

	if *schemaPath == "" {
		fmt.Fprintln(os.Stderr, "seed requires --schema")
		fs.PrintDefaults()
		os.Exit(1)
	}
	if !*dryRun && *dbConn == "" {
		fmt.Fprintln(os.Stderr, "seed requires --db (or use --dry-run)")
		fs.PrintDefaults()
		os.Exit(1)
	}

	tables, err := loadSchema(*schemaPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	order := tables
	if *tableName != "" {
		t := schema.TableByName(tables, *tableName)
		if t == nil {
			fmt.Fprintf(os.Stderr, "table %q not found\n", *tableName)
			os.Exit(1)
		}
		order = []*schema.Table{t}
	}

	reporter.Info("db-seed-ai v" + version)
	reporter.Info(fmt.Sprintf("Schema loaded:  %d tables", len(tables)))
	var orderNames []string
	for _, t := range order {
		orderNames = append(orderNames, t.Name)
	}
	reporter.Info("Insert order:   " + joinNames(orderNames))
	reporter.Info("AI model: " + *model)
	reporter.Info("")

	cfg := generator.DefaultConfig()
	cfg.Model = *model
	cfg.Style = generator.Style(*style)

	var dbObj *sql.DB
	var driver string
	if *dbConn != "" && !*dryRun {
		var err error
		dbObj, driver, err = inserter.Open(*dbConn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "db open:", err)
			os.Exit(1)
		}
		defer dbObj.Close()
	}

	reporter.Info("Generating seed data...")
	totalInserted := 0
	insertHeaderDone := false
	for _, t := range order {
		// Build ref IDs from already-inserted tables (so FKs reference real rows)
		refIDs := make(map[string][]interface{})
		if dbObj != nil {
			for _, c := range t.Columns {
				if c.ForeignKey != nil {
					key := c.ForeignKey.RefTable + "." + c.ForeignKey.RefColumn
					ids, err := inserter.FetchRefIDs(dbObj, c.ForeignKey.RefTable, c.ForeignKey.RefColumn, 1000)
					if err == nil && len(ids) > 0 {
						refIDs[key] = ids
					}
				}
			}
		}

		prompt := generator.BuildPrompt(t, *rows, nil, string(cfg.Style), refIDs)
		raw, err := generator.CallOllama(cfg, prompt)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Ollama:", err)
			os.Exit(1)
		}
		colNames := columnNames(t)
		parsed, err := generator.ParseJSONRows(raw, colNames)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Parse:", err)
			os.Exit(1)
		}
		reporter.Ok(fmt.Sprintf("%-20s %d rows", t.Name, len(parsed)))

		if *dryRun || dbObj == nil {
			continue
		}
		if !insertHeaderDone {
			reporter.Info("\nInserting into database...")
			insertHeaderDone = true
		}
		inserted := 0
		for i := 0; i < len(parsed); i += *batchSize {
			end := i + *batchSize
			if end > len(parsed) {
				end = len(parsed)
			}
			batch := parsed[i:end]
			n, err := inserter.InsertBatch(dbObj, driver, t.Name, colNames, batch)
			if err != nil {
				reporter.Err(fmt.Sprintf("%s: %v", t.Name, err))
				os.Exit(1)
			}
			inserted += n
		}
		totalInserted += inserted
		reporter.Ok(fmt.Sprintf("%-20s %d inserted", t.Name, inserted))
	}

	if *dryRun {
		reporter.Info("\nDry run — no data inserted.")
		return
	}
	reporter.Info("")
	reporter.Ok(fmt.Sprintf("Done — %d rows inserted across %d tables", totalInserted, len(order)))
}

func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	schemaPath := fs.String("schema", "", "Path to .sql schema file")
	rows := fs.Int("rows", 10, "Sample rows to generate")
	model := fs.String("model", "llama3", "Ollama model")
	_ = fs.Parse(args)

	if *schemaPath == "" {
		fmt.Fprintln(os.Stderr, "validate requires --schema")
		fs.PrintDefaults()
		os.Exit(1)
	}

	tables, err := loadSchema(*schemaPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cfg := generator.DefaultConfig()
	cfg.Model = *model
	var allErrs []string
	for _, t := range tables {
		prompt := generator.BuildPrompt(t, *rows, nil, string(generator.StyleRealistic), nil)
		raw, err := generator.CallOllama(cfg, prompt)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Ollama:", err)
			os.Exit(1)
		}
		colNames := columnNames(t)
		parsed, err := generator.ParseJSONRows(raw, colNames)
		if err != nil {
			allErrs = append(allErrs, t.Name+": parse error - "+err.Error())
			continue
		}
		errs := validator.ValidateRows(t, parsed)
		for _, e := range errs {
			allErrs = append(allErrs, t.Name+": "+e)
		}
	}
	if len(allErrs) > 0 {
		for _, e := range allErrs {
			reporter.Err(e)
		}
		os.Exit(1)
	}
	reporter.Ok("All generated rows passed validation")
}

func columnNames(t *schema.Table) []string {
	var out []string
	for _, c := range t.Columns {
		out = append(out, c.Name)
	}
	return out
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, " → ")
}


