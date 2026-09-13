package reports

import (
	"encoding/json"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
)

// decodeFindingRepro validates the repro identity of one failure entry against
// the cycle's repro policy. The policy decides which modes are available, so a
// Go project keeps the strict `go test` gate while another stack uses its own
// configured command (or, where no re-run can exist, evidence).
func decodeFindingRepro(item object, prefix, sourceStage string, policy findingrepro.Policy, evidence []string) (FindingRepro, *DiagnosticError) {
	raw, ok := item["repro"]
	if !ok || string(raw) == "null" {
		return FindingRepro{}, diag(CodeMissingField, prefix+"repro", "", "repro is required")
	}
	var fields object
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro", truncateValue(string(raw)), "repro must be an object")
	}
	allowed := map[string]struct{}{"mode": {}, "package": {}, "test": {}, "source": {}}
	if err := unknownFields(fields, allowed, prefix+"repro."); err != nil {
		return FindingRepro{}, err
	}

	modeRaw := ""
	if rawMode, ok := fields["mode"]; ok && string(rawMode) != "null" {
		var m string
		if json.Unmarshal(rawMode, &m) != nil {
			return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro.mode", truncateValue(string(rawMode)), "repro.mode must be a string")
		}
		modeRaw = m
	}
	mode, merr := policy.ResolveMode(modeRaw, sourceStage)
	if merr != nil {
		return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro.mode", strings.TrimSpace(modeRaw), merr.Error())
	}

	if mode == findingrepro.ModeEvidence {
		return decodeEvidenceRepro(fields, prefix, evidence)
	}

	pkgRaw, err := requireString(fields, "package")
	if err != nil {
		err.Field = prefix + "repro.package"
		return FindingRepro{}, err
	}
	testRaw, err := requireString(fields, "test")
	if err != nil {
		err.Field = prefix + "repro.test"
		return FindingRepro{}, err
	}
	pkg, test, ierr := findingrepro.CanonicalIdentity(mode, pkgRaw, testRaw)
	if ierr != nil {
		field, value := prefix+"repro.package", pkgRaw
		if strings.Contains(ierr.Error(), "repro test") {
			field, value = prefix+"repro.test", testRaw
		}
		return FindingRepro{}, diag(CodeInvalidEnum, field, value, ierr.Error())
	}
	if pkg == "" {
		return FindingRepro{}, diag(CodeMissingField, prefix+"repro.package", "", "repro.package is required")
	}
	if test == "" {
		return FindingRepro{}, diag(CodeMissingField, prefix+"repro.test", "", "repro.test is required")
	}

	source := ""
	if mode == findingrepro.ModeGoTest {
		source, err = requireString(fields, "source")
		if err != nil {
			err.Field = prefix + "repro.source"
			return FindingRepro{}, err
		}
	} else if rawSource, ok := fields["source"]; ok && string(rawSource) != "null" {
		var s string
		if json.Unmarshal(rawSource, &s) != nil {
			return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro.source", truncateValue(string(rawSource)), "repro.source must be a string")
		}
		source = strings.TrimSpace(s)
	}
	if source != "" {
		if redact.HasToken(source) {
			return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro.source", "", "repro.source contains a forbidden token pattern")
		}
		if serr := findingrepro.ValidateSourceForMode(mode, source, test); serr != nil {
			return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro.source", "", serr.Error())
		}
	}

	return FindingRepro{Mode: mode, Package: pkg, Test: test, Source: strings.TrimSpace(source)}, nil
}

func decodeEvidenceRepro(fields object, prefix string, evidence []string) (FindingRepro, *DiagnosticError) {
	for _, name := range []string{"package", "test", "source"} {
		if raw, ok := fields[name]; ok && string(raw) != "null" {
			var value string
			if json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != "" {
				return FindingRepro{}, diag(CodeInvalidEnum, prefix+"repro."+name, truncateValue(value),
					"evidence-mode repro must not set package, test, or source")
			}
		}
	}
	if len(evidence) == 0 {
		return FindingRepro{}, diag(CodeMissingField, prefix+"evidence", "",
			"evidence-mode repro requires a non-empty evidence array")
	}
	return FindingRepro{Mode: findingrepro.ModeEvidence}, nil
}
