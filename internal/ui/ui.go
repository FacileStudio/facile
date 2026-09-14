package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
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
	fmt.Fprintf(os.Stdout, "  %s\n", dimStyle.Sprintf(format, a...))
}

func line(w io.Writer, style *color.Color, glyph, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", style.Sprint(glyph), fmt.Sprintf(format, a...))
}
