package harness

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestNormalizePropertiesKeepsOnlyC5Keys(t *testing.T) {
	in := map[string]string{
		"fs":     "true",
		"th":     "max",
		"ef":     "high",
		"future": "x",
		"":       "ignored",
	}
	got := NormalizeProperties(in)
	want := map[string]string{"fs": "true", "th": "max", "ef": "high"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("key %s=%q want %q", k, got[k], v)
		}
	}
	if _, ok := got["future"]; ok {
		t.Fatal("future key must be filtered out of the request boundary")
	}
}

func TestNormalizePropertiesDropsEmptyAndTrims(t *testing.T) {
	got := NormalizeProperties(map[string]string{"fs": "  ", "th": " max ", "ef": ""})
	if len(got) != 1 || got["th"] != "max" {
		t.Fatalf("got %v", got)
	}
	if got = NormalizeProperties(nil); got != nil {
		t.Fatalf("nil input must stay nil, got %v", got)
	}
	if got = NormalizeProperties(map[string]string{}); got != nil {
		t.Fatalf("empty input must stay nil, got %v", got)
	}
}

func TestClonePropertiesIsDeep(t *testing.T) {
	src := map[string]string{"fs": "true", "ef": "high"}
	clone := CloneProperties(src)
	clone["fs"] = "false"
	if src["fs"] != "true" {
		t.Fatal("clone mutated the source map")
	}
	if CloneProperties(nil) != nil {
		t.Fatal("nil clone must be nil")
	}
}

func TestModelCapabilitiesPropertyLookup(t *testing.T) {
	c := ModelCapabilities{
		HarnessID: "opencode",
		ModelID:   "opencode-go/deepseek-v4-pro",
		Properties: []PropertyCapability{
			{Key: PropertyFast, AcceptedValues: []string{"true", "false"}, DefaultValue: "false", Available: true},
			{Key: PropertyThink, AcceptedValues: []string{"off", "max"}, DefaultValue: "off", Available: true},
		},
	}
	if p := c.Property(PropertyThink); p == nil || len(p.AcceptedValues) != 2 {
		t.Fatalf("th capability missing: %+v", c.Property(PropertyThink))
	}
	if p := c.Property(PropertyEffort); p != nil {
		t.Fatal("ef must be absent")
	}
}

// discoverStub exercises the optional discovery contract without a live harness.
type discoverStub struct{ caps ModelCapabilities }

func (d discoverStub) DiscoverModelProperties(_ context.Context, modelID string) (ModelCapabilities, error) {
	c := d.caps
	c.ModelID = modelID
	return c, nil
}

func TestModelPropertyDiscovererIsOptionalContract(t *testing.T) {
	var _ ModelPropertyDiscoverer = discoverStub{}
	var lister ModelLister = fakeLister{}
	// A lister without discovery keeps working: type assertion fails cleanly.
	if _, ok := interface{}(lister).(ModelPropertyDiscoverer); ok {
		t.Fatal("lister must not implement discovery")
	}
	ctx := context.Background()
	caps, err := (discoverStub{caps: ModelCapabilities{
		HarnessID:   "opencode",
		Properties:  []PropertyCapability{{Key: PropertyThink, AcceptedValues: []string{"max"}, Available: true}},
		RetrievedAt: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
	}}).DiscoverModelProperties(ctx, "opencode-go/deepseek-v4-pro")
	if err != nil {
		t.Fatal(err)
	}
	if caps.ModelID != "opencode-go/deepseek-v4-pro" || caps.Property(PropertyThink) == nil {
		t.Fatalf("normalized discovery result unexpected: %+v", caps)
	}
}

type fakeLister struct{}

func (fakeLister) ListModels(_ context.Context) ([]string, error) {
	return []string{"a", "b"}, nil
}

func TestPropertyRejectionErrorFormatting(t *testing.T) {
	err := PropertyRejection("ef", "opencode", "opencode-go/deepseek-v4-pro",
		fmt.Errorf("effort high not supported"))
	want := `property "ef" rejected by opencode for model "opencode-go/deepseek-v4-pro": effort high not supported`
	if err.Error() != want {
		t.Fatalf("got %q want %q", err.Error(), want)
	}
	var pre *PropertyRejectionError
	if !errors.As(err, &pre) {
		t.Fatal("errors.As must find PropertyRejectionError")
	}
	if pre.Property != "ef" || pre.Harness != "opencode" || pre.Model != "opencode-go/deepseek-v4-pro" {
		t.Fatalf("fields unexpected: %+v", pre)
	}
	if !IsPropertyRejection(err) {
		t.Fatal("IsPropertyRejection must be true")
	}
	// Wrapped errors are still identified.
	if !IsPropertyRejection(fmt.Errorf("wrap: %w", err)) {
		t.Fatal("wrapped rejection must be identified")
	}
	if IsPropertyRejection(errors.New("plain error")) {
		t.Fatal("plain error must not be a rejection")
	}
	// Bare rejection without harness/model stays readable.
	bare := (&PropertyRejectionError{Property: "th"}).Error()
	if bare != `property "th" rejected` {
		t.Fatalf("bare formatting got %q", bare)
	}
}

func TestExecuteRequestCarriesNormalizedProperties(t *testing.T) {
	req := ExecuteRequest{Model: "composer-2.5", Properties: map[string]string{"fs": "true", "ef": "high"}}
	if req.Properties["fs"] != "true" {
		t.Fatal("ExecuteRequest must expose Properties")
	}
}
