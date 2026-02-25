package tui

import "github.com/charmbracelet/lipgloss"

var (
    colorCyan    = lipgloss.Color("#00D7FF")
    colorGreen   = lipgloss.Color("#00FF87")
    colorYellow  = lipgloss.Color("#FFD700")
    colorRed     = lipgloss.Color("#FF5F5F")
    colorPurple  = lipgloss.Color("#AF87FF")
    colorWhite   = lipgloss.Color("#FFFFFF")
    colorGray    = lipgloss.Color("#626262")
    colorDimGray = lipgloss.Color("#3A3A3A")
    colorBg      = lipgloss.Color("#1A1A2E")
    colorBorder  = lipgloss.Color("#2A2A4A")
)

var panelStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(colorBorder).
    Padding(0, 1)

var activePanelStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(colorCyan).
    Padding(0, 1)

var tabStyle = lipgloss.NewStyle().
    Foreground(colorGray).
    Padding(0, 2)

var activeTabStyle = lipgloss.NewStyle().
    Foreground(colorCyan).
    Bold(true).
    Padding(0, 2).
    Border(lipgloss.NormalBorder(), false, false, true, false).
    BorderForeground(colorCyan)

var titleStyle       = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
var labelStyle       = lipgloss.NewStyle().Foreground(colorGray).Width(14)
var valueStyle       = lipgloss.NewStyle().Foreground(colorWhite)
var successStyle     = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
var errorStyle       = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
var warningStyle     = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
var dimStyle         = lipgloss.NewStyle().Foreground(colorGray)
var highlightStyle   = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
var spinnerStyle     = lipgloss.NewStyle().Foreground(colorPurple)
var badgeDone        = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
var badgeRunning     = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
var badgeWaiting     = lipgloss.NewStyle().Foreground(colorGray)
var badgeError       = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
var keyStyle         = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
var keyDescStyle     = lipgloss.NewStyle().Foreground(colorGray)

const (
    barFull  = "█"
    barEmpty = "░"
    barWidth = 20
)

func RenderProgressBar(pct float64) string {
    if pct < 0 { pct = 0 }
    if pct > 1 { pct = 1 }
    filled := int(pct * float64(barWidth))
    empty  := barWidth - filled
    bar := ""
    for i := 0; i < filled; i++ { bar += barFull }
    for i := 0; i < empty;  i++ { bar += barEmpty }
    var style lipgloss.Style
    switch {
    case pct >= 1.0:
        style = lipgloss.NewStyle().Foreground(colorGreen)
    case pct > 0:
        style = lipgloss.NewStyle().Foreground(colorYellow)
    default:
        style = lipgloss.NewStyle().Foreground(colorDimGray)
    }
    return style.Render(bar)
}

func RenderKeyBinding(key, desc string) string {
    return keyStyle.Render(key) + keyDescStyle.Render(" "+desc)
}


