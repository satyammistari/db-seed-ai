package reporter

import (
	"fmt"
	"os"
	"strings"
)

// NoColor disables ANSI color output.
var NoColor = false

const (
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	reset  = "\033[0m"
)

func c(s string) string {
	if NoColor {
		return ""
	}
	return s
}

// Ok prints a green check message.
func Ok(msg string) {
	fmt.Fprintf(os.Stderr, "  %s✓%s %s\n", c(green), c(reset), msg)
}

// Info prints an info line.
func Info(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// Warn prints a yellow warning.
func Warn(msg string) {
	fmt.Fprintf(os.Stderr, "  %s⚠%s %s\n", c(yellow), c(reset), msg)
}

// Err prints a red error.
func Err(msg string) {
	fmt.Fprintf(os.Stderr, "  %s✗%s %s\n", c(red), c(reset), msg)
}

// Table prints a simple ASCII table from rows (slice of maps) and column names.
func Table(columns []string, rows []map[string]interface{}) {
	if len(rows) == 0 {
		return
	}
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col)
	}
	for _, row := range rows {
		for i, col := range columns {
			v := row[col]
			s := fmt.Sprint(v)
			if len(s) > 30 {
				s = s[:27] + "..."
			}
			if len(s) > widths[i] {
				widths[i] = len(s)
			}
		}
	}
	// Cap max width
	for i := range widths {
		if widths[i] > 40 {
			widths[i] = 40
		}
	}
	sep := "+"
	for _, w := range widths {
		sep += strings.Repeat("-", w+2) + "+"
	}
	fmt.Println(sep)
	header := "|"
	for i, col := range columns {
		header += " " + pad(col, widths[i]) + " |"
	}
	fmt.Println(header)
	fmt.Println(sep)
	for _, row := range rows {
		line := "|"
		for i, col := range columns {
			s := fmt.Sprint(row[col])
			if len(s) > 40 {
				s = s[:37] + "..."
			}
			line += " " + pad(s, widths[i]) + " |"
		}
		fmt.Println(line)
	}
	fmt.Println(sep)
}

func pad(s string, w int) string {
	if len(s) > w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}


