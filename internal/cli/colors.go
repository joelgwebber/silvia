package cli

import (
	"github.com/charmbracelet/lipgloss"
)

// Color styles for consistent output
var (
	// Status indicators
	SuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Bold(true)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	WarningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("214"))

	InfoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	// Entity types
	PersonStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("213"))

	OrgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	ConceptStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("228"))

	WorkStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("141"))

	EventStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("208"))

	// UI elements
	HeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Underline(true).
		Foreground(lipgloss.Color("99"))

	SubheaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("243"))

	DimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	HighlightStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212"))

	URLStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Underline(true)
		
	PromptStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)

	// Queue priority
	HighPriorityStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	MedPriorityStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("214"))

	LowPriorityStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("243"))
)

// FormatSuccess formats a success message
func FormatSuccess(msg string) string {
	return SuccessStyle.Render("✅ " + msg)
}

// FormatError formats an error message
func FormatError(msg string) string {
	return ErrorStyle.Render("❌ " + msg)
}

// FormatWarning formats a warning message
func FormatWarning(msg string) string {
	return WarningStyle.Render("⚠️  " + msg)
}

// FormatInfo formats an info message
func FormatInfo(msg string) string {
	return InfoStyle.Render("ℹ️  " + msg)
}

// FormatPriority formats a priority level with appropriate color
func FormatPriority(priority SourcePriority) string {
	switch priority {
	case PriorityHigh:
		return HighPriorityStyle.Render("HIGH")
	case PriorityMedium:
		return MedPriorityStyle.Render("MED")
	case PriorityLow:
		return LowPriorityStyle.Render("LOW")
	default:
		return DimStyle.Render("UNKNOWN")
	}
}