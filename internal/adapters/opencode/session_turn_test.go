package opencode

import (
	"testing"
)

func TestParseSessionTurn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want sessionTurn
	}{
		{name: "idle nested", body: `{"status":{"type":"idle"}}`, want: sessionTurnIdle},
		{name: "busy nested", body: `{"status":{"type":"busy"}}`, want: sessionTurnBusy},
		{name: "idle string", body: `{"status":"idle"}`, want: sessionTurnIdle},
		{name: "unknown", body: `{"id":"ses_1"}`, want: sessionTurnUnknown},
		{name: "invalid", body: `{`, want: sessionTurnUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseSessionTurn([]byte(tc.body)); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
