package gui

import "github.com/charmbracelet/lipgloss"

// Tron-like color constants
const (
	ColorCyan    = "51"
	ColorMagenta = "201"
	ColorGreen   = "46"
	ColorRed     = "196"
	ColorYellow  = "226"
	ColorBlack   = "0"
	ColorGrey    = "242"
)

// StyleBuilder provides fluent styling
type StyleBuilder struct {
	style lipgloss.Style
}

// NewStyle creates a new style builder
func NewStyle() StyleBuilder {
	return StyleBuilder{
		style: lipgloss.NewStyle(),
	}
}

func (s StyleBuilder) CyanForeground() StyleBuilder {
	s.style = s.style.Foreground(lipgloss.Color(ColorCyan))
	return s
}

func (s StyleBuilder) MagentaForeground() StyleBuilder {
	s.style = s.style.Foreground(lipgloss.Color(ColorMagenta))
	return s
}

func (s StyleBuilder) GreenForeground() StyleBuilder {
	s.style = s.style.Foreground(lipgloss.Color(ColorGreen))
	return s
}

func (s StyleBuilder) RedForeground() StyleBuilder {
	s.style = s.style.Foreground(lipgloss.Color(ColorRed))
	return s
}

func (s StyleBuilder) YellowForeground() StyleBuilder {
	s.style = s.style.Foreground(lipgloss.Color(ColorYellow))
	return s
}

func (s StyleBuilder) GreyForeground() StyleBuilder {
	s.style = s.style.Foreground(lipgloss.Color(ColorGrey))
	return s
}

func (s StyleBuilder) GreenBg() StyleBuilder {
	s.style = s.style.Background(lipgloss.Color(ColorGreen))
	return s
}

func (s StyleBuilder) MagentaBg() StyleBuilder {
	s.style = s.style.Background(lipgloss.Color(ColorMagenta))
	return s
}

func (s StyleBuilder) CyanBg() StyleBuilder {
	s.style = s.style.Background(lipgloss.Color(ColorCyan))
	return s
}

func (s StyleBuilder) BlackBg() StyleBuilder {
	s.style = s.style.Background(lipgloss.Color(ColorBlack))
	return s
}

func (s StyleBuilder) Bold() StyleBuilder {
	s.style = s.style.Bold(true)
	return s
}

func (s StyleBuilder) Padding(v, h int) StyleBuilder {
	s.style = s.style.Padding(v, h)
	return s
}

func (s StyleBuilder) Width(w int) StyleBuilder {
	s.style = s.style.Width(w)
	return s
}

func (s StyleBuilder) Height(h int) StyleBuilder {
	s.style = s.style.Height(h)
	return s
}

func (s StyleBuilder) Border(b lipgloss.Border) StyleBuilder {
	s.style = s.style.Border(b)
	return s
}

func (s StyleBuilder) BorderFg(color string) StyleBuilder {
	s.style = s.style.BorderForeground(lipgloss.Color(color))
	return s
}

func (s StyleBuilder) Render(content string) string {
	return s.style.Render(content)
}

func (s StyleBuilder) Build() lipgloss.Style {
	return s.style
}
