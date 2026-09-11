package media

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestRenderMosaicRespectsBoundsAndProducesANSIBlocks(t *testing.T) {
	img := testMosaicImage(8, 6)
	result, err := RenderMosaic(img, MosaicConfig{
		Width:      7,
		Height:     5,
		ColorDepth: ColorDepth256,
	})
	if err != nil {
		t.Fatalf("RenderMosaic() error = %v", err)
	}
	if !result.Available || result.Text == "" {
		t.Fatalf("expected a non-empty available mosaic: %+v", result)
	}
	if result.Width > 7 || result.Height > 5 {
		t.Fatalf("mosaic exceeded requested bounds: %dx%d", result.Width, result.Height)
	}
	if got := visibleMosaicWidth(result.Text); got != result.Width {
		t.Fatalf("visible width = %d, result width = %d", got, result.Width)
	}
	if got := strings.Count(result.Text, "\n") + 1; got != result.Height {
		t.Fatalf("line count = %d, result height = %d", got, result.Height)
	}
	if !strings.Contains(result.Text, "\x1b[38;5;") || !strings.Contains(result.Text, "▀") {
		t.Fatalf("mosaic does not contain expected ANSI block output")
	}
	// Keep this an invariant test: the exact ANSI stream is intentionally not
	// frozen in a large snapshot.
	if len(result.Text) > MaxMosaicWidth*MaxMosaicHeight*40 {
		t.Fatalf("mosaic output is not bounded: %d bytes", len(result.Text))
	}
}

func TestRenderMosaicDegradesBelow256Colours(t *testing.T) {
	result, err := RenderMosaic(testMosaicImage(2, 2), MosaicConfig{
		Width:      4,
		Height:     4,
		ColorDepth: ColorDepthANSI,
	})
	if err != nil {
		t.Fatalf("RenderMosaic() error = %v", err)
	}
	if result.Available || result.Text != "" {
		t.Fatalf("low-colour preview should be unavailable: %+v", result)
	}
	if result.Message != MosaicUnavailableMessage {
		t.Fatalf("message = %q, want %q", result.Message, MosaicUnavailableMessage)
	}
}

func TestRenderMosaicResizeBoundsNeverPanics(t *testing.T) {
	tests := []MosaicConfig{
		{},
		{Width: -1, Height: 4, ColorDepth: ColorDepth256},
		{Width: 4, Height: -1, ColorDepth: ColorDepth256},
		{Width: 1, Height: 1, ColorDepth: ColorDepth256},
		{Width: MaxMosaicWidth * 100, Height: MaxMosaicHeight * 100, ColorDepth: ColorDepth256},
	}
	for _, config := range tests {
		result, err := RenderMosaic(testMosaicImage(3, 3), config)
		if err != nil && config.Width > 0 && config.Height > 0 && config.ColorDepth >= ColorDepth256 {
			t.Fatalf("config %+v returned unexpected error: %v", config, err)
		}
		if result.Width > MaxMosaicWidth || result.Height > MaxMosaicHeight {
			t.Fatalf("config %+v escaped package bounds: %dx%d", config, result.Width, result.Height)
		}
	}
}

func TestNewMosaicCmdUsesInjectedDecoderAndRenderer(t *testing.T) {
	called := false
	cmd := NewMosaicCmd("opaque-key", MosaicCommandOptions{
		Config: DefaultMosaicConfig(3, 2),
		Decode: func(path string) (image.Image, error) {
			if path != "opaque-key" {
				t.Fatalf("decoder path = %q", path)
			}
			return testMosaicImage(1, 1), nil
		},
		Renderer: func(image.Image, MosaicConfig) (MosaicResult, error) {
			called = true
			return MosaicResult{Text: "injected", Width: 1, Height: 1, Available: true}, nil
		},
	})
	resultMessage := cmd()
	message, ok := resultMessage.(MosaicMsg)
	if !ok {
		t.Fatalf("command message type = %T, want MosaicMsg", resultMessage)
	}
	if message.Err != nil || message.Result.Text != "injected" || !called {
		t.Fatalf("unexpected command result: %+v, called=%v", message, called)
	}
}

func testMosaicImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / maxInt(1, width-1)),
				G: uint8((y * 255) / maxInt(1, height-1)),
				B: 96,
				A: 255,
			})
		}
	}
	return img
}

func visibleMosaicWidth(value string) int {
	width := 0
	for index := 0; index < len(value); {
		if value[index] == '\x1b' {
			index++
			if index < len(value) && value[index] == '[' {
				index++
				for index < len(value) && (value[index] < '@' || value[index] > '~') {
					index++
				}
				if index < len(value) {
					index++
				}
				continue
			}
		}
		if value[index] == '\n' {
			break
		}
		if strings.HasPrefix(value[index:], "▀") {
			width++
			index += len("▀")
			continue
		}
		index++
	}
	return width
}
