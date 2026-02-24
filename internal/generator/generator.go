package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/satyammistari/seeddb/internal/schema"
)


func (g *Generator) Generate(
    table       *schema.Table,
    numRows     int,
    fullSchema  *schema.Schema,
    style       string,
    existingIDs map[string][]interface{},
) (*GenerationResult, error) {

    prompt := BuildPrompt(
        table, numRows, fullSchema,
        style, existingIDs,
    )

    raw, err := g.client.Generate(prompt)
    if err != nil {
        return nil, fmt.Errorf(
            "generate for %s: %w", table.Name, err,
        )
    }

    // ── ADD THIS DEBUG BLOCK ──────────────────────────
    fmt.Println("=== DEBUG RAW RESPONSE START ===")
    if len(raw) > 500 {
        fmt.Println(raw[:500])
    } else {
        fmt.Println(raw)
    }
    fmt.Println("=== DEBUG RAW RESPONSE END ===")
    // ── END DEBUG BLOCK ───────────────────────────────

    rows, err := parseJSONResponse(raw)
    if err != nil {
        return nil, fmt.Errorf(
            "parse: %w", err,
        )
    }
    // rest of function...
}





// Style is the data generation style.
type Style string

const (
	StyleRealistic  Style = "realistic"
	StyleMinimal   Style = "minimal"
	StyleEdgeCases Style = "edge-cases"
)

// Config holds generator options.
type Config struct {
	Model     string
	Style     Style
	OllamaURL string
}

// DefaultConfig returns config with defaults.
func DefaultConfig() Config {
	return Config{
		Model:     "llama3",
		Style:     StyleRealistic,
		OllamaURL: "http://localhost:11434",
	}
}

// GenerateRequest is the JSON body for Ollama generate API.
type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// GenerateResponse is the JSON response (non-streaming we use /api/generate with stream=false).
type GenerateResponse struct {
	Response string `json:"response"`
}

// BuildPrompt builds the AI prompt for generating rows for one table.
func BuildPrompt(t *schema.Table, tables []*schema.Table, rows int, style Style, refIDs map[string][]interface{}) string {
	var b strings.Builder
	b.WriteString("You are a database seed data generator. Generate exactly ")
	b.WriteString(fmt.Sprintf("%d", rows))
	b.WriteString(" rows of realistic data for the following table.\n\n")
	b.WriteString("Table: ")
	b.WriteString(t.Name)
	b.WriteString("\n\nColumns (generate valid values for each):\n")
	for _, c := range t.Columns {
		b.WriteString("  - ")
		b.WriteString(c.Name)
		b.WriteString(" (")
		b.WriteString(c.Type)
		if c.NotNull {
			b.WriteString(", NOT NULL")
		}
		if c.Unique {
			b.WriteString(", UNIQUE")
		}
		if len(c.CheckIn) > 0 {
			b.WriteString(", one of: ")
			b.WriteString(strings.Join(c.CheckIn, ", "))
		}
		if c.ForeignKey != nil {
			b.WriteString(", references ")
			b.WriteString(c.ForeignKey.RefTable)
			b.WriteString(".")
			b.WriteString(c.ForeignKey.RefColumn)
		}
		b.WriteString(")\n")
	}
	if style == StyleRealistic {
		b.WriteString("\nStyle: realistic — names, emails, and text that look like a real app. No placeholders like 'test' or 'foo'.\n")
	} else if style == StyleMinimal {
		b.WriteString("\nStyle: minimal — short values, ASCII only, no special characters. Good for tests.\n")
	} else {
		b.WriteString("\nStyle: edge-cases — include some NULLs where allowed, boundary numbers, max-length strings, special characters. Good for QA.\n")
	}
	// Pass existing IDs for FK columns
	for _, c := range t.Columns {
		if c.ForeignKey != nil {
			key := c.ForeignKey.RefTable + "." + c.ForeignKey.RefColumn
			if ids, ok := refIDs[key]; ok && len(ids) > 0 {
				b.WriteString("\nUse only these values for ")
				b.WriteString(c.Name)
				b.WriteString(" (existing IDs from ")
				b.WriteString(c.ForeignKey.RefTable)
				b.WriteString("): ")
				b.WriteString(idsToPrompt(ids))
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("\nRespond with a single JSON array of objects. Each object has keys matching column names. No markdown, no explanation — only the JSON array. Example: [{\"id\":1,\"name\":\"Alice\"},{...}]\n")
	return b.String()
}

func idsToPrompt(ids []interface{}) string {
	var parts []string
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%v", id))
	}
	if len(parts) > 50 {
		parts = parts[:50]
	}
	return strings.Join(parts, ", ")
}

// CallOllama sends the prompt to Ollama and returns the raw response text.
func CallOllama(cfg Config, prompt string) (string, error) {
	body, _ := json.Marshal(GenerateRequest{
		Model:  cfg.Model,
		Prompt: prompt,
		Stream: false,
	})
	url := strings.TrimSuffix(cfg.OllamaURL, "/") + "/api/generate"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", err
	}
	return genResp.Response, nil
}

func parseJSONResponse(
	raw string,
) ([]map[string]interface{}, error) {
	raw = strings.TrimSpace(raw)

	// ── Step 1: Strip DeepSeek <think> blocks ────────
	// DeepSeek-R1 outputs reasoning before the answer
	// Format: <think>...reasoning...</think>[actual JSON]
	// We remove everything up to and including </think>
	if idx := strings.Index(raw, "</think>"); idx != -1 {
		raw = strings.TrimSpace(raw[idx+8:])
	}

	// ── Step 2: Also strip opening <think> if present ─
	// Sometimes only <think> appears without </think>
	if idx := strings.Index(raw, "<think>"); idx != -1 {
		// Try to find the end
		endIdx := strings.Index(raw, "</think>")
		if endIdx != -1 {
			raw = strings.TrimSpace(raw[endIdx+8:])
		} else {
			// No closing tag — skip everything after <think>
			raw = strings.TrimSpace(raw[:idx])
		}
	}

	// ── Step 3: Strip markdown code blocks ───────────
	// AI sometimes wraps JSON in ```json ... ```
	// Remove those markers
	raw = strings.ReplaceAll(raw, "```json", "")
	raw = strings.ReplaceAll(raw, "```", "")
	raw = strings.TrimSpace(raw)

	// ── Step 4: Find the JSON array ──────────────────
	// Find the first [ and last ] to extract just the array
	// This handles any remaining text before or after JSON
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")

	if start == -1 || end == -1 || end <= start {
		// Show what we actually received to help debug
		preview := raw
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		return nil, fmt.Errorf(
			"no JSON array found in response\n"+
				"received: %s",
			preview,
		)
	}

	jsonStr := raw[start : end+1]

	// ── Step 5: Parse the JSON ────────────────────────
	var rows []map[string]interface{}
	if err := json.Unmarshal(
		[]byte(jsonStr), &rows,
	); err != nil {
		preview := jsonStr
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		return nil, fmt.Errorf(
			"unmarshal JSON: %w\n"+
				"json was: %s",
			err, preview,
		)
	}

	return rows, nil
}

