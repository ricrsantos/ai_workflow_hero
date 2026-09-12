package reports

// JudgeReport is the decoded Judge stage-agent payload.
type JudgeReport struct {
	Status             string
	ImplementationGaps []FailureEntry
	SDDAmbiguity       bool
	Summary            string
}

var judgeAllowed = map[string]struct{}{
	"status": {}, "implementation_gaps": {}, "sdd_ambiguity": {}, "summary": {},
}

// DecodeJudge validates and decodes a Judge JSON report.
func DecodeJudge(data []byte, ctx DecodeContext) (*JudgeReport, *DiagnosticError) {
	root, err := parseObject(data)
	if err != nil {
		return nil, err
	}
	if err := unknownFields(root, judgeAllowed, ""); err != nil {
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
	sddAmbiguity, err := requireBool(root, "sdd_ambiguity")
	if err != nil {
		return nil, err
	}

	rawGaps, ok := root["implementation_gaps"]
	if !ok {
		return nil, diag(CodeMissingField, "implementation_gaps", "", "field is required")
	}

	var gaps []FailureEntry
	if sddAmbiguity {
		if err := requireEmptyJSONArray(rawGaps, "implementation_gaps"); err != nil {
			return nil, err
		}
		gaps = []FailureEntry{}
	} else {
		gaps, err = decodeFailureEntries(rawGaps, "implementation_gaps", failureEntryOptions{
			sourceStage:       SourceJudge,
			defaultJudgeOwner: true,
			active:            ctx.ActiveOwners,
			activeImpl:        ctx.ActiveImplementationAgents,
			reopen:            ctx.ReopenIDs,
		})
		if err != nil {
			return nil, err
		}
	}

	if err := requireEmptyFailuresOnPass(status, "implementation_gaps", len(gaps)); err != nil {
		return nil, err
	}
	if err := ValidateFailedClose(SourceJudge, status, sddAmbiguity, gaps, ctx); err != nil {
		return nil, err
	}

	return &JudgeReport{
		Status:             status,
		ImplementationGaps: gaps,
		SDDAmbiguity:       sddAmbiguity,
		Summary:            summary,
	}, nil
}

// DecodeJudgeFromText extracts the first JSON object containing status from agent output.
func DecodeJudgeFromText(raw string, ctx DecodeContext) (*JudgeReport, *DiagnosticError) {
	root, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}
	return DecodeJudge(mustMarshal(root), ctx)
}
