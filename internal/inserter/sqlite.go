package inserter

import (
	"database/sql"
	"fmt"
	"strings"

	// The underscore import loads the sqlite3 driver
	// without us using it directly by name
	// Go's database/sql needs a driver registered
	// this import registers "sqlite3" as a driver name
	_ "github.com/mattn/go-sqlite3"
	"github.com/satyammistari/seeddb/internal/generator"
	"github.com/satyammistari/seeddb/internal/schema"
)

// SQLiteInserter holds the connection to SQLite
// SQLite = single file database, no server needed
// Perfect for local dev and testing
type SQLiteInserter struct {
	db *sql.DB
}

// NewSQLite opens (or creates) a SQLite database file
// path example: "./dev.db"
// If dev.db doesn't exist, SQLite creates it automatically
func NewSQLite(path string) (*SQLiteInserter, error) {
	// sql.Open doesn't actually connect yet
	// it just prepares the connection configuration
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf(
			"open sqlite %s: %w", path, err,
		)
	}

	// Ping actually tests the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf(
			"ping sqlite: %w\n"+
				"Check that the path is writable: %s",
			err, path,
		)
	}

	// SQLite ignores FK constraints by default
	// This PRAGMA turns FK enforcement ON
	// Without this, bad FK values silently succeed
	if _, err := db.Exec(
		"PRAGMA foreign_keys = ON",
	); err != nil {
		return nil, fmt.Errorf(
			"enable foreign keys: %w", err,
		)
	}

	return &SQLiteInserter{db: db}, nil
}

// Insert puts all rows into SQLite in batches
// Same signature as PostgresInserter.Insert()
// because both implement the Inserter interface
func (s *SQLiteInserter) Insert(
	result    *generator.GenerationResult,
	table     *schema.Table,
	batchSize int,
) (int, error) {
	total := 0

	for i := 0; i < len(result.Rows); i += batchSize {
		end := i + batchSize
		if end > len(result.Rows) {
			end = len(result.Rows)
		}
		chunk := result.Rows[i:end]

		n, err := s.insertBatch(result, chunk)
		if err != nil {
			return total, fmt.Errorf(
				"sqlite batch at row %d: %w",
				i+1, err,
			)
		}
		total += n
	}

	return total, nil
}

// insertBatch inserts one chunk using a prepared statement
// SQLite difference from Postgres:
//   Postgres: INSERT INTO t VALUES ($1,$2),($3,$4)
//   SQLite:   INSERT INTO t VALUES (?,?) — one row at a time
//             SQLite doesn't support multi-row VALUES well
//
// Prepared statement = SQL parsed once, executed many times
// Much faster than parsing SQL for every single row
func (s *SQLiteInserter) insertBatch(
	result *generator.GenerationResult,
	rows   []map[string]interface{},
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	cols := result.Columns

	// Build placeholders: (?, ?, ?) for each column
	ph := make([]string, len(cols))
	for i := range cols {
		ph[i] = "?"
	}

	// SQLite INSERT — one row at a time using ?
	sqlStr := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		result.TableName,
		strings.Join(cols, ", "),
		strings.Join(ph, ", "),
	)

	// BEGIN transaction — wrap all rows in one transaction
	// Same reason as Postgres: all succeed or all roll back
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf(
			"begin transaction: %w", err,
		)
	}
	defer tx.Rollback()

	// Prepare the statement ONCE inside the transaction
	// This is the key performance optimization:
	// SQLite parses and plans the SQL only one time
	// then we execute it once per row
	stmt, err := tx.Prepare(sqlStr)
	if err != nil {
		return 0, fmt.Errorf(
			"prepare statement: %w\nSQL: %s",
			err, sqlStr,
		)
	}
	// Close the prepared statement when done
	defer stmt.Close()

	count := 0
	for _, row := range rows {
		// Build args in column order
		// row is map[colName]value — maps have no order
		// so we extract values in cols order explicitly
		args := make([]interface{}, len(cols))
		for i, col := range cols {
			args[i] = row[col]
		}

		// Execute the prepared statement with this row's values
		if _, err := stmt.Exec(args...); err != nil {
			// Rollback happens via defer above
			return count, fmt.Errorf(
				"insert row %d: %w", count+1, err,
			)
		}
		count++
	}

	// COMMIT all rows at once
	return count, tx.Commit()
}

// FetchExistingIDs gets real PK values from SQLite
// Same purpose as Postgres version —
// called before generating FK-dependent rows
func (s *SQLiteInserter) FetchExistingIDs(
	tableName  string,
	columnName string,
) ([]interface{}, error) {
	rows, err := s.db.Query(fmt.Sprintf(
		"SELECT %s FROM %s LIMIT 1000",
		columnName, tableName,
	))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []interface{}
	for rows.Next() {
		var v interface{}
		// Scan reads one value from the current row
		if err := rows.Scan(&v); err != nil {
			continue
		}
		ids = append(ids, v)
	}

	return ids, nil
}

// Close shuts down the SQLite connection
func (s *SQLiteInserter) Close() error {
	return s.db.Close()
}

// Compile-time interface check
// Build fails here if SQLiteInserter is missing any method
var _ Inserter = (*SQLiteInserter)(nil)
```

---

## How All 4 Files Connect Together
```
You run:  db-seed-ai seed --schema ecommerce.sql --db sqlite:./dev.db

cmd/seed.go
    │
    ├── calls schema.NewParser() → reads your .sql file
    │
    ├── calls inserter.New("sqlite:./dev.db")
    │       └── sqlite.go: NewSQLite() → opens dev.db
    │
    ├── calls generator.New("deepseek-r1:7b")
    │       └── ollama.go: NewOllamaClient() → sets up HTTP client
    │
    └── for each table:
            │
            ├── sqlite.go: FetchExistingIDs()
            │     └── SELECT id FROM users → [1,2,3,4,5]
            │
            ├── prompt.go: BuildPrompt()
            │     └── "Generate 50 rows for orders table.
            │          user_id must be one of: 1,2,3,4,5"
            │
            ├── ollama.go: Generate(prompt)
            │     └── POST localhost:11434/api/generate
            │     └── strips <think> tags from DeepSeek
            │     └── returns JSON string
            │
            └── sqlite.go: Insert(rows)
                  └── BEGIN
                  └── INSERT INTO orders ... (500 rows)
                  └── COMMIT