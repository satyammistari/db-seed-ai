package tui

import (
    "fmt"
    "time"

    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/bubbles/textinput"
)

type Tab int

const (
    TabGenerate Tab = iota
    TabPreview
    TabHistory
    TabHelp
)

func (t Tab) String() string {
    return []string{
        " Generate ",
        " Preview ",
        " History ",
        " Help ",
    }[t]
}

type TableStatus int

const (
    StatusWaiting TableStatus = iota
    StatusRunning
    StatusInserting
    StatusDone
    StatusError
)

func (s TableStatus) Label() string {
    return []string{"waiting","generating","inserting","done","error"}[s]
}

type TableProgress struct {
    Name      string
    Status    TableStatus
    RowsDone  int
    RowsTotal int
    Err       error
}

func (tp TableProgress) Percent() float64 {
    if tp.RowsTotal == 0 { return 0 }
    return float64(tp.RowsDone) / float64(tp.RowsTotal)
}

type HistoryEntry struct {
    Timestamp    time.Time
    SchemaFile   string
    Database     string
    Model        string
    TablesSeeded int
    TotalRows    int
    Duration     time.Duration
    Success      bool
    ErrMsg       string
}

type Config struct {
    SchemaPath string
    DBConn     string
    Model      string
    Rows       int
    Style      string
    Tables     []string
}

type Model struct {
    ActiveTab     Tab
    Width         int
    Height        int
    Config        Config
    FocusedField  int
    Fields        []textinput.Model
    IsRunning     bool
    Progress      []TableProgress
    StartTime     time.Time
    FinishTime    time.Time
    TotalRows     int
    Spinner       spinner.Model
    PreviewTable  string
    PreviewRows   []map[string]interface{}
    PreviewCols   []string
    PreviewLoading bool
    PreviewScroll int
    History       []HistoryEntry
    HistoryScroll int
    StatusMsg     string
    StatusKind    string
    Err           error
}

func NewModel() Model {
    inputs := make([]textinput.Model, 4)

    inputs[0] = textinput.New()
    inputs[0].Placeholder = "testdata/ecommerce.sql"
    inputs[0].Focus()
    inputs[0].Width = 45
    inputs[0].Prompt = ""

    inputs[1] = textinput.New()
    inputs[1].Placeholder = "sqlite:./dev.db"
    inputs[1].Width = 45
    inputs[1].Prompt = ""

    inputs[2] = textinput.New()
    inputs[2].Placeholder = "deepseek-r1:7b"
    inputs[2].Width = 25
    inputs[2].Prompt = ""

    inputs[3] = textinput.New()
    inputs[3].Placeholder = "100"
    inputs[3].Width = 10
    inputs[3].Prompt = ""
    inputs[3].CharLimit = 6

    s := spinner.New()
    s.Spinner = spinner.Dot
    s.Style = spinnerStyle

    return Model{
        ActiveTab:   TabGenerate,
        FocusedField: 0,
        Fields:      inputs,
        Spinner:     s,
        Config:      Config{Model: "deepseek-r1:7b", Rows: 100, Style: "realistic"},
        History:     []HistoryEntry{},
        Progress:    []TableProgress{},
        StatusMsg:   "Ready → configure schema and database then press Enter",
        StatusKind:  "info",
    }
}

func (m Model) GetSchemaPath() string {
    v := m.Fields[0].Value()
    if v == "" { return m.Fields[0].Placeholder }
    return v
}

func (m Model) GetDBConn() string {
    v := m.Fields[1].Value()
    if v == "" { return m.Fields[1].Placeholder }
    return v
}

func (m Model) GetModel() string {
    v := m.Fields[2].Value()
    if v == "" { return "deepseek-r1:7b" }
    return v
}

func (m Model) GetRows() string {
    v := m.Fields[3].Value()
    if v == "" { return "100" }
    return v
}

func (m Model) TotalProgress() float64 {
    if len(m.Progress) == 0 { return 0 }
    total, done := 0, 0
    for _, p := range m.Progress {
        total += p.RowsTotal
        done  += p.RowsDone
    }
    if total == 0 { return 0 }
    return float64(done) / float64(total)
}

func (m Model) ElapsedTime() string {
    if m.StartTime.IsZero() { return "0s" }
    end := time.Now()
    if !m.FinishTime.IsZero() { end = m.FinishTime }
    return end.Sub(m.StartTime).Round(time.Second).String()
}

func (m Model) IsFinished() bool {
    if len(m.Progress) == 0 || m.IsRunning { return false }
    for _, p := range m.Progress {
        if p.Status == StatusWaiting ||
            p.Status == StatusRunning ||
            p.Status == StatusInserting {
            return false
        }
    }
    return true
}

func (m Model) anyFieldFocused() bool {
    for _, f := range m.Fields {
        if f.Focused() { return true }
    }
    return false
}

func (m Model) blurAllFields() Model {
    for i := range m.Fields { m.Fields[i].Blur() }
    return m
}

// Satisfy fmt import
var _ = fmt.Sprintf


