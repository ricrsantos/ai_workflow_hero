package reports

import (
	"encoding/json"
	"sort"
	"strings"
)

type object map[string]json.RawMessage

func parseObject(data []byte) (object, *DiagnosticError) {
	data = trimLeadingJSON(data)
	if len(data) == 0 {
		return nil, diag(CodeInvalidJSON, "", "", "report body is empty")
	}
	var root object
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, diag(CodeInvalidJSON, "", truncateValue(string(data)), "body must be a single JSON object")
	}
	if root == nil {
		return nil, diag(CodeInvalidJSON, "", "", "body must be a single JSON object")
	}
	return root, nil
}

func trimLeadingJSON(data []byte) []byte {
	return []byte(strings.TrimSpace(string(data)))
}

// dropUnknownFields keeps the cycle moving when a report carries more than the
// contract asks for. A field the contract does not know cannot change what Hero
// persists, so rejecting the whole report over one throws away real validation
// work — the failure mode that stalled QA when an agent attached `metrics`.
//
// Missing and malformed fields stay fail-closed: fewer fields are ambiguous,
// extra fields are merely noise.
//
// Before dropping a key it is matched against the contract case- and
// separator-insensitively. Silently discarding `reopenId` would let Hero
// allocate a fresh finding ID instead of reopening, which quietly bypasses the
// loop ceiling — so a near-miss is adopted rather than lost.
func dropUnknownFields(root object, allowed map[string]struct{}, prefix string) []ReportWarning {
	var warnings []ReportWarning
	for _, key := range sortedKeys(root) {
		if _, ok := allowed[key]; ok {
			continue
		}
		field := prefix + key
		if canonical, ok := matchContractField(key, allowed); ok {
			if _, taken := root[canonical]; !taken {
				root[canonical] = root[key]
				delete(root, key)
				warnings = append(warnings, warn(CodeFieldRenamed, field, canonical,
					"field name normalized onto the report contract"))
				continue
			}
		}
		delete(root, key)
		warnings = append(warnings, warn(CodeUnknownField, field, "",
			"field is not part of the report contract and was ignored"))
	}
	return warnings
}

// matchContractField resolves a key to its contract spelling, ignoring case and
// `_`/`-`/space separators (reopenId → reopen_id, Acceptance-Criteria →
// acceptance_criteria).
func matchContractField(key string, allowed map[string]struct{}) (string, bool) {
	target := normalizeFieldKey(key)
	if target == "" {
		return "", false
	}
	for _, canonical := range sortedSet(allowed) {
		if normalizeFieldKey(canonical) == target {
			return canonical, true
		}
	}
	return "", false
}

func normalizeFieldKey(key string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(key)) {
		switch r {
		case '_', '-', ' ':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sortedKeys(root object) []string {
	out := make([]string, 0, len(root))
	for k := range root {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func requireString(root object, name string) (string, *DiagnosticError) {
	raw, ok := root[name]
	if !ok {
		return "", diag(CodeMissingField, name, "", "field is required")
	}
	if string(raw) == "null" {
		return "", diag(CodeMissingField, name, "null", "field is required")
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", diag(CodeInvalidEnum, name, truncateValue(string(raw)), "field must be a string")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", diag(CodeMissingField, name, "", "field must be non-empty")
	}
	return value, nil
}

func optionalStringPtr(root object, name string) (*string, *DiagnosticError) {
	raw, ok := root[name]
	if !ok {
		return nil, nil
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, diag(CodeInvalidEnum, name, truncateValue(string(raw)), "field must be a string or null")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	return &value, nil
}

func requireBool(root object, name string) (bool, *DiagnosticError) {
	raw, ok := root[name]
	if !ok {
		return false, diag(CodeMissingField, name, "", "field is required")
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, diag(CodeInvalidEnum, name, truncateValue(string(raw)), "field must be a boolean")
	}
	return value, nil
}

func optionalBoolPtr(root object, name string) (*bool, *DiagnosticError) {
	raw, ok := root[name]
	if !ok {
		return nil, nil
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, diag(CodeInvalidEnum, name, truncateValue(string(raw)), "field must be a boolean or null")
	}
	return &value, nil
}

func requireStringArray(root object, name string) ([]string, *DiagnosticError) {
	raw, ok := root[name]
	if !ok {
		return nil, diag(CodeMissingField, name, "", "field is required")
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, diag(CodeInvalidEnum, name, truncateValue(string(raw)), "field must be a string array")
	}
	if values == nil {
		values = []string{}
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.TrimSpace(v)
	}
	return out, nil
}

func requireEmptyJSONArray(raw json.RawMessage, field string) *DiagnosticError {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return diag(CodeInvalidEnum, field, truncateValue(string(raw)), "field must be an array")
	}
	if len(items) > 0 {
		return diag(CodeInvalidEnum, field, "", "must be empty when sdd_ambiguity is true")
	}
	return nil
}

func truncateValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 120 {
		return v[:120] + "…"
	}
	return v
}
