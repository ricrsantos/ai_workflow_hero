package cursor_test

import (
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

var grokFamilyModels = []string{
	"cursor-grok-4.6",
	"cursor-grok-4.6-low",
	"cursor-grok-4.6-low-fast",
	"cursor-grok-4.6-medium",
	"cursor-grok-4.6-high",
	"cursor-grok-4.6-xhigh",
}

func TestParseSlugPropertiesGrokLow(t *testing.T) {
	got := cursoradapter.ParseSlugProperties("cursor-grok-4.6-low")
	if got[harness.PropertyEffort] != "low" {
		t.Fatalf("ef=%q", got[harness.PropertyEffort])
	}
	if got[harness.PropertyFast] != "" {
		t.Fatalf("fs=%q want empty", got[harness.PropertyFast])
	}
}

func TestSlugLockedPropertiesGrokLow(t *testing.T) {
	locked := cursoradapter.SlugLockedProperties("cursor-grok-4.6-low")
	if locked[harness.PropertyEffort] != "low" || locked[harness.PropertyFast] != "false" {
		t.Fatalf("locked=%v", locked)
	}
}

func TestInferCapabilitiesGrokLowNoSelectable(t *testing.T) {
	caps := cursoradapter.InferCapabilitiesFromModelList(grokFamilyModels, "cursor-grok-4.6-low")
	if cursoradapter.HasSelectableCapability(caps) {
		t.Fatalf("locked variant must not be selectable: %+v", caps.Properties)
	}
	ef := caps.Property(harness.PropertyEffort)
	if ef == nil || ef.DefaultValue != "low" || ef.Available {
		t.Fatalf("ef=%+v", ef)
	}
}

func TestInferCapabilitiesComposerFastToggle(t *testing.T) {
	models := []string{"composer-2.5", "composer-2.5-fast"}
	caps := cursoradapter.InferCapabilitiesFromModelList(models, "composer-2.5")
	if !cursoradapter.HasSelectableCapability(caps) {
		t.Fatal("composer base must expose selectable fs")
	}
	fs := caps.Property(harness.PropertyFast)
	if fs == nil || !fs.Available || fs.DefaultValue != "false" {
		t.Fatalf("fs=%+v", fs)
	}
}

func TestInferCapabilitiesGrokBaseEffort(t *testing.T) {
	caps := cursoradapter.InferCapabilitiesFromModelList(grokFamilyModels, "cursor-grok-4.6")
	if !cursoradapter.HasSelectableCapability(caps) {
		t.Fatal("base grok slug should expose selectable effort")
	}
	ef := caps.Property(harness.PropertyEffort)
	if ef == nil || len(ef.AcceptedValues) < 3 {
		t.Fatalf("ef=%+v", ef)
	}
}

func TestBaseModelCandidatesStripVariants(t *testing.T) {
	got := cursoradapter.BaseModelCandidates("cursor-grok-4.6-low-fast")
	wantLast := "cursor-grok-4.6"
	if len(got) == 0 || got[len(got)-1] != wantLast {
		t.Fatalf("got=%v want last %q", got, wantLast)
	}
}
