package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/satyammistari/db-seed-ai/internal/generator"
	"github.com/satyammistari/db-seed-ai/internal/reporter"
	"github.com/satyammistari/db-seed-ai/internal/schema"
	"github.com/satyammistari/db-seed-ai/internal/validator"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Generate and validate data without inserting",
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringP("table", "t", "", "Specific table to validate")
	validateCmd.Flags().IntP("rows", "r", 10, "Rows to generate and validate")
}

func runValidate(cmd *cobra.Command, args []string) error {
	schemaPath, _ := rootCmd.PersistentFlags().GetString("schema")
	tableName, _  := cmd.Flags().GetString("table")
	numRows, _    := cmd.Flags().GetInt("rows")
	model, _      := rootCmd.PersistentFlags().GetString("model")
	style, _      := rootCmd.PersistentFlags().GetString("style")

	if schemaPath == "" {
		return fmt.Errorf("--schema is required")
	}

	reporter.PrintBanner()

	p, _ := schema.NewParser(schemaPath)
	s, err := p.Parse()
	if err != nil {
		return err
	}

	gen := generator.New(model)
	if err := gen.Ping(); err != nil {
		return err
	}

	tables := s.InsertOrder
	if tableName != "" {
		tables = []string{tableName}
	}

	allPassed := true
	for _, name := range tables {
		t := s.TableMap[name]
		if t == nil {
			continue
		}
		result, err := gen.Generate(t, numRows, s, style, map[string][]interface{}{})
		if err != nil {
			color.Red("✗ %s: %v\n", name, err)
			allPassed = false
			continue
		}
		errs := validator.Validate(result.Rows, t)
		if len(errs) == 0 {
			color.Green("✓ %s: %d rows valid\n", name, numRows)
		} else {
			color.Red("✗ %s: %d violations\n", name, len(errs))
			for _, e := range errs {
				fmt.Printf("  • %v\n", e)
			}
			allPassed = false
		}
	}

	fmt.Println()
	if allPassed {
		color.Green("All tables passed validation.\n")
	} else {
		return fmt.Errorf("validation failed")
	}
	return nil
}