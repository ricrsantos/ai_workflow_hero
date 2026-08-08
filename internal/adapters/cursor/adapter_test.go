package cursor_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	cursoradapter "github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestAdapterSatisfiesHarnessInterface(t *testing.T) {
	var _ harness.HarnessAdapter = (*cursoradapter.Adapter)(nil)
}

func TestAdapterSupportsChatWithCursorDir(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := cursoradapter.NewAdapter(dir)
	if !adapter.SupportsChat() {
		t.Fatal("expected SupportsChat true when hero-start command exists")
	}
	if adapter.Name() != "cursor" {
		t.Fatalf("name=%q want cursor", adapter.Name())
	}
}

func TestDispatchFallbackWhenAgentCLIMissing(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		CycleID:    1,
		StageName:  "research",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Dispatched {
		t.Fatalf("expected fallback, got %+v", res)
	}
	if res.Message == "" {
		t.Fatal("expected fallback message")
	}
}

func TestDispatchFallbackWhenChatAssetsMissing(t *testing.T) {
	dir := t.TempDir()
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) {
		return "/usr/bin/cursor-agent", nil
	}
	adapter.VerifyAgent = func(context.Context, string) error { return nil }

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{ProjectDir: dir, StageName: "qa"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Dispatched {
		t.Fatalf("expected fallback without chat assets, got %+v", res)
	}
}

func TestDispatchCustomCommandFallbackMessage(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		Prompt:     "# Opsx propose\n\nDo the thing.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Dispatched {
		t.Fatalf("expected fallback, got %+v", res)
	}
	want := "Dispatch unavailable; run the same command in Cursor chat."
	if res.Message != want {
		t.Fatalf("message=%q want %q", res.Message, want)
	}
}

func TestDispatchCustomCommandPassesPromptToPusher(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	prompt := "# Custom\n\nExpanded markdown body."
	var gotPrompt string
	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/usr/bin/cursor-agent", nil }
	adapter.VerifyAgent = func(context.Context, string) error { return nil }
	adapter.Pusher = func(_ context.Context, _ string, req harness.DispatchRequest) (harness.DispatchResult, error) {
		gotPrompt = req.Prompt
		return harness.DispatchResult{Dispatched: true, Message: "ok"}, nil
	}

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		Prompt:     prompt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Dispatched {
		t.Fatalf("expected dispatched, got %+v", res)
	}
	if gotPrompt != prompt {
		t.Fatalf("pusher prompt=%q want %q", gotPrompt, prompt)
	}
}

func TestDispatchSuccessWhenPusherAvailable(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".cursor", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "hero-start.md"), []byte("# start"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := cursoradapter.NewAdapter(dir)
	adapter.LookPath = func(string) (string, error) { return "/usr/bin/cursor-agent", nil }
	adapter.VerifyAgent = func(context.Context, string) error { return nil }
	adapter.Pusher = func(_ context.Context, _ string, req harness.DispatchRequest) (harness.DispatchResult, error) {
		return harness.DispatchResult{
			Dispatched: true,
			Message:    "dispatched stage " + req.StageName,
		}, nil
	}

	res, err := adapter.Dispatch(context.Background(), harness.DispatchRequest{
		ProjectDir: dir,
		StageName:  "research",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Dispatched {
		t.Fatalf("expected dispatched=true, got %+v", res)
	}
}
