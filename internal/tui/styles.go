package tui

import "github.com/charmbracelet/lipgloss"

var (
	colAccent = lipgloss.AdaptiveColor{Light: "#5A3FC0", Dark: "#A78BFA"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#8B8FA3"}
	colWeak   = lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#FB923C"}
	colOK     = lipgloss.AdaptiveColor{Light: "#A16207", Dark: "#FACC15"}
	colStrong = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	colError  = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}

	appStyle    = lipgloss.NewStyle().Padding(1, 2)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	mutedStyle  = lipgloss.NewStyle().Foreground(colMuted)
	errorStyle  = lipgloss.NewStyle().Foreground(colError)
	okStyle     = lipgloss.NewStyle().Foreground(colStrong)
	headerStyle = lipgloss.NewStyle().Bold(true)
	selStyle    = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	passStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 2).
			Bold(true)
)

func ratingColor(bits float64) lipgloss.AdaptiveColor {
	switch {
	case bits < 60:
		return colWeak
	case bits < 80:
		return colOK
	default:
		return colStrong
	}
}
