package operation_viewport

import "github.com/charmbracelet/lipgloss"

// Style controls the rendered appearance of the viewport. Defaults match
// clown's original tent_loader (cyan spinner, dim adaptive tail, green
// success, red failure) so the visual language stays consistent across
// tools that adopt this primitive.
type Style struct {
	Spinner lipgloss.Style
	Tail    lipgloss.Style
	Success lipgloss.Style
	Failure lipgloss.Style
}

// DefaultStyle returns the opinionated palette.
func DefaultStyle() Style {
	return Style{
		Spinner: lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
		Tail: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "240", Dark: "245"}).
			PaddingLeft(2),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		Failure: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	}
}
