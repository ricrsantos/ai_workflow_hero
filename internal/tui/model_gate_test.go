package tui

import "testing"

func TestDefaultModelRequiredMessage_HeroNew(t *testing.T) {
	got := defaultModelRequiredMessage("/hero-new")
	want := "Select a default model with /hero-model first, then run /hero-new again."
	if got != want {
		t.Fatalf("message=%q want %q", got, want)
	}
}
