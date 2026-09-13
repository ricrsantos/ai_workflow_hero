package findingrepro

import "testing"

func TestCanonicalPackage(t *testing.T) {
	got, err := CanonicalPackage("internal/tui/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "./internal/tui" {
		t.Fatalf("got %q", got)
	}
	got, err = CanonicalPackage("./internal/tui")
	if err != nil || got != "./internal/tui" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := CanonicalPackage("../secret"); err == nil {
		t.Fatal("expected parent traversal rejection")
	}
	if _, err := CanonicalPackage("/abs"); err == nil {
		t.Fatal("expected absolute rejection")
	}
	empty, err := CanonicalPackage("  ")
	if err != nil || empty != "" {
		t.Fatalf("empty=%q err=%v", empty, err)
	}
}

func TestCanonicalTest(t *testing.T) {
	got, err := CanonicalTest("TestFindQA25")
	if err != nil || got != "TestFindQA25" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := CanonicalTest("TestFindQA.*"); err == nil {
		t.Fatal("expected regexp rejection")
	}
	if _, err := CanonicalTest("findQA"); err == nil {
		t.Fatal("expected Test prefix requirement")
	}
}

func TestSourceDeclaresTest(t *testing.T) {
	src := "package tui\n\nfunc TestFindQA25(t *testing.T) {\n\tt.Fatal(\"x\")\n}\n"
	if !SourceDeclaresTest(src, "TestFindQA25") {
		t.Fatal("expected declaration")
	}
	if err := ValidateSource(src, "TestFindQA25"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSource(src, "TestOther"); err == nil {
		t.Fatal("expected missing func rejection")
	}
}
