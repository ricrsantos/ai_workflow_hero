package claude

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestRunClaudeStreamJSONImageSpike_RecordsFailedFixtureAndBuildsCandidateWireInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.png")
	if err := os.WriteFile(path, tinyClaudePNG(t), 0o600); err != nil {
		t.Fatal(err)
	}
	attachment := harness.Attachment{Kind: harness.MediaKindImage, MIMEType: "image/png", Path: path}

	var gotInvocation Invocation
	var gotInput []byte
	runner := func(_ context.Context, invocation Invocation, input []byte) (ProbeResult, error) {
		gotInvocation = invocation.Clone()
		gotInput = append([]byte(nil), input...)
		// A normal text-capable stream does not prove that Claude accepted the
		// image block, so the fixture must select the degraded path.
		return ProbeResult{Stdout: `{"type":"system","subtype":"init","session_id":"fixture"}
{"type":"result","subtype":"success","session_id":"fixture","result":"text only"}`}, nil
	}

	result, err := RunClaudeStreamJSONImageSpike(context.Background(), runner, dir, "sonnet-fixture", attachment)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || result.Mode != ClaudeImageTransportDegradedFileRef {
		t.Fatalf("spike result = %+v, want failed degraded result", result)
	}
	if !strings.Contains(result.Diagnostic, "degraded file-reference") {
		t.Fatalf("spike diagnostic = %q", result.Diagnostic)
	}
	t.Logf("Claude stream-json image spike result: FAIL (%s)", result.Diagnostic)

	if !containsArgPair(gotInvocation.Args, "--input-format", "stream-json") {
		t.Fatalf("candidate invocation does not select stream-json input: %#v", gotInvocation.Args)
	}
	var frame struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type   string `json:"type"`
				Source struct {
					Type      string `json:"type"`
					MediaType string `json:"media_type"`
					Data      string `json:"data"`
				} `json:"source"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(gotInput), &frame); err != nil {
		t.Fatalf("decode candidate stdin: %v", err)
	}
	if frame.Type != "user" || len(frame.Message.Content) != 2 {
		t.Fatalf("candidate frame = %+v", frame)
	}
	if frame.Message.Content[0].Type != "image" || frame.Message.Content[0].Source.Type != "base64" || frame.Message.Content[0].Source.MediaType != "image/png" {
		t.Fatalf("candidate image block = %+v", frame.Message.Content[0])
	}
	decoded, err := base64.StdEncoding.DecodeString(frame.Message.Content[0].Source.Data)
	if err != nil || !bytes.Equal(decoded, tinyClaudePNG(t)) {
		t.Fatalf("candidate image data was not preserved")
	}
	if frame.Message.Content[1].Type != "text" {
		t.Fatalf("candidate content order = %+v", frame.Message.Content)
	}
}

func TestRunClaudeStreamJSONImageSpike_PassesOnlyExplicitImageAcknowledgement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.png")
	if err := os.WriteFile(path, tinyClaudePNG(t), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := func(_ context.Context, _ Invocation, _ []byte) (ProbeResult, error) {
		return ProbeResult{Stdout: `{"type":"system","subtype":"image_input_ack","accepted":true}`}, nil
	}

	result, err := RunClaudeStreamJSONImageSpike(context.Background(), runner, dir, "sonnet-fixture", harness.Attachment{MIMEType: "image/png", Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed || result.Mode != ClaudeImageTransportNative {
		t.Fatalf("spike result = %+v, want native pass", result)
	}
	t.Logf("Claude stream-json image spike result: PASS (%s)", result.Diagnostic)
}

func TestBuildClaudeAttachmentPlan_LabelsDegradedFileReferenceFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.png")
	if err := os.WriteFile(path, tinyClaudePNG(t), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildClaudeAttachmentPlan("describe it", []harness.Attachment{{Kind: harness.MediaKindImage, MIMEType: "image/png", Path: path}}, ClaudeAttachmentOptions{
		Model: "sonnet-fixture",
		Capability: harness.MediaCapability{
			ImageInputFileReference: true,
			SupportedImageMIMETypes: []string{"image/png"},
		},
		Spike: ClaudeImageSpikeResult{Mode: ClaudeImageTransportDegradedFileRef, Diagnostic: "fixture failed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != ClaudeImageTransportDegradedFileRef || len(plan.Stdin) != 0 {
		t.Fatalf("plan = %+v, want degraded prompt-only plan", plan)
	}
	for _, want := range []string{"DEGRADED", "file-reference", path, "describe it"} {
		if !strings.Contains(plan.Prompt, want) {
			t.Fatalf("degraded prompt does not contain %q: %s", want, plan.Prompt)
		}
	}
	if !strings.Contains(plan.Diagnostic, "degraded") {
		t.Fatalf("plan diagnostic = %q", plan.Diagnostic)
	}
}

func TestBuildClaudeAttachmentPlan_FailsWhenNeitherNativeNorDegradedPathWorks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.png")
	if err := os.WriteFile(path, tinyClaudePNG(t), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := BuildClaudeAttachmentPlan("describe it", []harness.Attachment{{MIMEType: "image/png", Path: path}}, ClaudeAttachmentOptions{
		Model:      "text-only-model",
		Capability: harness.MediaCapability{},
	})
	if err == nil {
		t.Fatal("expected explicit attachment failure")
	}
	for _, want := range []string{"Claude", "text-only-model", "native", "degraded"} {
		if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(want)) {
			t.Fatalf("error %q does not identify %q", err, want)
		}
	}
}

func TestClaudeWorkspaceImageWatch_CombinesToolResultPathsAndFSChangesByHash(t *testing.T) {
	png := tinyClaudePNG(t)
	filesystem := fstest.MapFS{
		"before.png": &fstest.MapFile{Data: png},
	}
	watch, err := NewClaudeWorkspaceImageWatchFS("/workspace", filesystem)
	if err != nil {
		t.Fatal(err)
	}
	filesystem["outputs/from-tool.png"] = &fstest.MapFile{Data: png}
	filesystem["outputs/same.png"] = &fstest.MapFile{Data: png}
	stream := []byte(`{"type":"user","message":{"content":[{"type":"tool_result","content":[{"type":"text","text":"wrote /workspace/outputs/from-tool.png and /workspace/outputs/same.png"}]}]}}`)

	assets, err := watch.FinishTurn("claude-session", "turn-9", stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("assets = %#v, want one deduplicated tool asset", assets)
	}
	asset := assets[0]
	if asset.Source != harness.AssetSourceTool || asset.Path != "/workspace/outputs/from-tool.png" || asset.ContentHash == "" {
		t.Fatalf("asset = %+v", asset)
	}
	if asset.SessionID != "claude-session" || asset.TurnID != "turn-9" {
		t.Fatalf("asset provenance = %+v", asset)
	}
	if asset.Width != 1 || asset.Height != 1 {
		t.Fatalf("asset dimensions = %dx%d", asset.Width, asset.Height)
	}
}

func TestExtractClaudeToolResultImagePaths_IgnoresAssistantImageMention(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"/workspace/not-tool.png"}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"saved file:///workspace/result.webp"}]}}`,
	}, "\n")
	paths := ExtractClaudeToolResultImagePaths([]byte(stream))
	if len(paths) != 1 || paths[0] != "/workspace/result.webp" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestBuildClaudeAttachmentPlan_UsesNativeOnlyAfterPassedSpike(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.png")
	if err := os.WriteFile(path, tinyClaudePNG(t), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildClaudeAttachmentPlan("native", []harness.Attachment{{MIMEType: "image/png", Path: path}}, ClaudeAttachmentOptions{
		WorkingDir: dir,
		Model:      "vision-model",
		Capability: harness.MediaCapability{ImageInputNative: true},
		Spike:      ClaudeImageSpikeResult{Passed: true, Mode: ClaudeImageTransportNative},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != ClaudeImageTransportNative || len(plan.Stdin) == 0 || !containsArgPair(plan.Invocation.Args, "--input-format", "stream-json") {
		t.Fatalf("native plan = %+v", plan)
	}
}

func containsArgPair(args []string, key, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}

func tinyClaudePNG(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
