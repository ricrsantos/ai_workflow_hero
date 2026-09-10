package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestResultAssemblerGoldenStreamNormalizesPermissionQuestionAndWarnings(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "stream.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	events, err := DecodeNDJSON(file)
	if err != nil {
		t.Fatal(err)
	}
	assembler := newResultAssembler(false)
	var deltas []harness.StreamDelta
	for _, event := range events {
		// The fixture also freezes an independent terminal-error protocol frame.
		// It is not part of the successful golden execution being assembled here.
		if event.Type == "result" && event.Subtype == "error" {
			continue
		}
		if event.Type == "mystery" {
			continue
		}
		got, err := assembler.consume(event)
		if err != nil {
			t.Fatalf("consume line %d: %v", event.Line, err)
		}
		deltas = append(deltas, got...)
	}
	result, err := assembler.result(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output != "Hello world" || result.SessionID != "session-001" || result.Usage.InputTokens != 12 || result.Usage.OutputTokens != 8 {
		t.Fatalf("result=%+v", result)
	}
	assertGoldenDelta(t, deltas, harness.StreamKindText, "", "Hello")
	assertGoldenDelta(t, deltas, harness.StreamKindThinking, "", "I should inspect the project.")
	assertGoldenDelta(t, deltas, harness.StreamKindTool, "", "Claude tool Read")
	assertGoldenHarnessDelta(t, deltas, harness.StreamKindTool, "system.subagent_started")
	assertGoldenHarnessDelta(t, deltas, harness.StreamKindTool, "system.subagent_completed")
	assertGoldenHarnessDelta(t, deltas, harness.StreamKindActivity, "system.retry")
	assertGoldenHarnessDelta(t, deltas, harness.StreamKindActivity, "system.hook_started")
	assertGoldenHarnessDelta(t, deltas, harness.StreamKindActivity, "system.plugin_loaded")
	assertGoldenHarnessDelta(t, deltas, harness.StreamKindActivity, "system.usage")
	assertGoldenDelta(t, deltas, harness.StreamKindPermission, "permission-001", "Claude requests permission for Bash")
	assertGoldenDelta(t, deltas, harness.StreamKindQuestion, "question-001", "Continue with the migration?")
}

func TestResultAssemblerDebugOnlyEventsStayHiddenWithoutDebug(t *testing.T) {
	assembler := newResultAssembler(false)
	events := debugOnlyEvents()
	events = append(events, RawEvent{Line: 99, Type: "mystery", Raw: []byte(`{"type":"mystery","payload":{"sensitive":"redacted-by-normalizer"}}`)})
	var deltas []harness.StreamDelta
	for _, event := range events {
		got, err := assembler.consume(event)
		if err != nil {
			t.Fatalf("consume line %d: %v", event.Line, err)
		}
		deltas = append(deltas, got...)
	}
	if len(deltas) != 0 {
		t.Fatalf("expected no deltas without debug, got %+v", deltas)
	}
}

func TestResultAssemblerDebugOnlyEventsSurfaceWithDebug(t *testing.T) {
	assembler := newResultAssembler(true)
	var deltas []harness.StreamDelta
	for _, event := range debugOnlyEvents() {
		got, err := assembler.consume(event)
		if err != nil {
			t.Fatalf("consume line %d: %v", event.Line, err)
		}
		deltas = append(deltas, got...)
	}
	for _, harnessType := range []string{
		"system.status",
		"stream_event.text_delta",
		"system.compact_boundary",
		"system.plugin_install",
		"system.thinking_tokens",
		"system.session_state_changed",
		"system.files_persisted",
		"system.commands_changed",
		"system.background_tasks_changed",
		"prompt_suggestion",
		"control_response",
		"control_cancel_request",
		"keep_alive",
		"notification",
		"memory_recall",
		"auth_status",
		"rate_limit_event",
		"control_request",
	} {
		assertGoldenHarnessDelta(t, deltas, harness.StreamKindActivity, harnessType)
	}
}

func TestResultAssemblerMapsOfficialEventsWithoutDebug(t *testing.T) {
	assembler := newResultAssembler(false)
	tests := []struct {
		name        string
		raw         string
		kind        harness.StreamKind
		harnessType string
		text        string
	}{
		{name: "tool_progress", raw: `{"type":"tool_progress","tool_name":"Bash","tool_use_id":"tool-1"}`, kind: harness.StreamKindTool, harnessType: "tool_progress", text: "Bash"},
		{name: "tool_use_summary", raw: `{"type":"tool_use_summary","summary":"used Read"}`, kind: harness.StreamKindActivity, harnessType: "tool_use_summary", text: "tool summary"},
		{name: "api_retry", raw: `{"type":"system","subtype":"api_retry","attempt":2}`, kind: harness.StreamKindActivity, harnessType: "system.api_retry", text: "retry"},
		{name: "hook_response", raw: `{"type":"system","subtype":"hook_response","hook_name":"PreToolUse"}`, kind: harness.StreamKindActivity, harnessType: "system.hook_response", text: "PreToolUse"},
		{name: "hook_progress", raw: `{"type":"system","subtype":"hook_progress","hook_name":"PreToolUse"}`, kind: harness.StreamKindActivity, harnessType: "system.hook_progress", text: "PreToolUse"},
		{name: "task_started", raw: `{"type":"system","subtype":"task_started","task_id":"t1","description":"Explore"}`, kind: harness.StreamKindTool, harnessType: "system.task_started", text: "Explore"},
		{name: "task_updated", raw: `{"type":"system","subtype":"task_updated","task_id":"t1","patch":{"status":"running"}}`, kind: harness.StreamKindTool, harnessType: "system.task_updated", text: "running"},
		{name: "task_notification", raw: `{"type":"system","subtype":"task_notification","task_id":"t1","status":"completed"}`, kind: harness.StreamKindTool, harnessType: "system.task_notification", text: "completed"},
		{name: "task_progress", raw: `{"type":"system","subtype":"task_progress","description":"Explore"}`, kind: harness.StreamKindActivity, harnessType: "system.task_progress", text: "Explore"},
		{name: "local_command_output", raw: `{"type":"system","subtype":"local_command_output","content":"cost $0.01"}`, kind: harness.StreamKindText, harnessType: "system.local_command_output", text: "cost $0.01"},
		{name: "informational", raw: `{"type":"system","subtype":"informational","level":"info","content":"hook note"}`, kind: harness.StreamKindActivity, harnessType: "system.informational", text: "hook note"},
		{name: "informational_warning", raw: `{"type":"system","subtype":"informational","level":"warning","content":"blocked"}`, kind: harness.StreamKindWarning, harnessType: "system.informational", text: "blocked"},
		{name: "permission_denied", raw: `{"type":"system","subtype":"permission_denied","tool_name":"Bash"}`, kind: harness.StreamKindWarning, harnessType: "system.permission_denied", text: "Bash"},
		{name: "elicitation_complete", raw: `{"type":"system","subtype":"elicitation_complete"}`, kind: harness.StreamKindActivity, harnessType: "system.elicitation_complete", text: "elicitation"},
		{name: "worker_shutting_down", raw: `{"type":"system","subtype":"worker_shutting_down","reason":"host_exit"}`, kind: harness.StreamKindWarning, harnessType: "system.worker_shutting_down", text: "host_exit"},
		{name: "conversation_reset", raw: `{"type":"conversation_reset"}`, kind: harness.StreamKindActivity, harnessType: "conversation_reset", text: "reset"},
		{name: "mirror_error", raw: `{"type":"mirror_error","error":"sync failed"}`, kind: harness.StreamKindWarning, harnessType: "mirror_error", text: "mirror"},
		{name: "auth_error", raw: `{"type":"auth_status","error":"login required"}`, kind: harness.StreamKindWarning, harnessType: "auth_status", text: "authentication failed"},
		{name: "rate_limit_rejected", raw: `{"type":"rate_limit_event","rate_limit_info":{"status":"rejected"}}`, kind: harness.StreamKindWarning, harnessType: "rate_limit_event", text: "rejected"},
		{name: "can_use_tool", raw: `{"type":"control_request","request_id":"req-1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"secret"}}}`, kind: harness.StreamKindWarning, harnessType: "control_request.can_use_tool", text: "Bash"},
		{name: "elicitation", raw: `{"type":"control_request","request":{"subtype":"elicitation"}}`, kind: harness.StreamKindWarning, harnessType: "control_request.elicitation", text: "elicitation"},
		{name: "image_block", raw: `{"type":"assistant","message":{"content":[{"type":"image","source":{"data":"secret"}}]}}`, kind: harness.StreamKindText, harnessType: "assistant.image", text: "Claude image"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := decodeRawEvent(1, []byte(tt.raw))
			if err != nil {
				t.Fatal(err)
			}
			deltas, err := assembler.consume(event)
			if err != nil {
				t.Fatalf("consume: %v", err)
			}
			assertGoldenHarnessDelta(t, deltas, tt.kind, tt.harnessType)
			assertGoldenDelta(t, deltas, tt.kind, "", tt.text)
			for _, delta := range deltas {
				if strings.Contains(delta.Text, "secret") {
					t.Fatalf("payload leaked: %q", delta.Text)
				}
				if delta.Kind == harness.StreamKindPermission {
					t.Fatalf("control protocol must not emit a permission gate: %+v", delta)
				}
			}
		})
	}
}

func TestResultAssemblerMapsOfficialResultErrors(t *testing.T) {
	assembler := newResultAssembler(false)
	tests := []struct {
		raw  string
		want string
	}{
		{raw: `{"type":"result","subtype":"error_max_turns","session_id":"s1","errors":["too many turns"]}`, want: "too many turns"},
		{raw: `{"type":"result","subtype":"error_during_execution","session_id":"s1"}`, want: "execution error"},
		{raw: `{"type":"result","subtype":"error_max_budget_usd","session_id":"s1"}`, want: "maximum budget"},
		{raw: `{"type":"result","subtype":"error_max_structured_output_retries","session_id":"s1"}`, want: "structured output"},
	}
	for _, tt := range tests {
		event, err := decodeRawEvent(1, []byte(tt.raw))
		if err != nil {
			t.Fatal(err)
		}
		_, err = assembler.consume(event)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("raw=%s err=%v want %q", tt.raw, err, tt.want)
		}
		if strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("official result error treated as unsupported: %v", err)
		}
	}
}

func debugOnlyEvents() []RawEvent {
	return []RawEvent{
		{Line: 1, Type: "system", Subtype: "status", Raw: []byte(`{"type":"system","subtype":"status","session_id":"session-001","status":"compacting"}`)},
		{Line: 2, Type: "stream_event", Raw: []byte(`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"tok"}}}`)},
		{Line: 3, Type: "system", Subtype: "compact_boundary", Raw: []byte(`{"type":"system","subtype":"compact_boundary","compact_metadata":{"trigger":"auto","pre_tokens":12}}`)},
		{Line: 4, Type: "system", Subtype: "plugin_install", Raw: []byte(`{"type":"system","subtype":"plugin_install","status":"started"}`)},
		{Line: 5, Type: "system", Subtype: "thinking_tokens", Raw: []byte(`{"type":"system","subtype":"thinking_tokens","estimated_tokens":8}`)},
		{Line: 6, Type: "system", Subtype: "session_state_changed", Raw: []byte(`{"type":"system","subtype":"session_state_changed"}`)},
		{Line: 7, Type: "system", Subtype: "files_persisted", Raw: []byte(`{"type":"system","subtype":"files_persisted","files":[{"filename":"secret.go"}]}`)},
		{Line: 8, Type: "system", Subtype: "commands_changed", Raw: []byte(`{"type":"system","subtype":"commands_changed"}`)},
		{Line: 9, Type: "system", Subtype: "background_tasks_changed", Raw: []byte(`{"type":"system","subtype":"background_tasks_changed"}`)},
		{Line: 10, Type: "prompt_suggestion", Raw: []byte(`{"type":"prompt_suggestion","suggestion":"next"}`)},
		{Line: 11, Type: "control_response", Raw: []byte(`{"type":"control_response"}`)},
		{Line: 12, Type: "control_cancel_request", Raw: []byte(`{"type":"control_cancel_request"}`)},
		{Line: 13, Type: "keep_alive", Raw: []byte(`{"type":"keep_alive"}`)},
		{Line: 14, Type: "notification", Raw: []byte(`{"type":"notification"}`)},
		{Line: 15, Type: "memory_recall", Raw: []byte(`{"type":"memory_recall"}`)},
		{Line: 16, Type: "auth_status", Raw: []byte(`{"type":"auth_status","isAuthenticating":true}`)},
		{Line: 17, Type: "rate_limit_event", Raw: []byte(`{"type":"rate_limit_event","rate_limit_info":{"status":"allowed"}}`)},
		{Line: 18, Type: "control_request", Raw: []byte(`{"type":"control_request","request":{"subtype":"initialize"}}`)},
	}
}

func TestResultAssemblerBoundsRedactedUnknownWarnings(t *testing.T) {
	assembler := newResultAssembler(true)
	var warnings []harness.StreamDelta
	for line := 1; line <= maxUnknownEventWarnings+3; line++ {
		event, err := decodeRawEvent(line, []byte(`{"type":"future_event","secret":"must not render"}`))
		if err != nil {
			t.Fatal(err)
		}
		deltas, err := assembler.consume(event)
		if err != nil {
			t.Fatal(err)
		}
		warnings = append(warnings, deltas...)
	}
	if len(warnings) != maxUnknownEventWarnings+1 {
		t.Fatalf("warnings=%d want %d: %+v", len(warnings), maxUnknownEventWarnings+1, warnings)
	}
	if warnings[len(warnings)-1].HarnessType != "claude.unknown.suppressed" || !strings.Contains(warnings[len(warnings)-1].Text, "suppressed") {
		t.Fatalf("suppression warning=%+v", warnings[len(warnings)-1])
	}
	for _, warning := range warnings {
		if strings.Contains(warning.Text, "must not render") {
			t.Fatalf("unknown warning leaked raw payload: %q", warning.Text)
		}
	}
}

func TestResultAssemblerRejectsMalformedFrameBeforeForwarding(t *testing.T) {
	if _, err := decodeRawEvent(7, []byte(`{"type":`)); err == nil || !strings.Contains(err.Error(), "line 7") {
		t.Fatalf("err=%v", err)
	}
}

func assertGoldenDelta(t *testing.T, deltas []harness.StreamDelta, kind harness.StreamKind, requestID, text string) {
	t.Helper()
	for _, delta := range deltas {
		if delta.Kind != kind || !strings.Contains(delta.Text, text) {
			continue
		}
		if requestID == "" || delta.Metadata["request_id"] == requestID {
			return
		}
	}
	t.Fatalf("missing %s delta request=%q text=%q in %+v", kind, requestID, text, deltas)
}

func assertGoldenHarnessDelta(t *testing.T, deltas []harness.StreamDelta, kind harness.StreamKind, harnessType string) {
	t.Helper()
	for _, delta := range deltas {
		if delta.Kind == kind && delta.HarnessType == harnessType {
			return
		}
	}
	t.Fatalf("missing %s delta with harness type %q in %+v", kind, harnessType, deltas)
}
