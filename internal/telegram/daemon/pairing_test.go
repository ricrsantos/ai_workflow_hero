package daemon

import (
	"testing"
	"time"
)

func TestPairingBeginValidate(t *testing.T) {
	now := time.Now()
	pm := newPairingManager(func() time.Time { return now })
	code := pm.begin()
	if len(code) != pairingCodeDigits {
		t.Fatalf("code=%q want %d digits", code, pairingCodeDigits)
	}
	matched, expired, _ := pm.validate("/start " + code)
	if !matched || expired {
		t.Fatalf("matched=%v expired=%v", matched, expired)
	}
	matched, expired, _ = pm.validate(code) // bare code also accepted
	if !matched || expired {
		t.Fatalf("bare code: matched=%v expired=%v", matched, expired)
	}
}

func TestPairingExpiresAfterTenMinutes(t *testing.T) {
	now := time.Now()
	pm := newPairingManager(func() time.Time { return now })
	code := pm.begin()

	// Eleven minutes later the code is invalid and cleared.
	now = now.Add(11 * time.Minute)
	matched, expired, _ := pm.validate(code)
	if matched || !expired {
		t.Fatalf("matched=%v expired=%v want expired", matched, expired)
	}
	if pm.active() != "" {
		t.Fatal("expired code should be invalidated")
	}
}

func TestPairingWrongCode(t *testing.T) {
	now := time.Now()
	pm := newPairingManager(func() time.Time { return now })
	pm.begin()
	matched, expired, _ := pm.validate("999999")
	if matched || expired {
		t.Fatalf("wrong code: matched=%v expired=%v", matched, expired)
	}
}

func TestPairingCancel(t *testing.T) {
	now := time.Now()
	pm := newPairingManager(func() time.Time { return now })
	pm.begin()
	pm.cancel()
	if pm.active() != "" {
		t.Fatal("cancelled code should not be active")
	}
	matched, expired, _ := pm.validate("anything")
	if matched || expired {
		t.Fatalf("no active code: matched=%v expired=%v", matched, expired)
	}
}

func TestRandomCodeLength(t *testing.T) {
	for i := 0; i < 20; i++ {
		if got := randomCode(pairingCodeDigits); len(got) != pairingCodeDigits {
			t.Fatalf("code=%q length %d", got, len(got))
		}
	}
}
