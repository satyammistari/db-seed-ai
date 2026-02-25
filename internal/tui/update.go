package tui

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/satyammistari/db-seed-ai/internal/generator"
	"github.com/satyammistari/db-seed-ai/internal/inserter"
	"github.com/satyammistari/db-seed-ai/internal/schema"
)

type schemaLoadedMsg  struct{ s *schema.Schema }
type tableProgressMsg struct {
	tableName string
	rowsDone  int
	rowsTotal int
	status    TableStatus
}
type seedDoneMsg  struct {
	totalRows int
	duration  time.Duration
}
type seedErrMsg   struct{ err error }
type previewReadyMsg struct {
	rows []map[string]interface{}
	cols []string
}
type errMsg struct{ err error }

func (m Model) Init() tea.Cmd {
    return tea.Batch(m.Spinner.Tick, textinput.Blink)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {

    case tea.WindowSizeMsg:
        m.Width  = msg.Width
        m.Height = msg.Height
        return m, nil

    case tea.KeyMsg:
        if msg.String() == "ctrl+c" || msg.String() == "Q" {
            return m, tea.Quit
        }
        switch m.ActiveTab {
        case TabGenerate:
            return m.handleGenerateKey(msg)
        case TabPreview:
            return m.handlePreviewKey(msg)
        case TabHistory:
            return m.handleHistoryKey(msg)
        case TabHelp:
            return m.handleHelpKey(msg)
        }

    case spinner.TickMsg:
        if m.IsRunning {
            var cmd tea.Cmd
            m.Spinner, cmd = m.Spinner.Update(msg)
            cmds = append(cmds, cmd)
        }

    case schemaLoadedMsg:
        m.Progress = make([]TableProgress, len(msg.s.InsertOrder))
        for i, name := range msg.s.InsertOrder {
            m.Progress[i] = TableProgress{
                Name:      name,
                Status:    StatusWaiting,
                RowsTotal: 100,
            }
        }
        m.StatusMsg  = fmt.Sprintf("Schema loaded → %d tables", len(msg.s.Tables))
        m.StatusKind = "success"

    case tableProgressMsg:
        for i, p := range m.Progress {
            if p.Name == msg.tableName {
                m.Progress[i].RowsDone  = msg.rowsDone
                m.Progress[i].RowsTotal = msg.rowsTotal
                m.Progress[i].Status    = msg.status
                break
            }
        }
        cmds = append(cmds, m.Spinner.Tick)

    case seedDoneMsg:
        m.IsRunning  = false
        m.FinishTime = time.Now()
		m.TotalRows  = msg.totalRows
		m.StatusMsg  = fmt.Sprintf(
			"✓ Done in %s → %d rows inserted",
			msg.duration.Round(time.Second), msg.totalRows,
		)
		m.StatusKind = "success"
		m.History = append([]HistoryEntry{{
			Timestamp:    time.Now(),
			SchemaFile:   m.GetSchemaPath(),
			Database:     m.GetDBConn(),
			Model:        m.GetModel(),
			TablesSeeded: len(m.Progress),
			TotalRows:    msg.totalRows,
			Duration:     msg.duration,
			Success:      true,
		}}, m.History...)

	case seedErrMsg:
		m.IsRunning  = false
		m.FinishTime = time.Now()
		m.Err        = msg.err
		m.StatusMsg  = fmt.Sprintf("✗ Error: %v", msg.err)
		m.StatusKind = "error"
		m.History = append([]HistoryEntry{{
			Timestamp:  time.Now(),
			SchemaFile: m.GetSchemaPath(),
			Database:   m.GetDBConn(),
			Model:      m.GetModel(),
			Success:    false,
			ErrMsg:     msg.err.Error(),
		}}, m.History...)

	case previewReadyMsg:
		m.PreviewRows    = msg.rows
		m.PreviewCols    = msg.cols
		m.PreviewLoading = false
		m.StatusMsg      = fmt.Sprintf("Preview ready → %d rows", len(msg.rows))
		m.StatusKind     = "success"

	case errMsg:
		m.StatusMsg  = fmt.Sprintf("✗ %v", msg.err)
		m.StatusKind = "error"
	}

	for i := range m.Fields {
		var cmd tea.Cmd
		m.Fields[i], cmd = m.Fields[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleGenerateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd
    switch msg.String() {
    case "tab":
        m = m.blurAllFields()
        m.ActiveTab = Tab((int(m.ActiveTab) + 1) % 4)
        return m, nil
    case "shift+tab":
        m = m.blurAllFields()
        m.ActiveTab = Tab((int(m.ActiveTab) + 3) % 4)
        return m, nil
    case "!": m = m.blurAllFields(); m.ActiveTab = TabGenerate; return m, nil  // Shift+1
    case "@": m = m.blurAllFields(); m.ActiveTab = TabPreview;  return m, nil  // Shift+2
    case "#": m = m.blurAllFields(); m.ActiveTab = TabHistory;  return m, nil  // Shift+3
    case "$": m = m.blurAllFields(); m.ActiveTab = TabHelp;     return m, nil  // Shift+4
    case "I", "J": // Shift+i or Shift+j for focus next field
        if !m.anyFieldFocused() {
            m.FocusedField = 0
            m.Fields[0].Focus()
            return m, textinput.Blink
        }
        m.Fields[m.FocusedField].Blur()
        m.FocusedField = (m.FocusedField + 1) % len(m.Fields)
        m.Fields[m.FocusedField].Focus()
        return m, textinput.Blink
    case "L", "K": // Shift+l or Shift+k for focus previous field
        if !m.anyFieldFocused() { return m, nil }
        m.Fields[m.FocusedField].Blur()
        m.FocusedField = (m.FocusedField + len(m.Fields) - 1) % len(m.Fields)
        m.Fields[m.FocusedField].Focus()
        return m, textinput.Blink
    case "esc":
        m = m.blurAllFields()
        return m, nil
    case "enter":
        if m.IsRunning { return m, nil }
        m = m.blurAllFields()
        return m.startSeeding()
    }
    if m.anyFieldFocused() {
        var cmd tea.Cmd
        m.Fields[m.FocusedField], cmd = m.Fields[m.FocusedField].Update(msg)
        cmds = append(cmds, cmd)
    }
    return m, tea.Batch(cmds...)
}

func (m Model) handlePreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "tab":       m.ActiveTab = Tab((int(m.ActiveTab)+1)%4)
    case "shift+tab": m.ActiveTab = Tab((int(m.ActiveTab)+3)%4)
    case "!": m.ActiveTab = TabGenerate  // Shift+1
    case "@": m.ActiveTab = TabPreview   // Shift+2
    case "#": m.ActiveTab = TabHistory   // Shift+3
    case "$": m.ActiveTab = TabHelp      // Shift+4
    case "J": // Shift+j for scroll down
        if m.PreviewScroll < len(m.PreviewRows)-1 { m.PreviewScroll++ }
    case "K": // Shift+k for scroll up
        if m.PreviewScroll > 0 { m.PreviewScroll-- }
    case "g": m.PreviewScroll = 0
    case "G":
        if len(m.PreviewRows) > 0 { m.PreviewScroll = len(m.PreviewRows)-1 }
    case "enter":
        return m.startPreview()
    }
    return m, nil
}

func (m Model) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "tab":       m.ActiveTab = Tab((int(m.ActiveTab)+1)%4)
    case "shift+tab": m.ActiveTab = Tab((int(m.ActiveTab)+3)%4)
    case "!": m.ActiveTab = TabGenerate  // Shift+1
    case "@": m.ActiveTab = TabPreview   // Shift+2
    case "#": m.ActiveTab = TabHistory   // Shift+3
    case "$": m.ActiveTab = TabHelp      // Shift+4
    case "J": // Shift+j for scroll down
        if m.HistoryScroll < len(m.History)-1 { m.HistoryScroll++ }
    case "K": // Shift+k for scroll up
        if m.HistoryScroll > 0 { m.HistoryScroll-- }
    }
    return m, nil
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "tab":       m.ActiveTab = Tab((int(m.ActiveTab)+1)%4)
    case "shift+tab": m.ActiveTab = Tab((int(m.ActiveTab)+3)%4)
    case "!": m.ActiveTab = TabGenerate  // Shift+1
    case "@": m.ActiveTab = TabPreview   // Shift+2
    case "#": m.ActiveTab = TabHistory   // Shift+3
    case "$": m.ActiveTab = TabHelp      // Shift+4
    }
    return m, nil
}

func (m Model) startSeeding() (Model, tea.Cmd) {
    m.IsRunning  = true
    m.StartTime  = time.Now()
    m.FinishTime = time.Time{}
    m.TotalRows  = 0
    m.Err        = nil
    m.StatusMsg  = "Starting seed pipeline..."
    m.StatusKind = "info"

    schemaPath := m.GetSchemaPath()
    dbConn     := m.GetDBConn()
    modelName  := m.GetModel()
    rows, _    := strconv.Atoi(m.GetRows())
    if rows <= 0 { rows = 100 }

    return m, tea.Batch(
        m.Spinner.Tick,
        func() tea.Msg {
            return runSeedPipeline(schemaPath, dbConn, modelName, rows)
        },
    )
}

func runSeedPipeline(schemaPath, dbConn, modelName string, numRows int) tea.Msg {
	start := time.Now()

	// Read schema file
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return seedErrMsg{err: fmt.Errorf("read schema file: %w", err)}
	}

	// Parse schema
	s, err := schema.ParseFileToSchema(string(content))
	if err != nil {
		return seedErrMsg{err: fmt.Errorf("parse schema: %w", err)}
	}

	// Create generator
	cfg := generator.DefaultConfig()
	cfg.Model = modelName
	cfg.Style = generator.StyleRealistic
	gen := generator.New(cfg)

	// Open database connection
	db, driver, err := inserter.Open(dbConn)
	if err != nil {
		return seedErrMsg{err: fmt.Errorf("connect db: %w", err)}
	}
	defer db.Close()

	totalRows := 0

	// Generate and insert for each table
	for _, tableName := range s.InsertOrder {
		t := s.TableMap[tableName]
		if t == nil {
			continue
		}

		// Fetch existing IDs for FK references
		existingIDs := make(map[string][]interface{})
		for _, col := range t.FKColumns() {
			if col.ForeignKey == nil {
				continue
			}
			ids, err := inserter.FetchRefIDs(db, col.ForeignKey.RefTable, col.ForeignKey.RefColumn, 1000)
			if err == nil {
				existingIDs[col.Name] = ids
			}
		}

		// Generate rows
		result, err := gen.Generate(t, numRows, s, "realistic", existingIDs)
		if err != nil {
			return seedErrMsg{err: fmt.Errorf("generate %s: %w", tableName, err)}
		}

		// Insert rows
		n, err := inserter.InsertBatch(db, driver, tableName, result.Columns, result.Rows)
		if err != nil {
			return seedErrMsg{err: fmt.Errorf("insert %s: %w", tableName, err)}
		}
		totalRows += n
	}

	return seedDoneMsg{totalRows: totalRows, duration: time.Since(start)}
}

func (m Model) startPreview() (Model, tea.Cmd) {
	m.PreviewLoading = true
	m.StatusMsg      = "Generating preview rows..."
	m.StatusKind     = "info"

	schemaPath := m.GetSchemaPath()
	modelName  := m.GetModel()

	return m, func() tea.Msg {
		// Read schema file
		content, err := os.ReadFile(schemaPath)
		if err != nil {
			return errMsg{err: err}
		}

		// Parse schema
		s, err := schema.ParseFileToSchema(string(content))
		if err != nil {
			return errMsg{err: err}
		}

		if len(s.Tables) == 0 {
			return errMsg{err: fmt.Errorf("no tables found")}
		}

		// Generate preview for first table
		t := s.Tables[0]
		cfg := generator.DefaultConfig()
		cfg.Model = modelName
		cfg.Style = generator.StyleRealistic
		gen := generator.New(cfg)

		result, err := gen.Generate(t, 5, s, "realistic", map[string][]interface{}{})
		if err != nil {
			return errMsg{err: err}
		}

		return previewReadyMsg{rows: result.Rows, cols: result.Columns}
	}
}


