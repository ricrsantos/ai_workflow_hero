// Package output provides semantic output helpers for the Hero CLI.
// Icons and colors respect NO_COLOR/TTY settings (UI §2).
package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// IsTerminal reports whether w is an interactive terminal with color support.
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

// Success prints a success message. ✓ (green) or [OK] in plain mode.
func Success(w io.Writer, msg string) {
	if IsTerminal(w) {
		fmt.Fprintf(w, "\033[32m✓\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(w, "[OK] %s\n", msg)
	}
}

// Successf prints a formatted success message.
func Successf(w io.Writer, format string, args ...any) {
	Success(w, fmt.Sprintf(format, args...))
}

// Warning prints a warning message. ⚠ (yellow) or [WARN] in plain mode.
func Warning(w io.Writer, msg string) {
	if IsTerminal(w) {
		fmt.Fprintf(w, "\033[33m⚠\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(w, "[WARN] %s\n", msg)
	}
}

// Warningf prints a formatted warning message.
func Warningf(w io.Writer, format string, args ...any) {
	Warning(w, fmt.Sprintf(format, args...))
}

// Progress prints a progress/info message. → (blue) or [INFO] in plain mode.
func Progress(w io.Writer, msg string) {
	if IsTerminal(w) {
		fmt.Fprintf(w, "\033[34m→\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(w, "[INFO] %s\n", msg)
	}
}

// Progressf prints a formatted progress message.
func Progressf(w io.Writer, format string, args ...any) {
	Progress(w, fmt.Sprintf(format, args...))
}

// Table renders a simple ASCII table to w.
// headers is the list of column names; rows is a slice of string slices (one per row).
func Table(w io.Writer, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	separator := buildSeparator(widths)
	fmt.Fprintln(w, separator)
	fmt.Fprintln(w, buildRow(headers, widths))
	fmt.Fprintln(w, separator)
	for _, row := range rows {
		fmt.Fprintln(w, buildRow(row, widths))
	}
	fmt.Fprintln(w, separator)
}

func buildSeparator(widths []int) string {
	var sb strings.Builder
	sb.WriteString("+")
	for _, w := range widths {
		sb.WriteString(strings.Repeat("-", w+2))
		sb.WriteString("+")
	}
	return sb.String()
}

func buildRow(cells []string, widths []int) string {
	var sb strings.Builder
	sb.WriteString("|")
	for i, w := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		sb.WriteString(" ")
		sb.WriteString(cell)
		sb.WriteString(strings.Repeat(" ", w-len(cell)))
		sb.WriteString(" |")
	}
	return sb.String()
}
