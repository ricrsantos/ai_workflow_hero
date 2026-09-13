package reprotest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPassFailAndMissing(t *testing.T) {
	root := t.TempDir()
	mod := []byte("module example.com/reprogate\n\ngo 1.22\n")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), mod, 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(root, "sample")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "sample.go"), []byte("package sample\n\nfunc Value() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	failSrc := "package sample\n\nimport \"testing\"\n\nfunc TestFindQAFail(t *testing.T) {\n\tt.Fatal(\"still broken\")\n}\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "fail_test.go"), []byte(failSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), root, Spec{Package: "./sample", Test: "TestFindQAFail"})
	if err == nil {
		t.Fatal("expected failing test to reject done")
	}
	if errors.Is(err, ErrNotRun) {
		t.Fatalf("failing test ran: %v", err)
	}

	passSrc := "package sample\n\nimport \"testing\"\n\nfunc TestFindQAPass(t *testing.T) {\n\tif Value() != 1 {\n\t\tt.Fatal(Value())\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "pass_test.go"), []byte(passSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), root, Spec{Package: "./sample", Test: "TestFindQAPass"}); err != nil {
		t.Fatalf("passing test: %v", err)
	}

	err = Run(context.Background(), root, Spec{Package: "./sample", Test: "TestFindQAMissing"})
	if !errors.Is(err, ErrNotRun) {
		t.Fatalf("missing test err=%v want ErrNotRun", err)
	}
	if !strings.Contains(err.Error(), "TestFindQAMissing") {
		t.Fatalf("error should name the test: %v", err)
	}
}
