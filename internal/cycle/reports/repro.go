package reports

import (
	"encoding/json"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
)

func decodeFindingRepro(item object, prefix string) (FindingRepro, *DiagnosticError) {
	raw, ok := item["repro"]
	if !ok || string(raw) == "null" {
		return FindingRepro{}, diag(CodeMissingField, prefix+"repro", "", "repro is required")
	}
	var fields object
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro", truncateValue(string(raw)), "repro must be an object")
	}
	allowed := map[string]struct{}{"package": {}, "test": {}, "source": {}}
	if err := unknownFields(fields, allowed, prefix+"repro."); err != nil {
		return FindingRepro{}, err
	}

	pkgRaw, err := requireString(fields, "package")
	if err != nil {
		err.Field = prefix + "repro.package"
		return FindingRepro{}, err
	}
	pkg, perr := findingrepro.CanonicalPackage(pkgRaw)
	if perr != nil || pkg == "" {
		return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro.package", pkgRaw, "repro.package must be a relative Go package path such as ./internal/tui")
	}

	testRaw, err := requireString(fields, "test")
	if err != nil {
		err.Field = prefix + "repro.test"
		return FindingRepro{}, err
	}
	test, terr := findingrepro.CanonicalTest(testRaw)
	if terr != nil || test == "" {
		return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro.test", testRaw, "repro.test must be a Go Test* function name")
	}

	source, err := requireString(fields, "source")
	if err != nil {
		err.Field = prefix + "repro.source"
		return FindingRepro{}, err
	}
	if redact.HasToken(source) {
		return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro.source", "", "repro.source contains a forbidden token pattern")
	}
	if serr := findingrepro.ValidateSource(source, test); serr != nil {
		return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro.source", "", serr.Error())
	}

	return FindingRepro{Package: pkg, Test: test, Source: strings.TrimSpace(source)}, nil
}
