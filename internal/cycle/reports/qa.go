package reports

import (
	"encoding/json"
)

// QAReport is the decoded QA stage-agent payload.
type QAReport struct {
	Status   string
	Failures []FailureEntry
	Summary  string
	// ContractWarnings records deviations Hero tolerated (extra fields it
	// ignored, key names it normalized). Never a reason to reject the report.
	ContractWarnings []ReportWarning
}

var qaAllowed = map[string]struct{}{
	"status": {}, "failures": {}, "summary": {},
}

// DecodeQA validates and decodes a QA JSON report.
func DecodeQA(data []byte, ctx DecodeContext) (*QAReport, *DiagnosticError) {
	root, err := parseObject(data)
	if err != nil {
		return nil, err
	}
	var warnings []ReportWarning
	collect(&warnings, dropUnknownFields(root, qaAllowed, ""))

	status, err := parseValidationStatus(root)
	if err != nil {
		return nil, err
	}
	summary, err := requireString(root, "summary")
	if err != nil {
		return nil, err
	}

	rawFailures, ok := root["failures"]
	if !ok {
		return nil, diag(CodeMissingField, "failures", "", "field is required")
	}
	failures, err := decodeFailureEntries(rawFailures, "failures", failureEntryOptions{
		sourceStage:  SourceQA,
		requireOwner: true,
		policy:       ctx.EffectiveReproPolicy(),
		active:       ctx.ActiveOwners,
		reopen:       ctx.ReopenIDs,
		warnings:     &warnings,
	})
	if err != nil {
		return nil, err
	}
	if err := requireEmptyFailuresOnPass(status, "failures", len(failures)); err != nil {
		return nil, err
	}
	if err := ValidateFailedClose(SourceQA, status, false, failures, ctx); err != nil {
		return nil, err
	}

	return &QAReport{Status: status, Failures: failures, Summary: summary, ContractWarnings: warnings}, nil
}

// DecodeQAFromText extracts the first JSON object containing status from agent output.
func DecodeQAFromText(raw string, ctx DecodeContext) (*QAReport, *DiagnosticError) {
	root, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}
	return DecodeQA(mustMarshal(root), ctx)
}

func mustMarshal(root object) []byte {
	b, _ := json.Marshal(root)
	return b
}
