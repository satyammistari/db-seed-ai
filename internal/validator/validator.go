package validator

import (
	"fmt"

	"github.com/satyammistari/db-seed-ai/schema"
)

// ValidateRow checks one row against the table schema.
func ValidateRow(t *schema.Table, row map[string]interface{}) []string {
	var errs []string
	for _, col := range t.Columns {
		v, ok := row[col.Name]
		if !ok {
			if col.NotNull {
				errs = append(errs, fmt.Sprintf("%s: NOT NULL but missing", col.Name))
			}
			continue
		}
		if v == nil {
			if col.NotNull {
				errs = append(errs, fmt.Sprintf("%s: NOT NULL but got nil", col.Name))
			}
			continue
		}
		if len(col.CheckIn) > 0 {
			found := false
			s := fmt.Sprint(v)
			for _, allowed := range col.CheckIn {
				if s == allowed {
					found = true
					break
				}
			}
			if !found {
				errs = append(errs, fmt.Sprintf("%s: value %q not in %v", col.Name, v, col.CheckIn))
			}
		}
		// Type sanity (optional): we could check number/string format
	}
	return errs
}

// ValidateRows runs ValidateRow on each row and returns all errors.
func ValidateRows(t *schema.Table, rows []map[string]interface{}) []string {
	var errs []string
	for i, row := range rows {
		rowErrs := ValidateRow(t, row)
		for _, e := range rowErrs {
			errs = append(errs, fmt.Sprintf("row %d: %s", i+1, e))
		}
	}
	return errs
}
