package reports

import (
	"encoding/json"
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

func extractJSONObject(raw string) (object, *DiagnosticError) {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '{' {
			continue
		}
		obj, err := parseObject([]byte(raw[i:]))
		if err == nil && obj != nil {
			if _, ok := obj["status"]; ok {
				return obj, nil
			}
		}
	}
	return nil, diag(CodeInvalidJSON, "", "", "no JSON report object with status found")
}

func trimLeadingJSON(data []byte) []byte {
	return []byte(strings.TrimSpace(string(data)))
}

func unknownFields(root object, allowed map[string]struct{}, prefix string) *DiagnosticError {
	for key := range root {
		if _, ok := allowed[key]; !ok {
			field := key
			if prefix != "" {
				field = prefix + key
			}
			return diag(CodeUnknownField, field, "", "field is not part of the report contract")
		}
	}
	return nil
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
