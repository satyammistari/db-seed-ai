package schema

import (
	"bufio"
	"regexp"
	"strings"
)

// ParseFile reads a SQL file and returns tables in dependency order (topological sort).
func ParseFile(content string) ([]*Table, error) {
	tables := parseTables(content)
	return topologicalSort(tables), nil
}

func parseTables(content string) []*Table {
	var tables []*Table
	// Normalize: single line per statement for simpler parsing
	content = normalizeSQL(content)
	// Split by CREATE TABLE (Go regexp has no (?:), so we use two groups)
	re := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(IF\s+NOT\s+EXISTS\s+)?["']?(\w+)["']?\s*\(`)
	matches := re.FindAllStringSubmatchIndex(content, -1)
	for i, loc := range matches {
		tableName := content[loc[4]:loc[5]]
		start := loc[0]
		var end int
		if i+1 < len(matches) {
			end = matches[i+1][0]
		} else {
			end = len(content)
		}
		body := content[start:end]
		// Find matching closing paren for CREATE TABLE (
		body = extractParenBlock(body)
		t := parseTableBody(tableName, body)
		if t != nil {
			tables = append(tables, t)
		}
	}
	return tables
}

func normalizeSQL(s string) string {
	var b strings.Builder
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		b.WriteString(" ")
		b.WriteString(line)
	}
	return b.String()
}

func extractParenBlock(s string) string {
	start := strings.Index(s, "(")
	if start == -1 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[start+1 : i]
			}
		}
	}
	return s[start+1:]
}

func parseTableBody(tableName, body string) *Table {
	t := &Table{Name: tableName}
	// Parse column and constraint lines (comma-separated, respecting parens)
	parts := splitTopLevel(body, ',')
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// CONSTRAINT name ... or column def
		if strings.HasPrefix(strings.ToUpper(p), "CONSTRAINT ") {
			// Parse FK or CHECK that references our columns
			applyTableConstraint(t, p)
			continue
		}
		if strings.HasPrefix(strings.ToUpper(p), "PRIMARY KEY") {
			applyPrimaryKey(t, p)
			continue
		}
		if strings.HasPrefix(strings.ToUpper(p), "FOREIGN KEY") {
			applyForeignKey(t, p)
			continue
		}
		// Column definition
		col := parseColumnDef(p)
		if col != nil {
			t.Columns = append(t.Columns, *col)
		}
	}
	return t
}

func splitTopLevel(s string, sep byte) []string {
	var parts []string
	var cur strings.Builder
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '(':
			depth++
			cur.WriteByte(c)
		case ')':
			depth--
			cur.WriteByte(c)
		case sep:
			if depth == 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			} else {
				cur.WriteByte(c)
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

var colDefRe = regexp.MustCompile(`(?i)^["']?(\w+)["']?\s+(\w+)(\s*\([^)]*\))?`)

func parseColumnDef(s string) *Column {
	col := &Column{}
	upper := strings.ToUpper(s)
	col.NotNull = strings.Contains(upper, "NOT NULL")
	col.Unique = strings.Contains(upper, "UNIQUE")
	// PRIMARY KEY in column def
	if strings.Contains(upper, "PRIMARY KEY") {
		col.PrimaryKey = true
	}
	// CHECK (col IN ('a','b'))
	checkIn := regexp.MustCompile(`(?i)CHECK\s*\(\s*\w+\s+IN\s*\(([^)]+)\)`)
	if m := checkIn.FindStringSubmatch(s); len(m) > 1 {
		col.CheckIn = parseQuotedList(m[1])
	}
	// REFERENCES other(col)
	refRe := regexp.MustCompile(`(?i)REFERENCES\s+["']?(\w+)["']?\s*\(\s*["']?(\w+)["']?\s*\)`)
	if m := refRe.FindStringSubmatch(s); len(m) >= 3 {
		col.ForeignKey = &ForeignKey{RefTable: m[1], RefColumn: m[2]}
	}
	// Name and type
	idx := colDefRe.FindStringSubmatchIndex(s)
	if idx == nil {
		return nil
	}
	col.Name = s[idx[2]:idx[3]]
	typePart := strings.TrimSpace(s[idx[4]:idx[5]])
	if len(idx) > 6 && idx[6] >= 0 {
		typePart += strings.TrimSpace(s[idx[6]:idx[7]])
	}
	col.Type = normalizeType(typePart)
	return col
}

func parseQuotedList(s string) []string {
	var out []string
	// 'a', 'b', 'c'
	re := regexp.MustCompile(`'([^']*)'|"([^"]*)"`)
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		if m[1] != "" {
			out = append(out, m[1])
		} else if m[2] != "" {
			out = append(out, m[2])
		}
	}
	return out
}

func normalizeType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	// varchar(n), char(n) -> text
	if strings.HasPrefix(t, "varchar") || strings.HasPrefix(t, "char") || t == "text" || strings.HasPrefix(t, "character") {
		return "text"
	}
	if strings.HasPrefix(t, "int") || t == "serial" || strings.HasPrefix(t, "bigserial") || strings.HasPrefix(t, "smallint") {
		return "integer"
	}
	if strings.HasPrefix(t, "decimal") || strings.HasPrefix(t, "numeric") || strings.HasPrefix(t, "real") || strings.HasPrefix(t, "double") || t == "float" {
		return "decimal"
	}
	if strings.Contains(t, "timestamp") || strings.Contains(t, "date") || t == "datetime" {
		return "timestamp"
	}
	if t == "bool" || strings.HasPrefix(t, "boolean") {
		return "boolean"
	}
	return "text"
}

func applyTableConstraint(t *Table, s string) {
	// FOREIGN KEY (col) REFERENCES other(col)
	fkRe := regexp.MustCompile(`(?i)FOREIGN\s+KEY\s*\(\s*["']?(\w+)["']?\s*\)\s+REFERENCES\s+["']?(\w+)["']?\s*\(\s*["']?(\w+)["']?\s*\)`)
	if m := fkRe.FindStringSubmatch(s); len(m) >= 4 {
		for i := range t.Columns {
			if t.Columns[i].Name == m[1] {
				t.Columns[i].ForeignKey = &ForeignKey{RefTable: m[2], RefColumn: m[3]}
				break
			}
		}
	}
}

func applyPrimaryKey(t *Table, s string) {
	// PRIMARY KEY (col) or PRIMARY KEY (a, b)
	re := regexp.MustCompile(`(?i)PRIMARY\s+KEY\s*\(\s*([^)]+)\)`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		for _, part := range strings.Split(m[1], ",") {
			name := strings.TrimSpace(part)
			for i := range t.Columns {
				if t.Columns[i].Name == name {
					t.Columns[i].PrimaryKey = true
					break
				}
			}
		}
	}
}

func applyForeignKey(t *Table, s string) {
	fkRe := regexp.MustCompile(`(?i)FOREIGN\s+KEY\s*\(\s*["']?(\w+)["']?\s*\)\s+REFERENCES\s+["']?(\w+)["']?\s*\(\s*["']?(\w+)["']?\s*\)`)
	if m := fkRe.FindStringSubmatch(s); len(m) >= 4 {
		for i := range t.Columns {
			if t.Columns[i].Name == m[1] {
				t.Columns[i].ForeignKey = &ForeignKey{RefTable: m[2], RefColumn: m[3]}
				break
			}
		}
	}
}

// topologicalSort returns tables in insert order (dependencies first).
func topologicalSort(tables []*Table) []*Table {
	byName := make(map[string]*Table)
	for _, t := range tables {
		byName[t.Name] = t
	}
	var order []*Table
	visited := make(map[string]bool)
	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		if t, ok := byName[name]; ok {
			for _, dep := range t.DependsOn() {
				visit(dep)
			}
			order = append(order, t)
		}
	}
	for _, t := range tables {
		visit(t.Name)
	}
	return order
}

// TableByName returns a table by name from the slice (original order not required).
func TableByName(tables []*Table, name string) *Table {
	for _, t := range tables {
		if t.Name == name {
			return t
		}
	}
	return nil
}


