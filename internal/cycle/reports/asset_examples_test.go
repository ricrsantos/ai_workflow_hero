package reports

import (
	"encoding/json"
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/assets"
)

func TestEmbeddedC15ExamplesDecode(t *testing.T) {
	t.Parallel()
	harnesses := []string{"cursor", "opencode", "codex", "claude"}
	ctx := testCtx()
	ctx.ActiveImplementationAgents = []string{OwnerBackend, OwnerFrontend, OwnerGeneric}

	cases := []struct {
		agent  string
		decode func(raw []byte, obj map[string]json.RawMessage) *DiagnosticError
	}{
		{
			agent: "backend_agent",
			decode: func(raw []byte, obj map[string]json.RawMessage) *DiagnosticError {
				return decodeImplementationExample(raw, obj)
			},
		},
		{
			agent: "frontend_agent",
			decode: func(raw []byte, obj map[string]json.RawMessage) *DiagnosticError {
				return decodeImplementationExample(raw, obj)
			},
		},
		{
			agent: "generic_agent",
			decode: func(raw []byte, obj map[string]json.RawMessage) *DiagnosticError {
				return decodeImplementationExample(raw, obj)
			},
		},
		{
			agent: "qa_agent",
			decode: func(raw []byte, obj map[string]json.RawMessage) *DiagnosticError {
				_, err := DecodeQA(raw, ctx)
				return err
			},
		},
		{
			agent: "judge_agent",
			decode: func(raw []byte, obj map[string]json.RawMessage) *DiagnosticError {
				_, err := DecodeJudge(raw, ctx)
				return err
			},
		},
		{
			agent: "browser_ui_agent",
			decode: func(raw []byte, obj map[string]json.RawMessage) *DiagnosticError {
				_, err := DecodeBrowserUI(raw, ctx)
				return err
			},
		},
		{
			agent: "end2end_qa_agent",
			decode: func(raw []byte, obj map[string]json.RawMessage) *DiagnosticError {
				_, err := DecodeQAEndToEnd(raw, ctx)
				return err
			},
		},
	}

	for _, harnessID := range harnesses {
		for _, tc := range cases {
			rel := path.Join(harnessID, "agents", tc.agent+".md")
			data, err := fs.ReadFile(assets.FS, rel)
			if err != nil {
				t.Fatalf("read %s: %v", rel, err)
			}
			decoded := 0
			for i, raw := range jsonFences(string(data)) {
				obj, ok := unmarshalObject(raw)
				if !ok || isMetricsOnlyObject(obj) {
					continue
				}
				if _, hasStatus := obj["status"]; !hasStatus {
					continue
				}
				if derr := tc.decode(raw, obj); derr != nil {
					t.Errorf("%s fence %d: %v\n%s", rel, i+1, derr, raw)
					continue
				}
				decoded++
			}
			if decoded == 0 {
				t.Errorf("%s: no C15 JSON examples decoded", rel)
			}
		}
	}
}

func decodeImplementationExample(raw []byte, obj map[string]json.RawMessage) *DiagnosticError {
	agent := "generic_agent"
	if v, err := requireString(object(obj), "agent"); err == nil {
		agent = v
	}
	assignment := append(softIDs(obj, "tasks_completed"), softIDs(obj, "tasks_remaining")...)
	_, err := DecodeImplementation(raw, agent, assignment, testCtx())
	return err
}

func jsonFences(md string) [][]byte {
	matches := jsonFenceRE.FindAllStringSubmatch(md, -1)
	out := make([][]byte, 0, len(matches))
	for _, m := range matches {
		out = append(out, []byte(strings.TrimSpace(m[1])))
	}
	return out
}

func unmarshalObject(raw []byte) (map[string]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return nil, false
	}
	return obj, true
}

func isMetricsOnlyObject(obj map[string]json.RawMessage) bool {
	_, hasMetrics := obj["metrics"]
	return hasMetrics && len(obj) == 1
}

func softIDs(obj map[string]json.RawMessage, field string) []string {
	raw, ok := obj[field]
	if !ok {
		return nil
	}
	var values []string
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if id := strings.TrimSpace(v); id != "" {
			out = append(out, id)
		}
	}
	return out
}
