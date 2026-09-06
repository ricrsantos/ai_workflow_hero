package daemon

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/telegram/ipc"
)

func TestRegistryAllocatesBaseThenSuffix(t *testing.T) {
	r := newRegistry()
	ch := make(chan ipc.Message, 1)
	c1, a1 := r.register("/p", ipc.ModeCycle, "MyProj!", ch)
	if a1 != "myproj" {
		t.Fatalf("a1=%q want myproj", a1)
	}
	c2, a2 := r.register("/p", ipc.ModeCycle, "myproj", ch)
	if a2 != "myproj_2" {
		t.Fatalf("a2=%q want myproj_2", a2)
	}
	_ = c1
	_ = c2
}

func TestRegistryReusesBaseAfterLastExit(t *testing.T) {
	r := newRegistry()
	ch := make(chan ipc.Message, 1)
	c1, a1 := r.register("/p", ipc.ModeCycle, "proj", ch)
	c2, a2 := r.register("/p", ipc.ModeCycle, "proj", ch)
	if a1 != "proj" || a2 != "proj_2" {
		t.Fatalf("unexpected addresses %q %q", a1, a2)
	}
	r.unregister(c1)
	r.unregister(c2)
	_, a3 := r.register("/p", ipc.ModeCycle, "proj", ch)
	if a3 != "proj" {
		t.Fatalf("a3=%q want proj after last exit", a3)
	}
}

func TestRegistryFreeChatAddresses(t *testing.T) {
	r := newRegistry()
	ch := make(chan ipc.Message, 1)
	_, a1 := r.register("", ipc.ModeFree, "", ch)
	_, a2 := r.register("", ipc.ModeFree, "", ch)
	if a1 != "free_1" || a2 != "free_2" {
		t.Fatalf("free chat addresses %q %q", a1, a2)
	}
}

func TestRegistryConcurrentRegistrationNoCollision(t *testing.T) {
	r := newRegistry()
	seen := make(map[string]bool)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := make(chan ipc.Message, 1)
			_, addr := r.register("/p", ipc.ModeCycle, "proj", ch)
			mu.Lock()
			if seen[addr] {
				t.Errorf("collision on address %q", addr)
			}
			seen[addr] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(seen) != 20 {
		t.Fatalf("expected 20 distinct addresses, got %d", len(seen))
	}
}

func TestNormalizeAbbrev(t *testing.T) {
	cases := map[string]string{
		"My Project!": "myproject",
		"ai_workflow": "ai_workflow",
		"  ":          "proj",
		"ÄBC":         "bc",
	}
	for in, want := range cases {
		if got := normalizeAbbrev(in); got != want {
			t.Errorf("normalizeAbbrev(%q)=%q want %q", in, got, want)
		}
	}
}

func TestRegistryLookupAndCount(t *testing.T) {
	r := newRegistry()
	ch := make(chan ipc.Message, 1)
	c, addr := r.register("/p", ipc.ModeCycle, "proj", ch)
	if r.count() != 1 {
		t.Fatalf("count=%d want 1", r.count())
	}
	if _, ok := r.lookup(addr); !ok {
		t.Fatalf("lookup(%q) failed", addr)
	}
	if _, ok := r.lookup("nope"); ok {
		t.Fatal("lookup(nope) should fail")
	}
	r.unregister(c)
	if r.count() != 0 {
		t.Fatalf("count=%d want 0 after unregister", r.count())
	}
}

func TestRegistryAddressesAreSorted(t *testing.T) {
	r := newRegistry()
	ch := make(chan ipc.Message, 1)
	_, _ = r.register("/p", ipc.ModeCycle, "zebra", ch)
	_, _ = r.register("/p", ipc.ModeCycle, "alpha", ch)

	if got := r.addresses(); fmt.Sprint(got) != "[alpha zebra]" {
		t.Fatalf("addresses=%v want sorted addresses", got)
	}
}

func TestPrefixAddress(t *testing.T) {
	if got := prefixAddress("ai_workflow_2", "Cycle #42 started."); got != fmt.Sprintf("ai_workflow_2: Cycle #42 started.") {
		t.Fatalf("unexpected prefix: %q", got)
	}
}
