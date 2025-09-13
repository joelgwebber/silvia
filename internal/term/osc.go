// Package term provides terminal utilities including OSC (Operating System Command) sequences
package term

import (
	"fmt"
	"io"
	"os"
)

// ModeType represents the terminal UI mode
type ModeType string

const (
	ModeAppend      ModeType = "append"      // Normal line-by-line output
	ModeInteractive ModeType = "interactive" // TUI with cursor movement
	ModeInput       ModeType = "input"       // Waiting for user input
	ModeProcessing  ModeType = "processing"  // Long-running operation
)

// OSCWriter wraps an io.Writer to inject OSC sequences
type OSCWriter struct {
	writer      io.Writer
	currentMode ModeType
	modeOutput  io.Writer // Where to send OSC sequences (usually stderr)
}

// NewOSCWriter creates a new OSC-aware writer
func NewOSCWriter(w io.Writer) *OSCWriter {
	return &OSCWriter{
		writer:      w,
		currentMode: ModeAppend,
		modeOutput:  os.Stderr,
	}
}

// SetMode changes the current mode and emits an OSC sequence
func (o *OSCWriter) SetMode(mode ModeType) {
	if o.currentMode != mode {
		o.currentMode = mode
		// OSC 51 is a private-use sequence we're defining for mode changes
		// Format: ESC ] 51 ; key=value BEL
		fmt.Fprintf(o.modeOutput, "\033]51;mode=%s\007", mode)
	}
}

// Write implements io.Writer
func (o *OSCWriter) Write(p []byte) (n int, err error) {
	return o.writer.Write(p)
}

// EnterInteractive signals entering an interactive UI
func (o *OSCWriter) EnterInteractive(context string) {
	o.SetMode(ModeInteractive)
	if context != "" {
		fmt.Fprintf(o.modeOutput, "\033]51;context=%s\007", context)
	}
}

// ExitInteractive signals returning to normal mode
func (o *OSCWriter) ExitInteractive() {
	o.SetMode(ModeAppend)
}

// StartProcessing signals a long-running operation
func (o *OSCWriter) StartProcessing(operation string) {
	o.SetMode(ModeProcessing)
	if operation != "" {
		fmt.Fprintf(o.modeOutput, "\033]51;operation=%s\007", operation)
	}
}

// EndProcessing signals operation complete
func (o *OSCWriter) EndProcessing() {
	o.SetMode(ModeAppend)
}

// SendMetadata sends arbitrary metadata via OSC
func (o *OSCWriter) SendMetadata(key, value string) {
	fmt.Fprintf(o.modeOutput, "\033]51;%s=%s\007", key, value)
}

// Helper functions for common terminal operations

// ClearScreen clears the terminal screen
func ClearScreen(w io.Writer) {
	fmt.Fprint(w, "\033[2J\033[H")
}

// MoveCursor moves cursor to row, col (1-indexed)
func MoveCursor(w io.Writer, row, col int) {
	fmt.Fprintf(w, "\033[%d;%dH", row, col)
}

// ClearLine clears the current line
func ClearLine(w io.Writer) {
	fmt.Fprint(w, "\033[2K")
}

// SaveCursor saves the current cursor position
func SaveCursor(w io.Writer) {
	fmt.Fprint(w, "\033[s")
}

// RestoreCursor restores the saved cursor position
func RestoreCursor(w io.Writer) {
	fmt.Fprint(w, "\033[u")
}

// EnterAlternateScreen switches to alternate screen buffer
func EnterAlternateScreen(w io.Writer) {
	fmt.Fprint(w, "\033[?1049h")
}

// ExitAlternateScreen returns to main screen buffer
func ExitAlternateScreen(w io.Writer) {
	fmt.Fprint(w, "\033[?1049l")
}

// Color codes for terminal output
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
)

// Colorize wraps text with color codes
func Colorize(text, color string) string {
	return color + text + Reset
}
