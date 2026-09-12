package reports

import "encoding/json"

// ReportJSONFromText returns the canonical JSON bytes for the first status object in agent output.
func ReportJSONFromText(raw string) ([]byte, *DiagnosticError) {
	root, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}
	b, marshalErr := json.Marshal(root)
	if marshalErr != nil {
		return nil, diag(CodeInvalidJSON, "", "", "could not serialize report object")
	}
	return b, nil
}
