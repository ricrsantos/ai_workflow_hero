// Package clierr provides structured CLI error types and formatting for the Hero CLI.
// All errors follow the UI §5 convention:
//
//	✗ <description>
//
//	  Suggestion: <fix>
//
//	(exit code: 1)
//
// When NO_COLOR is set or stdout is not a TTY, the ✗ icon is replaced by [ERROR].
package clierr

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// HeroError is a structured CLI error with an optional suggestion.
type HeroError struct {
	Description string
	Suggestion  string
}

func (e *HeroError) Error() string {
	return e.Description
}

// New creates a HeroError with a description but no suggestion.
func New(description string) *HeroError {
	return &HeroError{Description: description}
}

// NewWithSuggestion creates a HeroError with both a description and a suggestion.
func NewWithSuggestion(description, suggestion string) *HeroError {
	return &HeroError{Description: description, Suggestion: suggestion}
}

// Newf creates a HeroError using a format string.
func Newf(format string, args ...any) *HeroError {
	return &HeroError{Description: fmt.Sprintf(format, args...)}
}

// IsTerminal reports whether w is an interactive terminal.
func IsTerminal(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// Format formats a HeroError for display to the given writer.
// It respects NO_COLOR/TTY settings.
func Format(w io.Writer, err *HeroError) {
	isTerm := IsTerminal(w)

	var sb strings.Builder
	if isTerm {
		// Red ✗
		sb.WriteString("\033[31m✗\033[0m ")
	} else {
		sb.WriteString("[ERROR] ")
	}
	sb.WriteString(err.Description)
	sb.WriteString("\n")

	if err.Suggestion != "" {
		sb.WriteString("\n  Suggestion: ")
		sb.WriteString(err.Suggestion)
		sb.WriteString("\n")
	}
	sb.WriteString("\n(exit code: 1)\n")

	fmt.Fprint(w, sb.String())
}
