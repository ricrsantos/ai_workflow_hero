package conversation

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"
)

func TestTitleFreeChat(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		attachmentOnly bool
		want           string
	}{
		{"plain", "Deploy the API", false, "Deploy the API"},
		{"trim and collapse", "  hello\t\nworld  ", false, "hello world"},
		{"attachment only", "", true, titleImageConversation},
		{"whitespace only becomes image", "   \n\t  ", false, titleImageConversation},
		{"nfc", "café", false, "café"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TitleFreeChat(tc.text, tc.attachmentOnly)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestTitleFreeChat48Graphemes(t *testing.T) {
	text := strings.Repeat("a", 50)
	got := TitleFreeChat(text, false)
	if uniseg.GraphemeClusterCount(got) != 48 {
		t.Fatalf("grapheme count=%d title=%q", uniseg.GraphemeClusterCount(got), got)
	}
}

func TestCycleAwareTitles(t *testing.T) {
	if got := TitleOrchestration(3); got != "C3 · Orchestration · ORCH" {
		t.Fatalf("orchestration=%q", got)
	}
	if got := TitleResearch(3); got != "C3 · Research · DISC" {
		t.Fatalf("research=%q", got)
	}
	if got := TitleStageAgent(2, "implementation", "backend_agent"); got != "C2 · Implementation · BACK" {
		t.Fatalf("stage agent title=%q", got)
	}
	if got := TitleStageAgent(1, "qa", "qa_agent"); got != "C1 · QA · QA" {
		t.Fatalf("qa title=%q", got)
	}
}

func TestStageDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		stage string
		want  string
	}{
		{"known", "implementation", "Implementation"},
		{"accented", "café_etape", "Café Etape"},
		{"cjk", "日本語_review", "日本語 Review"},
		{"emoji", "🎨_design", "🎨 Design"},
		{"combining_nfd", "cafe\u0301_room", "Café Room"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := StageDisplayName(tc.stage)
			if norm.NFC.String(got) != norm.NFC.String(tc.want) {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("invalid UTF-8: %q", got)
			}
		})
	}
}
