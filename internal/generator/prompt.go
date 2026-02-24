package generator

import (
	"fmt"
	"strings"

	"github.com/satyammistari/seeddb/internal/schema"
)

// BuildPrompt creates the text we send to DeepSeek/Ollama
// This is the most important function in the project
// Better prompt = better data quality
//
// It tells the AI:
// 1. What table to generate for
// 2. What columns exist and their rules
// 3. What values are allowed (CHECK constraints)
// 4. What FK values already exist in the DB
// 5. Exactly what format to return
func BuildPrompt(
	table      *schema.Table,
	numRows    int,
	fullSchema *schema.Schema,
	style      string,
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

// formatColumnDefs builds the column list for the prompt
// Tells AI: column name, type, and what rules apply
//
// Example output:
//   - email: VARCHAR(255) [REQUIRED] [MUST BE UNIQUE]
//   - user_id: INTEGER [REQUIRED] [FK → users.id]
//   - status: VARCHAR(20) [ONLY ALLOWED: pending, paid, shipped]
func formatColumnDefs(t *schema.Table) string {
	var sb strings.Builder

	for _, col := range t.NonAutoColumns() {
		// Start with: "  - columnname: DATATYPE"
		sb.WriteString(fmt.Sprintf(
			"  - %s: %s", col.Name, col.DataType,
		))

		// Add length for VARCHAR(255) style types
		if col.MaxLength > 0 {
			sb.WriteString(fmt.Sprintf(
				"(%d)", col.MaxLength,
			))
		}

		// Required = NOT NULL in schema
		if !col.IsNullable {
			sb.WriteString(" [REQUIRED]")
		}

		// Unique = UNIQUE constraint in schema
		if col.IsUnique {
			sb.WriteString(" [MUST BE UNIQUE]")
		}

		// Foreign key = REFERENCES other_table(col)
		if col.IsForeignKey && col.References != nil {
			sb.WriteString(fmt.Sprintf(
				" [FK → %s.%s]",
				col.References.Table,
				col.References.Column,
			))
		}

		// CHECK IN constraint — only allowed values
		if len(col.AllowedValues) > 0 {
			sb.WriteString(fmt.Sprintf(
				" [ONLY ALLOWED VALUES: %s]",
				strings.Join(col.AllowedValues, ", "),
			))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// formatConstraints writes rules in plain English
// Humans understand "MUST NOT be null" better than SQL
// AI models also respond better to plain English rules
//
// Example output:
//   - email MUST NOT be null or empty
//   - email MUST be unique across all rows
//   - status MUST be one of: pending | paid | shipped
//   - user_id MUST be a value from EXISTING FK VALUES list
func formatConstraints(
	t           *schema.Table,
	existingIDs map[string][]interface{},
) string {
	var constraints []string

	for _, col := range t.NonAutoColumns() {

		// NOT NULL rule
		if !col.IsNullable {
			constraints = append(constraints,
				fmt.Sprintf(
					"  - %s MUST NOT be null or empty",
					col.Name,
				),
			)
		}

		// UNIQUE rule
		if col.IsUnique {
			constraints = append(constraints,
				fmt.Sprintf(
					"  - %s MUST be unique — "+
						"no two rows can have the same value",
					col.Name,
				),
			)
		}

		// CHECK IN constraint rule
		if len(col.AllowedValues) > 0 {
			constraints = append(constraints,
				fmt.Sprintf(
					"  - %s MUST be exactly one of: %s",
					col.Name,
					strings.Join(col.AllowedValues, " | "),
				),
			)
		}

		// FK rule — most important constraint
		// Without this AI invents IDs that don't exist
		if col.IsForeignKey {
			if ids, ok := existingIDs[col.Name]; ok && len(ids) > 0 {
				// Show first 10 valid IDs as examples
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
						"  - %s MUST be one of these "+
							"exact values: [%s]",
						col.Name,
						strings.Join(vals, ", "),
					),
				)
			} else {
				constraints = append(constraints,
					fmt.Sprintf(
						"  - %s is a foreign key — "+
							"use small integers like 1, 2, 3",
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

// formatStyleHints adds extra instructions based on style flag
// --style realistic = names look real, emails match names
// --style edge-cases = NULLs, boundary values, special chars
// --style minimal = short simple values
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

// formatExistingIDs shows the AI what FK values exist
// This is the key to making FK relationships work
//
// Example output:
//   user_id: [1, 2, 3, 4, 5, 8, 12]
//   category_id: [1, 2, 3]
func formatExistingIDs(
	existingIDs map[string][]interface{},
) string {
	if len(existingIDs) == 0 {
		return "  This table has no foreign keys"
	}

	var sb strings.Builder

	for col, ids := range existingIDs {
		// Only show max 20 IDs to keep prompt short
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