package generator
import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/satyammistari/db-seed-ai/internal/schema"
)

// Generate calls Ollama to produce rows for a single table.
// It is the main entry-point used by main.go's runSeed/runValidate.
func (g *Generator) Generate(
	table *schema.Table,
	numRows int,
	fullSchema *schema.Schema,
	style string,
	existingIDs map[string][]interface{},
) (*GenerationResult, error) {
	prompt := BuildPrompt(table, numRows, fullSchema, style, existingIDs)

	raw, err := g.client.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("generate for %s: %w", table.Name, err)
	}

	colNames := nonAutoColNames(table)
	rows, err := ParseJSONRows(raw, colNames)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	return &GenerationResult{
		TableName: table.Name,
		Columns:   colNames,
		Rows:      rows,
	}, nil
}

// nonAutoColNames returns column names for non-auto (non-serial PK) columns.
func nonAutoColNames(t *schema.Table) []string {
	var names []string
	for _, c := range t.NonAutoColumns() {
		names = append(names, c.Name)
	}
	return names
}

// Style is the data generation style.
type Style string

const (
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

// GenerateResponse is the JSON response from Ollama (stream=false).
type GenerateResponse struct {
	Response string `json:"response"`
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

// parseJSONResponse extracts a JSON array from the raw AI response string.
// Handles DeepSeek <think> blocks and markdown code fences.
func parseJSONResponse(raw string) ([]map[string]interface{}, error) {
	raw = strings.TrimSpace(raw)

	// Strip DeepSeek <think> blocks
	if idx := strings.Index(raw, "</think>"); idx != -1 {
		raw = strings.TrimSpace(raw[idx+8:])
	}
	if idx := strings.Index(raw, "<think>"); idx != -1 {
		endIdx := strings.Index(raw, "</think>")
		if endIdx != -1 {
			raw = strings.TrimSpace(raw[endIdx+8:])
		} else {
			raw = strings.TrimSpace(raw[:idx])
		}
	}

	// Strip markdown code blocks
	raw = strings.ReplaceAll(raw, "```json", "")
	raw = strings.ReplaceAll(raw, "```", "")
	raw = strings.TrimSpace(raw)

	// Find the JSON array
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end == -1 || end <= start {
		preview := raw
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		return nil, fmt.Errorf(
			"no JSON array found in response\nreceived: %s",
			preview,
		)
	}

	jsonStr := raw[start : end+1]
	
	// Try to repair common JSON issues
	jsonStr = repairJSON(jsonStr)
	
	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &rows); err != nil {
		// If parsing fails, try to salvage partial data
		jsonStr = removeIncompleteLastObject(jsonStr)
		if err2 := json.Unmarshal([]byte(jsonStr), &rows); err2 != nil {
			preview := jsonStr
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			return nil, fmt.Errorf(
				"unmarshal JSON: %w\njson was: %s",
				err, preview,
			)
		}
	}
	return rows, nil
}

// repairJSON fixes common JSON formatting issues
func repairJSON(s string) string {
	// Remove trailing commas before closing brackets
	s = strings.ReplaceAll(s, ",]", "]")
	s = strings.ReplaceAll(s, ", ]", "]")
	s = strings.ReplaceAll(s, " ,]", "]")
	
	// Remove trailing commas before closing braces
	s = strings.ReplaceAll(s, ",}", "}")
	s = strings.ReplaceAll(s, ", }", "}")
	
	return strings.TrimSpace(s)
}

// removeIncompleteLastObject removes the last object if it's incomplete
func removeIncompleteLastObject(s string) string {
	// Find the last complete object by looking for the second-to-last closing brace
	lastBrace := strings.LastIndex(s, "}")
	if lastBrace == -1 {
		return "[]"
	}
	
	// Find the second-to-last closing brace
	beforeLast := s[:lastBrace]
	secondLastBrace := strings.LastIndex(beforeLast, "}")
	if secondLastBrace == -1 {
		// Only one object, and it might be complete
		return s
	}
	
	// Check if there's content after the second-to-last brace (excluding whitespace, commas)
	afterSecond := strings.TrimSpace(s[secondLastBrace+1:])
	if len(afterSecond) > 0 && afterSecond[0] == ',' {
		afterSecond = strings.TrimSpace(afterSecond[1:])
	}
	
	// If there's incomplete data, remove it
	if len(afterSecond) > 2 && !strings.HasSuffix(afterSecond, "}]") {
		return s[:secondLastBrace+1] + "]"
	}
	
	return s
}


