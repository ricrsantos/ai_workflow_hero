package reports

// QAEndToEndReport is the decoded QA End-to-End stage-agent payload.
type QAEndToEndReport struct {
	Status         string
	UsePlaywright  *bool
	TestsPassed    *bool
	FlowsValidated []string
	Failures       []FailureEntry
	Summary        string
}

var e2eAllowed = map[string]struct{}{
	"status": {}, "use_playwright": {}, "tests_passed": {}, "flows_validated": {},
	"failures": {}, "summary": {},
}

// DecodeQAEndToEnd validates and decodes a QA End-to-End JSON report.
func DecodeQAEndToEnd(data []byte, ctx DecodeContext) (*QAEndToEndReport, *DiagnosticError) {
	root, err := parseObject(data)
	if err != nil {
		return nil, err
	}
	if err := unknownFields(root, e2eAllowed, ""); err != nil {
		return nil, err
	}

	status, err := parseValidationStatus(root)
	if err != nil {
		return nil, err
	}
	summary, err := requireString(root, "summary")
	if err != nil {
		return nil, err
	}

	report := &QAEndToEndReport{Status: status, Summary: summary}
	report.UsePlaywright, err = optionalBoolPtr(root, "use_playwright")
	if err != nil {
		return nil, err
	}
	report.TestsPassed, err = optionalBoolPtr(root, "tests_passed")
	if err != nil {
		return nil, err
	}
	if _, ok := root["flows_validated"]; ok {
		flows, err := requireStringArray(root, "flows_validated")
		if err != nil {
			return nil, err
		}
		report.FlowsValidated = flows
	} else {
		report.FlowsValidated = []string{}
	}

	rawFailures, ok := root["failures"]
	if !ok {
		return nil, diag(CodeMissingField, "failures", "", "field is required")
	}
	failures, err := decodeFailureEntries(rawFailures, "failures", failureEntryOptions{
		sourceStage:  SourceQAEndToEnd,
		requireOwner: true,
		active:       ctx.ActiveOwners,
		reopen:       ctx.ReopenIDs,
	})
	if err != nil {
		return nil, err
	}
	report.Failures = failures

	if err := requireEmptyFailuresOnPass(status, "failures", len(failures)); err != nil {
		return nil, err
	}
	if err := ValidateFailedClose(SourceQAEndToEnd, status, false, failures, ctx); err != nil {
		return nil, err
	}

	return report, nil
}
