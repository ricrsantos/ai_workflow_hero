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
	events := []RawEvent{
		{Line: 1, Type: "system", Subtype: "status", Raw: []byte(`{"type":"system","subtype":"status","session_id":"session-001","status":"compacting"}`)},
		{Line: 2, Type: "stream_event", Raw: []byte(`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"tok"}}}`)},
		{Line: 3, Type: "mystery", Raw: []byte(`{"type":"mystery","payload":{"sensitive":"redacted-by-normalizer"}}`)},
	}
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
	events := []RawEvent{
		{Line: 1, Type: "system", Subtype: "status", Raw: []byte(`{"type":"system","subtype":"status","session_id":"session-001","status":"compacting"}`)},
		{Line: 2, Type: "stream_event", Raw: []byte(`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"tok"}}}`)},
	}
	var deltas []harness.StreamDelta
	for _, event := range events {
		got, err := assembler.consume(event)
		if err != nil {
			t.Fatalf("consume line %d: %v", event.Line, err)
		}
		deltas = append(deltas, got...)
	}
	assertGoldenHarnessDelta(t, deltas, harness.StreamKindActivity, "system.status")
	assertGoldenHarnessDelta(t, deltas, harness.StreamKindActivity, "stream_event.text_delta")
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
