package schema

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
