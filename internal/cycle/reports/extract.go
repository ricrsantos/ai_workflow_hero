package reports

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
)

var jsonFenceRE = regexp.MustCompile("(?is)```json\\s*\\n(.*?)```")

const maxJSONObjectRepairs = 16

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

func extractJSONObject(raw string) (object, *DiagnosticError) {
	var firstErr *DiagnosticError
	for _, candidate := range jsonExtractCandidates(raw) {
		obj, err := extractStatusObject(candidate)
		if err == nil {
			return obj, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, diag(CodeInvalidJSON, "", "", "no JSON report object with status found")
}

func jsonExtractCandidates(raw string) []string {
	out := make([]string, 0, 4)
	for _, m := range jsonFenceRE.FindAllStringSubmatch(raw, -1) {
		if len(m) < 2 {
			continue
		}
		if body := strings.TrimSpace(m[1]); body != "" {
			out = append(out, body)
		}
	}
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		out = append(out, trimmed)
	}
	return out
}

func extractStatusObject(raw string) (object, *DiagnosticError) {
	var firstErr *DiagnosticError
	for i := 0; i < len(raw); i++ {
		if raw[i] != '{' {
			continue
		}
		data := []byte(raw[i:])
		obj, err := decodeOneObject(data)
		if err == nil && obj != nil {
			if _, ok := obj["status"]; ok {
				return obj, nil
			}
			continue
		}
		if !looksLikeStatusReport(data) {
			continue
		}
		repaired, repairErr := repairTruncatedJSONObjects(data)
		if repairErr != nil {
			if firstErr == nil {
				firstErr = jsonDiagnostic(data, repairErr)
			}
			continue
		}
		obj, err = decodeOneObject(repaired)
		if err != nil {
			if firstErr == nil {
				firstErr = jsonDiagnostic(repaired, err)
			}
			continue
		}
		if obj != nil {
			if _, ok := obj["status"]; ok {
				return obj, nil
			}
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, diag(CodeInvalidJSON, "", "", "no JSON report object with status found")
}

func decodeOneObject(data []byte) (object, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, io.EOF
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var root object
	if err := dec.Decode(&root); err != nil {
		return nil, err
	}
	if root == nil {
		return nil, errors.New("body must be a single JSON object")
	}
	return root, nil
}

func looksLikeStatusReport(data []byte) bool {
	head := bytes.TrimSpace(data)
	if len(head) == 0 || head[0] != '{' {
		return false
	}
	if len(head) > 256 {
		head = head[:256]
	}
	return bytes.Contains(head, []byte(`"status"`))
}

func repairTruncatedJSONObjects(data []byte) ([]byte, error) {
	cur := bytes.TrimSpace(data)
	var last error
	for i := 0; i < maxJSONObjectRepairs; i++ {
		if _, err := decodeOneObject(cur); err == nil {
			return cur, nil
		} else {
			last = err
		}
		var se *json.SyntaxError
		if !errors.As(last, &se) {
			return nil, last
		}
		pos := int(se.Offset)
		if pos <= 0 {
			pos = len(cur)
		} else if pos > len(cur) {
			pos = len(cur)
		}
		msg := se.Error()
		switch {
		case strings.Contains(msg, "invalid character ']' after object key:value pair"):
			insertAt := pos - 1
			if insertAt < 0 {
				insertAt = 0
			}
			cur = append(append([]byte{}, cur[:insertAt]...), append([]byte{'}'}, cur[insertAt:]...)...)
		case strings.Contains(msg, "unexpected end of JSON input"):
			cur = append(cur, '}')
		default:
			return nil, last
		}
	}
	if last == nil {
		last = errors.New("json object repair limit exceeded")
	}
	return nil, last
}

func jsonDiagnostic(data []byte, err error) *DiagnosticError {
	if err == nil {
		return diag(CodeInvalidJSON, "", "", "body must be a single JSON object")
	}
	return diag(CodeInvalidJSON, "", truncateValue(string(data)), err.Error())
}
