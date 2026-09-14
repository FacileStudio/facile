package ui

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

var (
	stepStyle = color.New(color.FgCyan)
	okStyle   = color.New(color.FgGreen)
	warnStyle = color.New(color.FgYellow)
	errStyle  = color.New(color.FgRed)
	dimStyle  = color.New(color.Faint)
)

// Prompt asks for input on stderr with no trailing newline, keeping stdout
// clean for the data a caller may be piping.
func Prompt(format string, a ...any) { fmt.Fprintf(os.Stderr, format, a...) }

// Out prints data to stdout with no glyph, for output a script would consume.
func Out(format string, a ...any) { fmt.Fprintf(os.Stdout, format+"\n", a...) }

// Dim renders text in the dim style without printing it.
func Dim(s string) string { return dimStyle.Sprint(s) }

// Accent renders text in the step style without printing it, for the one field
// on a line that the reader is meant to land on first.
func Accent(s string) string { return stepStyle.Sprint(s) }

// Good renders text in the success style without printing it, for a field
// stating something is healthy and up to date.
func Good(s string) string { return okStyle.Sprint(s) }

// Notice renders text in the warning style without printing it, for a field
// stating something is not a released version but is not broken either.
func Notice(s string) string { return warnStyle.Sprint(s) }

// Alert renders text in the error style without printing it, for a field
// stating something is wrong or unreadable.
func Alert(s string) string { return errStyle.Sprint(s) }
