package install

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintSetupHeader_Plain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	printSetupHeader(&buf)
	got := buf.String()
	if strings.Contains(got, "🚀") {
		t.Errorf("expected no rocket emoji with NO_COLOR, got %q", got)
	}
	if !strings.Contains(got, "Hero Project Setup") {
		t.Errorf("expected setup header, got %q", got)
	}
}

func TestHeroInstallTheme_NoLeftBorder(t *testing.T) {
	theme := heroInstallTheme()
	if theme == nil {
		t.Fatal("theme is nil")
	}
	base := theme.Focused.Base.String()
	if strings.Contains(base, "│") {
		t.Errorf("focused base should not render a left border character, got %q", base)
	}
}
