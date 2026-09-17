package reports

import (
	"encoding/json"
)

// BrowserUIReport is the decoded Browser UI Validation payload.
type BrowserUIReport struct {
	Status       string
	HealthPassed *bool
	VisualRan    *bool
	VisualPassed *bool
	FailureClass *string
	Failures     []FailureEntry
	Warnings     []string
	ArtifactsDir string
	Summary      string
	// ContractWarnings is Hero's own tolerance record, distinct from the
	// agent-supplied Warnings above.
	ContractWarnings []ReportWarning
}

var browserUIAllowed = map[string]struct{}{
	"status": {}, "health_passed": {}, "visual_ran": {}, "visual_passed": {},
	"failure_class": {}, "failures": {}, "warnings": {}, "artifacts_dir": {}, "summary": {},
}

// DecodeBrowserUI validates and decodes a Browser UI Validation JSON report.
func DecodeBrowserUI(data []byte, ctx DecodeContext) (*BrowserUIReport, *DiagnosticError) {
	root, err := parseObject(data)
	if err != nil {
		return nil, err
	}
	var warnings []ReportWarning
	collect(&warnings, dropUnknownFields(root, browserUIAllowed, ""))

	status, err := parseValidationStatus(root)
	if err != nil {
		return nil, err
	}
	summary, err := requireString(root, "summary")
	if err != nil {
		return nil, err
	}

	report := &BrowserUIReport{Status: status, Summary: summary}
	report.HealthPassed, err = optionalBoolPtr(root, "health_passed")
	if err != nil {
		return nil, err
	}
	report.VisualRan, err = optionalBoolPtr(root, "visual_ran")
	if err != nil {
		return nil, err
	}
	report.VisualPassed, err = optionalBoolPtr(root, "visual_passed")
	if err != nil {
		return nil, err
	}
	report.FailureClass, err = optionalStringPtr(root, "failure_class")
	if err != nil {
		return nil, err
	}

	rawFailures, ok := root["failures"]
	if !ok {
		return nil, diag(CodeMissingField, "failures", "", "field is required")
	}
	failures, err := decodeFailureEntries(rawFailures, "failures", failureEntryOptions{
		sourceStage:        SourceBrowserUI,
		deriveBrowserOwner: true,
		policy:             ctx.EffectiveReproPolicy(),
		active:             ctx.ActiveOwners,
		reopen:             ctx.ReopenIDs,
		warnings:           &warnings,
	})
	if err != nil {
		return nil, err
	}
	report.Failures = failures

	if raw, ok := root["warnings"]; ok {
		warnings, err := decodeEvidence(raw, "warnings")
		if err != nil {
			return nil, err
		}
		report.Warnings = warnings
	} else {
		report.Warnings = []string{}
	}

	if raw, ok := root["artifacts_dir"]; ok && string(raw) != "null" {
		var dir string
		if json.Unmarshal(raw, &dir) != nil {
			return nil, diag(CodeInvalidEnum, "artifacts_dir", truncateValue(string(raw)), "artifacts_dir must be a string")
		}
		report.ArtifactsDir = dir
	}

	if err := requireEmptyFailuresOnPass(status, "failures", len(failures)); err != nil {
		return nil, err
	}
	if err := ValidateFailedClose(SourceBrowserUI, status, false, failures, ctx); err != nil {
		return nil, err
	}

	report.ContractWarnings = warnings
	return report, nil
}
