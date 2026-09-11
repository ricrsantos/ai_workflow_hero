package media

import (
	"image"
	"strings"
	"testing"
)

func TestRenderProtocolPreviewIsOffByDefault(t *testing.T) {
	preview, err := RenderProtocolPreview(testMosaicImage(2, 2), ProtocolConfig{
		Environment: map[string]string{
			"TERM_PROGRAM":    "kitty",
			"KITTY_WINDOW_ID": "42",
		},
	})
	if err != nil {
		t.Fatalf("RenderProtocolPreview() error = %v", err)
	}
	if preview.Available || preview.Sequence != "" || !preview.CardVisible {
		t.Fatalf("default protocol preview should be card-only: %+v", preview)
	}
	if preview.Reason != "advanced inline preview disabled" {
		t.Fatalf("reason = %q", preview.Reason)
	}
}

func TestRenderProtocolPreviewEmitsOnlyForExplicitOptIn(t *testing.T) {
	tests := []struct {
		name   string
		config ProtocolConfig
		prefix string
	}{
		{
			name:   "kitty manual selection",
			config: ProtocolConfig{Enabled: true, Protocol: ProtocolKitty},
			prefix: "\x1b_G",
		},
		{
			name:   "sixel detected",
			config: ProtocolConfig{Enabled: true, Environment: map[string]string{"TERM": "xterm-sixel"}},
			prefix: "\x1bPq",
		},
		{
			name:   "iterm detected",
			config: ProtocolConfig{Enabled: true, Environment: map[string]string{"TERM_PROGRAM": "iTerm.app"}},
			prefix: "\x1b]1337;File=",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preview, err := RenderProtocolPreview(testMosaicImage(2, 2), test.config)
			if err != nil {
				t.Fatalf("RenderProtocolPreview() error = %v", err)
			}
			if !preview.Available || !preview.CardVisible {
				t.Fatalf("expected an available card-preserving preview: %+v", preview)
			}
			if !strings.HasPrefix(preview.Sequence, test.prefix) {
				t.Fatalf("sequence prefix = %q, want %q", preview.Sequence[:minString(len(preview.Sequence), len(test.prefix))], test.prefix)
			}
		})
	}
}

func TestRenderProtocolPreviewCanBeForceDisabled(t *testing.T) {
	preview, err := RenderProtocolPreview(testMosaicImage(2, 2), ProtocolConfig{
		Enabled:  true,
		Protocol: ProtocolKitty,
		Override: ProtocolOverrideDisable,
	})
	if err != nil {
		t.Fatalf("RenderProtocolPreview() error = %v", err)
	}
	if preview.Available || preview.Sequence != "" || !preview.CardVisible {
		t.Fatalf("disable override emitted a sequence: %+v", preview)
	}
}

func TestProtocolPreviewClearsOnScrollAndResizeButKeepsCard(t *testing.T) {
	preview, err := RenderProtocolPreview(testMosaicImage(2, 2), ProtocolConfig{
		Enabled:  true,
		Protocol: ProtocolKitty,
	})
	if err != nil {
		t.Fatalf("RenderProtocolPreview() error = %v", err)
	}
	for _, event := range []PreviewClearReason{PreviewClearScroll, PreviewClearResize} {
		cleared := preview.HandleViewportEvent(event)
		if !cleared.Cleared || cleared.Available || !cleared.CardVisible {
			t.Fatalf("event %q did not preserve card while clearing: %+v", event, cleared)
		}
		if cleared.Sequence == "" {
			t.Fatalf("event %q produced no clear sequence", event)
		}
	}
}

func TestNewProtocolPreviewCmdSkipsDecodeWhenDisabled(t *testing.T) {
	decoded := false
	cmd := NewProtocolPreviewCmd("must-not-open", ProtocolCommandOptions{
		Decode: func(string) (image.Image, error) {
			decoded = true
			return testMosaicImage(1, 1), nil
		},
	})
	resultMessage := cmd()
	message, ok := resultMessage.(ProtocolPreviewMsg)
	if !ok {
		t.Fatalf("command message type = %T, want ProtocolPreviewMsg", resultMessage)
	}
	if decoded || message.Err != nil || message.Preview.Available {
		t.Fatalf("disabled command decoded or emitted output: decoded=%v msg=%+v", decoded, message)
	}
}

func TestDetectProtocolsIsDeterministicAndConservative(t *testing.T) {
	if got := DetectProtocol(map[string]string{"TERM": "xterm-256color"}); got != ProtocolNone {
		t.Fatalf("ordinary terminal detected protocol %q", got)
	}
	if got := DetectProtocol(map[string]string{"TERM_PROGRAM": "kitty"}); got != ProtocolKitty {
		t.Fatalf("kitty detection = %q", got)
	}
	if got := DetectProtocol(map[string]string{"TERM_PROGRAM": "iTerm.app"}); got != ProtocolITerm2 {
		t.Fatalf("iTerm detection = %q", got)
	}
}

func minString(left, right int) int {
	if left < right {
		return left
	}
	return right
}
