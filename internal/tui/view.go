package tui

import (
    "fmt"
    "strings"
    "time"

    "github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
    if m.Width == 0 { return "Loading..." }
    return strings.Join([]string{
        m.renderHeader(),
        m.renderContent(),
        m.renderStatusBar(),
        m.renderKeyBar(),
    }, "\n")
}

func (m Model) renderHeader() string {
    logo := titleStyle.Render(`
 ███████ ███████ ███████ ██████  ██████  ██████  
 ██      ██      ██      ██   ██ ██   ██ ██   ██ 
 ███████ █████   █████   ██   ██ ██████  ██████  
      ██ ██      ██      ██   ██ ██   ██ ██   ██ 
 ███████ ███████ ███████ ██████  ██████  ██████  
                                                  
         🌱 AI-Powered Database Seeding Tool 🌱`)
    
    tabs  := ""
    for i := Tab(0); i < 4; i++ {
        if i == m.ActiveTab {
            tabs += activeTabStyle.Render(i.String())
        } else {
            tabs += tabStyle.Render(i.String())
        }
    }
    right := ""
    if m.IsRunning {
        right = warningStyle.Render(m.Spinner.View() + " Running...")
    }
    
    header := lipgloss.JoinVertical(lipgloss.Left, logo, "")
    nav := tabs
    if right != "" {
        gap := m.Width - lipgloss.Width(nav) - lipgloss.Width(right) - 10
        if gap < 0 { gap = 0 }
        nav = nav + strings.Repeat(" ", gap) + right
    }
    
    return lipgloss.JoinVertical(lipgloss.Left, header, panelStyle.Width(m.Width - 4).Render(nav))
}

func (m Model) renderContent() string {
    switch m.ActiveTab {
    case TabGenerate: return m.renderGenerateTab()
    case TabPreview:  return m.renderPreviewTab()
    case TabHistory:  return m.renderHistoryTab()
    case TabHelp:     return m.renderHelpTab()
    }
    return ""
}

func (m Model) renderGenerateTab() string {
    cw := m.Width - 4
    lw := cw / 2
    rw := cw - lw - 3
    return lipgloss.JoinHorizontal(lipgloss.Top,
        m.renderConfigPanel(lw), "  ", m.renderProgressPanel(rw),
    )
}

func (m Model) renderConfigPanel(width int) string {
    var sb strings.Builder
    sb.WriteString(titleStyle.Render("Configuration") + "\n\n")

    defs := []struct{ label string; idx int }{
        {"Schema", 0}, {"Database", 1}, {"AI Model", 2}, {"Rows", 3},
    }
    for _, fd := range defs {
        lbl := labelStyle.Render(fd.label + ":")
        if m.Fields[fd.idx].Focused() {
            lbl = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Width(14).Render(fd.label + ":")
        }
        sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, lbl, m.Fields[fd.idx].View()) + "\n")
    }

    sb.WriteString("\n" + labelStyle.Render("Style:"))
    for _, s := range []string{"realistic", "minimal", "edge-cases"} {
        if s == m.Config.Style {
            sb.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Padding(0,1).Render("["+s+"]"))
        } else {
            sb.WriteString(dimStyle.Copy().Padding(0,1).Render(s))
        }
    }

    sb.WriteString("\n\n")
    if m.IsRunning {
        sb.WriteString(warningStyle.Render(m.Spinner.View() + " Generating..."))
    } else {
        sb.WriteString(
            lipgloss.NewStyle().Foreground(colorBg).Background(colorCyan).Bold(true).Padding(0,3).Render("  Enter to Start  "),
        )
    }
    return activePanelStyle.Width(width).Render(sb.String())
}

func (m Model) renderProgressPanel(width int) string {
    var sb strings.Builder
    sb.WriteString(titleStyle.Render("Progress") + "\n\n")

    if len(m.Progress) == 0 {
        sb.WriteString(dimStyle.Render("Press Enter to start seeding."))
    } else {
        for _, p := range m.Progress {
            sb.WriteString(valueStyle.Render(fmt.Sprintf("%-14s", p.Name)))
            sb.WriteString(RenderProgressBar(p.Percent()) + " ")
            sb.WriteString(dimStyle.Render(fmt.Sprintf("%-10s", fmt.Sprintf("%d/%d", p.RowsDone, p.RowsTotal))))
            switch p.Status {
            case StatusDone:      sb.WriteString(badgeDone.Render("✓"))
            case StatusRunning:   sb.WriteString(badgeRunning.Render(m.Spinner.View()))
            case StatusInserting: sb.WriteString(badgeRunning.Render("↑"))
            case StatusError:     sb.WriteString(badgeError.Render("✗"))
            default:              sb.WriteString(badgeWaiting.Render("◦"))
            }
            sb.WriteString("\n")
        }

        if m.IsRunning || m.IsFinished() {
            sb.WriteString("\n" + dimStyle.Render(strings.Repeat("-", width-4)) + "\n")
            overall := m.TotalProgress()
            sb.WriteString("Total  " + RenderProgressBar(overall))
            sb.WriteString(fmt.Sprintf("  %d%%  %s\n", int(overall*100), m.ElapsedTime()))
            if m.IsFinished() {
                sb.WriteString("\n" + successStyle.Render(fmt.Sprintf("✓ Done → %d rows in %s", m.TotalRows, m.ElapsedTime())))
            }
        }
    }
    return panelStyle.Width(width).Render(sb.String())
}

func (m Model) renderPreviewTab() string {
    var sb strings.Builder
    width := m.Width - 6
    sb.WriteString(titleStyle.Render("Preview Generated Rows") + "\n\n")

    if m.PreviewLoading {
        sb.WriteString(warningStyle.Render(m.Spinner.View() + " Generating preview..."))
    } else if len(m.PreviewRows) == 0 {
        sb.WriteString(dimStyle.Render("Press Enter to generate a 5-row preview.\nNothing will be inserted."))
    } else {
        if len(m.PreviewCols) > 0 {
            var hparts []string
            for _, col := range m.PreviewCols {
                hparts = append(hparts, highlightStyle.Render(fmt.Sprintf("%-18s", truncate(col, 17))))
            }
            sb.WriteString(strings.Join(hparts, dimStyle.Render(" │ ")) + "\n")
            sb.WriteString(dimStyle.Render(strings.Repeat("-", width)) + "\n")

            for i, row := range m.PreviewRows {
                if i < m.PreviewScroll { continue }
                var rparts []string
                for _, col := range m.PreviewCols {
                    v := "NULL"
                    if row[col] != nil { v = fmt.Sprintf("%v", row[col]) }
                    rparts = append(rparts, valueStyle.Render(fmt.Sprintf("%-18s", truncate(v, 17))))
                }
                sb.WriteString(strings.Join(rparts, dimStyle.Render(" │ ")) + "\n")
            }
        }
        sb.WriteString("\n" + dimStyle.Render("↑↓ scroll  •  read-only"))
    }
    return panelStyle.Width(width).Render(sb.String())
}

func (m Model) renderHistoryTab() string {
    var sb strings.Builder
    width := m.Width - 6
    sb.WriteString(titleStyle.Render("Seed History") + "\n\n")

    if len(m.History) == 0 {
        sb.WriteString(dimStyle.Render("No runs yet.\nHistory appears here after you seed a database."))
    } else {
        for i, h := range m.History {
            if i < m.HistoryScroll { continue }
            icon := successStyle.Render("✓")  // checkmark
            if !h.Success { icon = errorStyle.Render("✗") }  // X mark
            ts     := dimStyle.Render(h.Timestamp.Format("Jan 02 15:04"))
            schema := valueStyle.Render(truncate(h.SchemaFile, 25))
            var stats string
            if h.Success {
                stats = successStyle.Render(fmt.Sprintf("%d rows  %s", h.TotalRows, h.Duration.Round(time.Second)))
            } else {
                stats = errorStyle.Render(truncate(h.ErrMsg, 30))
            }
            sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, icon,"  ",ts,"  ",schema,"  ",stats) + "\n")
            if i < len(m.History)-1 {
                sb.WriteString(dimStyle.Render(strings.Repeat("-", width-4)) + "\n")
            }
        }
    }
    return panelStyle.Width(width).Render(sb.String())
}

func (m Model) renderHelpTab() string {
    var sb strings.Builder
    sb.WriteString(titleStyle.Render("Keyboard Shortcuts") + "\n\n")

    sections := []struct {
        title string
        keys  [][2]string
    }{
        {"Navigation", [][2]string{
            {"Tab / Shift+Tab", "Switch tabs"},
            {"Shift+1,2,3,4",  "Jump to tab"},
            {"Shift+j / k",        "Navigate fields / scroll"},
            {"Esc",             "Blur text fields"},
            {"Shift+q / Ctrl+C",      "Quit"},
        }},
        {"Generate Tab", [][2]string{
            {"Shift+i / j", "Focus next field"},
            {"Shift+l / k", "Focus previous field"},
            {"Enter", "Start seed pipeline"},
        }},
        {"Config Fields", [][2]string{
            {"Schema",   "Path to your .sql file"},
            {"Database", "postgres://... or sqlite:./dev.db"},
            {"AI Model", "Ollama model (deepseek-r1:7b)"},
            {"Rows",     "Rows per table (default 100)"},
        }},
        {"Data Styles", [][2]string{
            {"realistic",  "Real names, emails, prices"},
            {"minimal",    "Short simple ASCII values"},
            {"edge-cases", "NULLs, max lengths, boundaries"},
        }},
    }

    for _, sec := range sections {
        sb.WriteString(lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("  "+sec.title) + "\n")
        for _, pair := range sec.keys {
            sb.WriteString("  " + keyStyle.Width(22).Render(pair[0]) + keyDescStyle.Render(pair[1]) + "\n")
        }
        sb.WriteString("\n")
    }
    return panelStyle.Width(m.Width - 6).Render(sb.String())
}

func (m Model) renderStatusBar() string {
    var style lipgloss.Style
    switch m.StatusKind {
    case "success": style = successStyle
    case "error":   style = errorStyle
    case "warning": style = warningStyle
    default:        style = dimStyle
    }
    return lipgloss.NewStyle().
        Width(m.Width-4).
        Border(lipgloss.NormalBorder(), true, false, false, false).
        BorderForeground(colorBorder).
        Render(style.Render("  " + m.StatusMsg))
}

func (m Model) renderKeyBar() string {
    keys := []string{
        RenderKeyBinding("Tab","switch"),
        RenderKeyBinding("Shift+j/k","navigate"),
        RenderKeyBinding("Enter","run"),
        RenderKeyBinding("Shift+1-4","tabs"),
        RenderKeyBinding("Esc","blur"),
        RenderKeyBinding("Shift+q","quit"),
    }
    return dimStyle.Width(m.Width-4).Render("  " + strings.Join(keys, dimStyle.Render("  │  ")))
}

func truncate(s string, n int) string {
    if len(s) <= n { return s }
    return s[:n-1] + "…"  // ellipsis
}

func maxInt(a, b int) int {
    if a > b { return a }
    return b
}


