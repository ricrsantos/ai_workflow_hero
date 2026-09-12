package telegram

import "testing"

func TestIsProjectControlCommand(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"/hero-add-todo", true},
		{"/hero-add-todo find-qa-1", true},
		{"/HERO-COMPLETE-TODO todo-1", true},
		{"/hero-status", false},
		{"hello", false},
	}
	for _, tc := range cases {
		if got := IsProjectControlCommand(tc.text); got != tc.want {
			t.Fatalf("IsProjectControlCommand(%q)=%v want %v", tc.text, got, tc.want)
		}
	}
}
