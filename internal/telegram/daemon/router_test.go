package daemon

import "testing"

func TestParseAddressed(t *testing.T) {
	for _, tc := range []struct {
		in      string
		addr    string
		payload string
	}{
		{in: "myproj: /hero-status", addr: "myproj", payload: "/hero-status"},
		{in: "aiwkhero: hello", addr: "aiwkhero", payload: "hello"},
		{in: "aiwkhero_2: /status", addr: "aiwkhero_2", payload: "/status"},
		{in: "free_1: hi", addr: "free_1", payload: "hi"},
		{in: "https://example.com", addr: "https", payload: "//example.com"},
	} {
		addr, payload, ok := parseAddressed(tc.in)
		if !ok || addr != tc.addr || payload != tc.payload {
			t.Errorf("parseAddressed(%q)=(%q, %q, %v), want (%q, %q, true)",
				tc.in, addr, payload, ok, tc.addr, tc.payload)
		}
	}
}

func TestParseAddressedMalformed(t *testing.T) {
	prose := "Verifique porque está dando este erro na chamada do orchestration agente: aiwkhero: cursor agent execute failed: exit status 1"
	for _, in := range []string{
		"no-colon",
		":leading",
		"addr:",
		"",
		"   ",
		"Nota: faça X",
		prose,
		"MyProj: hello", // uppercase rejected; live addresses are lowercased
		"-bad: hi",
		"_bad: hi",
	} {
		if _, _, ok := parseAddressed(in); ok {
			t.Errorf("parseAddressed(%q) should fail", in)
		}
	}
}

func TestClassifyInbound(t *testing.T) {
	a, arg := classifyInbound("/telegram-cancel-pending")
	if a != actionCancelPending {
		t.Fatalf("action=%d want cancel", a)
	}
	a, arg = classifyInbound("/hero-status")
	if a != actionCommand || arg != "/hero-status" {
		t.Fatalf("action=%d arg=%q", a, arg)
	}
	a, arg = classifyInbound("hello there")
	if a != actionPlain || arg != "hello there" {
		t.Fatalf("action=%d arg=%q", a, arg)
	}
}

func TestParseSelect(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int
		ok   bool
	}{
		{text: "/select 1", want: 1, ok: true},
		{text: "/select 42", want: 42, ok: true},
		{text: "/select", ok: false},
		{text: "/select 0", ok: false},
		{text: "/select one", ok: false},
		{text: "/select 1 extra", ok: false},
	} {
		got, ok := parseSelect(tc.text)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseSelect(%q)=(%d, %v), want (%d, %v)", tc.text, got, ok, tc.want, tc.ok)
		}
	}
}
