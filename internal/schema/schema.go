package schema

// Schema wraps a slice of tables for historical API compatibility.
// prompt.go uses *schema.Schema to pass the full set of tables.
type Schema struct {
	Tables []*Table
}

// Table represents a parsed database table.
type Table struct {
	Name    string
	Columns []Column
}

// Column represents a table column with constraints.
type Column struct {
	Name       string
	Type       string   // normalized: integer, text, decimal, timestamp, boolean
	NotNull    bool
	Unique     bool
	PrimaryKey bool
	CheckIn    []string // allowed values from CHECK (col IN (...))
	ForeignKey *ForeignKey
}

// DataType is an alias accessor for Type, used by prompt.go.
func (c Column) GetDataType() string { return c.Type }

// ForeignKey describes a reference to another table.
type ForeignKey struct {
	RefTable  string
	RefColumn string
}

// DependsOn returns table names this table's FKs reference (for topological sort).
func (t *Table) DependsOn() []string {
	var out []string
	seen := make(map[string]bool)
	for _, c := range t.Columns {
		if c.ForeignKey != nil && !seen[c.ForeignKey.RefTable] {
			seen[c.ForeignKey.RefTable] = true
			out = append(out, c.ForeignKey.RefTable)
		}
	}
	return out
}

// NonAutoColumns returns all columns that are not auto-generated serial PKs.
// Columns named "id" with type "integer" and PrimaryKey=true are skipped.
func (t *Table) NonAutoColumns() []Column {
	var out []Column
	for _, c := range t.Columns {
		if c.PrimaryKey && c.Type == "integer" {
			continue
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return t.Columns
	}
	return out
}
