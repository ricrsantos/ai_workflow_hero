package daemon

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestQAImageDeliveryUsesRequiredLeveledLogging(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "image_delivery.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var unsupported []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch selector.Sel.Name {
		case "Debug", "Info", "Error":
			seen[selector.Sel.Name] = true
		case "Warn":
			unsupported = append(unsupported, "Warn")
		}
		return true
	})
	if len(unsupported) > 0 {
		t.Fatalf("unsupported warn-level logging present: %v", unsupported)
	}
	for _, level := range []string{"Debug", "Info", "Error"} {
		if !seen[level] {
			t.Errorf("missing required %s-level logging", level)
		}
	}
}
