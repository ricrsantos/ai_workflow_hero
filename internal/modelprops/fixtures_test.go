package modelprops

import (
	"testing"
)

// TestSyntheticFixtureFiles loads the capability-only/partial/absent fixture
// files shipped under testdata (PRD-C05-001 §7).
func TestSyntheticFixtureFiles(t *testing.T) {
	cat := LoadCatalogFromDir("testdata")

	complete, ok := cat.CatalogValues("fixture/complete", "th")
	if !ok || !complete.Available || len(complete.Values) != 3 || complete.Default != "max" {
		t.Fatalf("complete fixture: %+v", complete)
	}
	ef, ok := cat.CatalogValues("fixture/complete", "ef")
	if !ok || !ef.Available || len(ef.Values) != 3 {
		t.Fatalf("complete ef fixture: %+v", ef)
	}

	th, ok := cat.CatalogValues("fixture/partial", "th")
	if !ok || !th.Available {
		t.Fatalf("partial th fixture: %+v", th)
	}
	pef, ok := cat.CatalogValues("fixture/partial", "ef")
	if !ok || pef.Available {
		t.Fatalf("partial ef must be unavailable: %+v", pef)
	}
	if !cat.HasModel("fixture/absent") {
		t.Fatal("absent fixture model missing")
	}
	if p, ok := cat.CatalogValues("fixture/absent", "fs"); ok && p.HasProperty {
		t.Fatal("absent fixture must carry no property metadata")
	}
}
