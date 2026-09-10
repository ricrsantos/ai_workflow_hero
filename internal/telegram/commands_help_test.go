package telegram

import (
	"strings"
	"testing"
)

func TestIsHelpCommand(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"/help", true},
		{" /Help ", true},
		{"/HELP", true},
		{"/help me", false},
		{"help", false},
		{"/list", false},
	}
	for _, tc := range cases {
		if got := IsHelpCommand(tc.text); got != tc.want {
			t.Fatalf("IsHelpCommand(%q)=%v want %v", tc.text, got, tc.want)
		}
	}
}

func TestCommandHelpTextListsCoreCommands(t *testing.T) {
	text := CommandHelpText()
	for _, want := range []string{
		"/list",
		"/select",
		"/help",
		"/status",
		"/version",
		"/interrupt",
		"/kill",
		"/auto-update",
		"/model",
		"/hero-config",
		"/hero-permission",
		"/telegram-cancel-pending",
		"/hero-new",
		"/hero-start",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("CommandHelpText missing %q:\n%s", want, text)
		}
	}
}
