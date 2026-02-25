package inserter

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/satyammistari/db-seed-ai/internal/generator"
	"github.com/satyammistari/db-seed-ai/internal/schema"
)

// Inserter is the interface both SQLiteInserter and PostgresInserter implement.
type Inserter interface {
	Insert(result *generator.GenerationResult, table *schema.Table, batchSize int) (int, error)
	FetchExistingIDs(tableName, columnName string) ([]interface{}, error)
	Close() error
}

// PostgresInserter holds a connection to a PostgreSQL database.
// Uses the pgx driver registered under the "pgx" name via pgx/v5/stdlib.
type PostgresInserter struct {
	db *sql.DB
}

// NewPostgres opens a PostgreSQL connection from a DSN string.
// DSN format: postgres://user:pass@host:port/dbname or postgresql://...
func NewPostgres(dsn string) (*PostgresInserter, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres %s: %w", dsn, err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf(
			"ping postgres: %w\n"+
				"Check that the server is running and the DSN is correct: %s",
			err, dsn,
		)
	}
	return &PostgresInserter{db: db}, nil
}

// Insert puts all rows into PostgreSQL in batches.
// Uses multi-row INSERT: INSERT INTO t (a,b) VALUES ($1,$2),($3,$4),...
func (p *PostgresInserter) Insert(
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

		n, err := p.insertBatch(result, chunk)
		if err != nil {
			return total, fmt.Errorf("postgres batch at row %d: %w", i+1, err)
		}
		total += n
	}
	return total, nil
}

// insertBatch inserts one chunk using a prepared statement and $N placeholders.
// PostgreSQL supports multi-row VALUES, so we build one big INSERT per batch.
func (p *PostgresInserter) insertBatch(
	result *generator.GenerationResult,
	rows []map[string]interface{},
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	cols := result.Columns

	// Build placeholders: ($1,$2,...),($3,$4,...) for multi-row insert
	var valueSets []string
	idx := 1
	for range rows {
		var ph []string
		for range cols {
			ph = append(ph, fmt.Sprintf("$%d", idx))
			idx++
		}
		valueSets = append(valueSets, "("+strings.Join(ph, ", ")+")")
	}

	// Quote column names to handle reserved words and mixed case
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = `"` + strings.ReplaceAll(c, `"`, `""`) + `"`
	}

	sqlStr := fmt.Sprintf(
		`INSERT INTO "%s" (%s) VALUES %s`,
		strings.ReplaceAll(result.TableName, `"`, `""`),
		strings.Join(quotedCols, ", "),
		strings.Join(valueSets, ", "),
	)

	// Wrap everything in a transaction so all rows succeed or all roll back
	tx, err := p.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Flatten row maps into positional args
	args := make([]interface{}, 0, len(rows)*len(cols))
	for _, row := range rows {
		for _, col := range cols {
			args = append(args, row[col])
		}
	}

	if _, err := tx.Exec(sqlStr, args...); err != nil {
		return 0, fmt.Errorf(
			"exec insert: %w\nSQL: %s",
			err, sqlStr,
		)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	return len(rows), nil
}

// FetchExistingIDs returns up to 1000 values from tableName.columnName.
// Used to gather FK reference values before generating dependent tables.
func (p *PostgresInserter) FetchExistingIDs(
	tableName string,
	columnName string,
) ([]interface{}, error) {
	query := fmt.Sprintf(
		`SELECT "%s" FROM "%s" LIMIT 1000`,
		strings.ReplaceAll(columnName, `"`, `""`),
		strings.ReplaceAll(tableName, `"`, `""`),
	)
	rows, err := p.db.Query(query)
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
	return ids, rows.Err()
}

// Close shuts down the PostgreSQL connection pool.
func (p *PostgresInserter) Close() error {
	return p.db.Close()
}

// Compile-time interface check: build fails if PostgresInserter is missing methods.
var _ Inserter = (*PostgresInserter)(nil)


