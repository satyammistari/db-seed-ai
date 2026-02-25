package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"github.com/satyammistari/db-seed-ai/internal/schema"
)

// OllamaClient wraps the Ollama HTTP API.
type OllamaClient struct {
	cfg Config
}

// NewOllamaClient creates a client from a Config.
func NewOllamaClient(cfg Config) *OllamaClient {
	return &OllamaClient{cfg: cfg}
}

// Generate sends a prompt to Ollama and returns the raw text response.
func (c *OllamaClient) Generate(prompt string) (string, error) {
	return CallOllama(c.cfg, prompt)
}

// Generator holds an OllamaClient and is the high-level entry point.
type Generator struct {
	client *OllamaClient
	cfg    Config
}

// New returns a Generator with the given config.
func New(cfg Config) *Generator {
	return &Generator{
		client: NewOllamaClient(cfg),
		cfg:    cfg,
	}
}

// GenerationResult contains the rows returned by the AI plus metadata
// needed by the inserter to build the SQL statement.
type GenerationResult struct {
	TableName string
	Columns   []string
	Rows      []map[string]interface{}
}

// ParseJSONRows parses the raw AI response into typed rows.
// columnHint is used to filter / order columns if provided.
func ParseJSONRows(raw string, columnHint []string) ([]map[string]interface{}, error) {
	return parseJSONResponse(raw)
}

// GenerateForTable is a convenience wrapper used by cmd.
func GenerateForTable(
	cfg Config,
	table *schema.Table,
	numRows int,
	fullSchema []*schema.Table,
	style string,
	existingIDs map[string][]interface{},
) (*GenerationResult, error) {
	g := New(cfg)
	return g.Generate(table, numRows, &schema.Schema{Tables: fullSchema}, style, existingIDs)
}

// colNames returns non-auto column names for a table.
func colNames(t *schema.Table) []string {
	var names []string
	for _, c := range t.Columns {
		names = append(names, c.Name)
	}
	return names
}

// buildOllamaRequestBody creates the JSON body for the Ollama generate API.
func buildOllamaRequestBody(model, prompt string) ([]byte, error) {
	return json.Marshal(struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	})
}

// sendOllamaRequest sends the HTTP request and decodes the response.
func sendOllamaRequest(url string, body []byte) (string, error) {
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

	var genResp struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", err
	}
	return genResp.Response, nil
}

// shuffleStrings shuffles a string slice in-place (used for edge-case style).
func shuffleStrings(s []string) {
	rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

// StyleRealistic is the constant for realistic data generation.
const StyleRealistic Style = "realistic"

// joinStyle converts a Style value to a lowercase string for prompt building.
func joinStyle(s Style) string {
	return strings.ToLower(string(s))
}


