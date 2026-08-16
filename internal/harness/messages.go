package harness

import (
	"fmt"
	"strings"
)

// SupportedToolIDs returns harness identifiers Hero supports in this version (ADR-034).
func SupportedToolIDs() []string {
	return []string{"cursor", "opencode"}
}

// UnsupportedMarkerWarningLine formats the primary doctor/install warning line (UI-C02-001 §5).
func UnsupportedMarkerWarningLine(m MarkerDir, inCLITools bool) string {
	return "⚠ " + UnsupportedMarkerWarningText(m, inCLITools)
}

// UnsupportedMarkerWarningText is the warning body without the leading icon (for output.Warning).
func UnsupportedMarkerWarningText(m MarkerDir, inCLITools bool) string {
	dir := m.Dir
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	if inCLITools {
		return fmt.Sprintf("Detected %s (unsupported in this Hero version — not installed).", dir)
	}
	return fmt.Sprintf("Detected %s but cli.tools does not include it (unsupported in this Hero version — not installed).", dir)
}

// UnsupportedMarkerSuggestionLine formats the secondary hint line (UI-C02-001 §5).
func UnsupportedMarkerSuggestionLine() string {
	return "→ " + UnsupportedMarkerSuggestionText()
}

// UnsupportedMarkerSuggestionText is the suggestion body without the leading arrow (for output.Progress).
func UnsupportedMarkerSuggestionText() string {
	return fmt.Sprintf("Supported today: %s. See docs for multi-harness roadmap (D1).", strings.Join(SupportedToolIDs(), ", "))
}

// UnsupportedMarkerMessage returns the full two-line warning text for an unsupported marker.
func UnsupportedMarkerMessage(m MarkerDir, inCLITools bool) string {
	return UnsupportedMarkerWarningLine(m, inCLITools) + "\n" + UnsupportedMarkerSuggestionLine()
}
