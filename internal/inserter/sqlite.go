package inserter

import (
	"database/sql"
	"fmt"
	"strings"

	// The underscore import loads the sqlite3 driver
	// without us using it directly by name.
	_ "github.com/mattn/go-sqlite3"
	"github.com/satyammistari/db-seed-ai/internal/generator"
	"github.com/satyammistari/db-seed-ai/internal/schema"
)

// SQLiteInserter holds the connection to SQLite.
// SQLite = single file database, no server needed.
// Perfect for local dev and testing.
type SQLiteInserter struct {
	db *sql.DB
}

// NewSQLite opens (or creates) a SQLite database file.
// path example: "./dev.db"
// If dev.db doesn't exist, SQLite creates it automatically.
func NewSQLite(path string) (*SQLiteInserter, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf(
			"ping sqlite: %w\nCheck that the path is writable: %s",
			err, path,
		)
	}
	// SQLite ignores FK constraints by default — turn enforcement ON.
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	return &SQLiteInserter{db: db}, nil
}

// Insert puts all rows into SQLite in batches.
// Same signature as PostgresInserter.Insert() because both implement Inserter.
func (s *SQLiteInserter) Insert(
	result *generator.GenerationResult,
	table *schema.Table,
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
			return total, fmt.Errorf("sqlite batch at row %d: %w", i+1, err)
		}
		total += n
	}
	return total, nil
}

// insertBatch inserts one chunk using a prepared statement.
// SQLite uses ? placeholders; we insert one row at a time inside a transaction.
func (s *SQLiteInserter) insertBatch(
	result *generator.GenerationResult,
	rows []map[string]interface{},
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

	sqlStr := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		result.TableName,
		strings.Join(cols, ", "),
		strings.Join(ph, ", "),
	)

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(sqlStr)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w\nSQL: %s", err, sqlStr)
	}
	defer stmt.Close()

	count := 0
	for _, row := range rows {
		args := make([]interface{}, len(cols))
		for i, col := range cols {
			args[i] = row[col]
		}
		if _, err := stmt.Exec(args...); err != nil {
			return count, fmt.Errorf("insert row %d: %w", count+1, err)
		}
		count++
	}
	return count, tx.Commit()
}

// FetchExistingIDs gets real PK values from SQLite.
// Called before generating FK-dependent rows.
func (s *SQLiteInserter) FetchExistingIDs(
	tableName string,
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
		if err := rows.Scan(&v); err != nil {
			continue
		}
		ids = append(ids, v)
	}
	return ids, nil
}

// Close shuts down the SQLite connection.
func (s *SQLiteInserter) Close() error {
	return s.db.Close()
}

// Compile-time interface check: build fails if SQLiteInserter is missing any method.
var _ Inserter = (*SQLiteInserter)(nil)


