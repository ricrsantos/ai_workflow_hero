package daemon

import "testing"

func TestParseAddressed(t *testing.T) {
	addr, payload, ok := parseAddressed("myproj: /hero-status")
	if !ok || addr != "myproj" || payload != "/hero-status" {
		t.Fatalf("addr=%q payload=%q ok=%v", addr, payload, ok)
	}
}

func TestParseAddressedMalformed(t *testing.T) {
	for _, in := range []string{"no-colon", ":leading", "addr:", "", "   "} {
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
