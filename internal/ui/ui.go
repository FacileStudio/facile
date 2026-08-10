package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

var (
	stepStyle = color.New(color.FgCyan)
	okStyle   = color.New(color.FgGreen)
	warnStyle = color.New(color.FgYellow)
	errStyle  = color.New(color.FgRed)
	hintStyle = color.New(color.Faint)
	dimStyle  = color.New(color.Faint)
)

// SetColor forces colored output on or off, overriding TTY detection.
func SetColor(on bool) { color.NoColor = !on }

// Enabled reports whether colored output is currently being emitted.
func Enabled() bool { return !color.NoColor }

// Step announces work that is about to happen.
func Step(format string, a ...any) { line(os.Stdout, stepStyle, "▸", format, a...) }

// Success announces work that completed.
func Success(format string, a ...any) { line(os.Stdout, okStyle, "✓", format, a...) }

// Warn reports a degraded state the run is continuing through.
func Warn(format string, a ...any) { line(os.Stderr, warnStyle, "!", format, a...) }

// Error reports the failure a run is aborting on.
func Error(format string, a ...any) { line(os.Stderr, errStyle, "✗", format, a...) }

// Hint prints an indented dim explanation under the line it belongs to.
func Hint(format string, a ...any) {
	fmt.Fprintf(os.Stdout, "  %s\n", hintStyle.Sprintf(format, a...))
}

// Prompt asks for input on stderr with no trailing newline, keeping stdout
// clean for the data a caller may be piping.
func Prompt(format string, a ...any) { fmt.Fprintf(os.Stderr, format, a...) }

// Dim renders text in the dim style without printing it.
func Dim(s string) string { return dimStyle.Sprint(s) }

// Out prints data to stdout with no glyph, for output a script would consume.
func Out(format string, a ...any) { fmt.Fprintf(os.Stdout, format+"\n", a...) }

func line(w io.Writer, style *color.Color, glyph, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", style.Sprint(glyph), fmt.Sprintf(format, a...))
}
