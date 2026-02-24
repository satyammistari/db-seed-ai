package generator

import (
	"fmt"
	"strings"

	"github.com/satyammistari/seeddb/internal/schema"
)

// BuildPrompt creates the text we send to Ollama.
// It tells the AI:
//  1. What table to generate for
//  2. What columns exist and their rules
//  3. What values are allowed (CHECK constraints)
//  4. What FK values already exist in the DB
//  5. Exactly what format to return
func BuildPrompt(
	table *schema.Table,
	numRows int,
	fullSchema *schema.Schema,
	style string,
	existingIDs map[string][]interface{},
) string {
	return fmt.Sprintf(
		`You are a database seed data generator.
Generate exactly %d rows of realistic data for this table.

TABLE NAME: %s

COLUMNS (what each column needs):
%s

RULES YOU MUST FOLLOW STRICTLY:
%s

DATA STYLE: %s
%s

FOREIGN KEY VALUES (ONLY use these exact values for FK columns):
%s

OUTPUT RULES — FOLLOW EXACTLY:
- Your response must start with [ and end with ]
- Return ONLY the JSON array, absolutely nothing else
- No words before the array, no words after the array
- No markdown, no backticks, no code fences
- No "Here are the rows:" type introduction text
- Each element must be a JSON object with {} braces
- All keys must be column names in double quotes
- Skip SERIAL and AUTO_INCREMENT primary key columns

START YOUR RESPONSE WITH [ AND NOTHING ELSE.

EXAMPLE of correct output format:
[
  {"name": "Sarah Mitchell", "email": "sarah@example.com"},
  {"name": "James Rodriguez", "email": "james@mail.com"}
]

Generate the JSON array for table %s now:`,
		numRows,
		table.Name,
		formatColumnDefs(table),
		formatConstraints(table, existingIDs),
		style,
		formatStyleHints(style),
		formatExistingIDs(existingIDs),
		table.Name,
	)
}

// formatColumnDefs builds the column list for the prompt.
// Example output:
//   - email: text [REQUIRED] [MUST BE UNIQUE]
//   - user_id: integer [REQUIRED] [FK → users.id]
//   - status: text [ONLY ALLOWED: pending, paid, shipped]
func formatColumnDefs(t *schema.Table) string {
	var sb strings.Builder

	for _, col := range t.NonAutoColumns() {
		sb.WriteString(fmt.Sprintf("  - %s: %s", col.Name, col.Type))

		if col.NotNull {
			sb.WriteString(" [REQUIRED]")
		}
		if col.Unique {
			sb.WriteString(" [MUST BE UNIQUE]")
		}
		if col.ForeignKey != nil {
			sb.WriteString(fmt.Sprintf(
				" [FK → %s.%s]",
				col.ForeignKey.RefTable,
				col.ForeignKey.RefColumn,
			))
		}
		if len(col.CheckIn) > 0 {
			sb.WriteString(fmt.Sprintf(
				" [ONLY ALLOWED VALUES: %s]",
				strings.Join(col.CheckIn, ", "),
			))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatConstraints writes rules in plain English.
// Example output:
//   - email MUST NOT be null or empty
//   - email MUST be unique across all rows
//   - status MUST be exactly one of: pending | paid | shipped
func formatConstraints(
	t *schema.Table,
	existingIDs map[string][]interface{},
) string {
	var constraints []string

	for _, col := range t.NonAutoColumns() {
		if col.NotNull {
			constraints = append(constraints,
				fmt.Sprintf("  - %s MUST NOT be null or empty", col.Name),
			)
		}
		if col.Unique {
			constraints = append(constraints,
				fmt.Sprintf(
					"  - %s MUST be unique — no two rows can have the same value",
					col.Name,
				),
			)
		}
		if len(col.CheckIn) > 0 {
			constraints = append(constraints,
				fmt.Sprintf(
					"  - %s MUST be exactly one of: %s",
					col.Name,
					strings.Join(col.CheckIn, " | "),
				),
			)
		}
		if col.ForeignKey != nil {
			if ids, ok := existingIDs[col.Name]; ok && len(ids) > 0 {
				shown := ids
				if len(shown) > 10 {
					shown = shown[:10]
				}
				vals := make([]string, len(shown))
				for i, v := range shown {
					vals[i] = fmt.Sprintf("%v", v)
				}
				constraints = append(constraints,
					fmt.Sprintf(
						"  - %s MUST be one of these exact values: [%s]",
						col.Name,
						strings.Join(vals, ", "),
					),
				)
			} else {
				constraints = append(constraints,
					fmt.Sprintf(
						"  - %s is a foreign key — use small integers like 1, 2, 3",
						col.Name,
					),
				)
			}
		}
	}

	if len(constraints) == 0 {
		return "  No special constraints"
	}
	return strings.Join(constraints, "\n")
}

// formatStyleHints adds extra instructions based on style flag.
func formatStyleHints(style string) string {
	switch style {
	case "realistic":
		return `
STYLE HINTS for realistic data:
- Names should sound like real people from different cultures
- Emails should match the person's name (sarah.m@gmail.com)
- Dates spread across the last 2 years
- Prices realistic for the domain (not 0.01 or 99999.99)
- Text fields contain coherent readable sentences`

	case "edge-cases":
		return `
STYLE HINTS for edge case data:
- Include NULL for about 20% of nullable fields
- Include strings near their maximum length
- Include special characters: apostrophes, hyphens, accents
- Include numbers at boundaries: 0, 1, max value
- Include dates at month and year boundaries`

	case "minimal":
		return `
STYLE HINTS for minimal data:
- Short simple values only
- ASCII characters only, no special chars
- No punctuation in text fields
- Simple short strings like "name1", "name2"`

	default:
		return ""
	}
}

// formatExistingIDs shows the AI what FK reference IDs exist.
// Example output:
//
//	user_id: [1, 2, 3, 4, 5, 8, 12]
//	category_id: [1, 2, 3]
func formatExistingIDs(existingIDs map[string][]interface{}) string {
	if len(existingIDs) == 0 {
		return "  This table has no foreign keys"
	}

	var sb strings.Builder
	for col, ids := range existingIDs {
		shown := ids
		if len(shown) > 20 {
			shown = shown[:20]
		}
		vals := make([]string, len(shown))
		for i, v := range shown {
			vals[i] = fmt.Sprintf("%v", v)
		}
		sb.WriteString(fmt.Sprintf(
			"  %s: [%s]\n",
			col,
			strings.Join(vals, ", "),
		))
	}
	return sb.String()
}
