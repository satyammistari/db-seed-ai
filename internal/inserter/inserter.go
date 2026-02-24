package inserter

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

// Open opens a database from a connection string.
// Formats: "postgres://...", "postgresql://...", "sqlite:path" or "sqlite://path"
// Returns db and driver name ("pgx" or "sqlite3") for placeholder style in inserts.
func Open(conn string) (*sql.DB, string, error) {
	driver, dsn := parseConn(conn)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, "", err
	}
	return db, driver, nil
}

func parseConn(conn string) (driver, dsn string) {
	if strings.HasPrefix(conn, "sqlite:") {
		return "sqlite3", strings.TrimPrefix(conn, "sqlite:")
	}
	if strings.HasPrefix(conn, "sqlite://") {
		return "sqlite3", strings.TrimPrefix(conn, "sqlite://")
	}
	if strings.HasPrefix(conn, "postgres://") || strings.HasPrefix(conn, "postgresql://") {
		return "pgx", conn
	}
	return "pgx", conn
}

// FetchRefIDs returns existing values for a table.column (e.g. for FK context).
func FetchRefIDs(db *sql.DB, table, column string, limit int) ([]interface{}, error) {
	query := fmt.Sprintf("SELECT %s FROM %s LIMIT %d", quoteIdent(column), quoteIdent(table), limit)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []interface{}
	for rows.Next() {
		var v interface{}
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		ids = append(ids, v)
	}
	return ids, rows.Err()
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// InsertBatch inserts rows in a single transaction. Each row is a map of column name -> value.
// driverName is "pgx" for PostgreSQL ($1, $2) or "sqlite3" for SQLite (?).
func InsertBatch(db *sql.DB, driverName, table string, columns []string, rows []map[string]interface{}) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	placeholders := buildPlaceholders(driverName, len(columns), len(rows))
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
		quoteIdent(table),
		quotedList(columns),
		placeholders,
	)
	stmt, err := tx.Prepare(query)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	args := flattenArgs(columns, rows)
	_, err = stmt.Exec(args...)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func buildPlaceholders(driverName string, numCols, numRows int) string {
	var parts []string
	idx := 0
	for i := 0; i < numRows; i++ {
		var placeholders []string
		for j := 0; j < numCols; j++ {
			idx++
			if driverName == "sqlite3" {
				placeholders = append(placeholders, "?")
			} else {
				placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
			}
		}
		parts = append(parts, "("+strings.Join(placeholders, ",")+")")
	}
	return strings.Join(parts, ",")
}

func quotedList(cols []string) string {
	var q []string
	for _, c := range cols {
		q = append(q, quoteIdent(c))
	}
	return strings.Join(q, ",")
}

func flattenArgs(columns []string, rows []map[string]interface{}) []interface{} {
	var args []interface{}
	for _, row := range rows {
		for _, col := range columns {
			args = append(args, row[col])
		}
	}
	return args
}
