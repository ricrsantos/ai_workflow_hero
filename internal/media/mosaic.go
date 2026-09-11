// Package media contains terminal-safe image preview primitives.
package media

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// MaxMosaicWidth and MaxMosaicHeight keep a malformed resize or an
	// accidentally unbounded terminal dimension from allocating an enormous
	// ANSI string.
	MaxMosaicWidth  = 256
	MaxMosaicHeight = 128

	// MosaicUnavailableMessage is the actionable fallback shown when the
	// terminal cannot render the 256-colour ANSI mosaic.
	MosaicUnavailableMessage = "Preview unavailable (terminal color depth insufficient). Use [o] to open in viewer."
)

// ColorDepth describes the number of terminal colours available to a
// renderer. Mosaic output intentionally requires the 256-colour profile.
type ColorDepth int

const (
	// ColorDepthUnknown is a conservative value: an unknown terminal profile
	// must not cause colour-heavy output to be emitted.
	ColorDepthUnknown ColorDepth = 0
	ColorDepthANSI    ColorDepth = 16
	ColorDepth256     ColorDepth = 256
	ColorDepthTrue    ColorDepth = 1 << 24
	// ColorDepthANSI256 and ColorDepthTrueColor are descriptive aliases for
	// callers that use terminal-profile terminology.
	ColorDepthANSI256   = ColorDepth256
	ColorDepthTrueColor = ColorDepthTrue
)

// SupportsMosaic reports whether the terminal can display the mosaic's
// indexed foreground and background SGR sequences.
func (d ColorDepth) SupportsMosaic() bool {
	return d >= ColorDepth256
}

// MosaicConfig bounds a mosaic to the currently available pane. Width and
// Height are terminal cells, not source-image pixels. A zero ColorDepth is
// treated as unknown and therefore degrades to card-only guidance.
type MosaicConfig struct {
	Width      int
	Height     int
	ColorDepth ColorDepth
	// MaxWidth and MaxHeight allow callers to tighten the package bounds for a
	// particular pane. Zero uses MaxMosaicWidth/MaxMosaicHeight.
	MaxWidth  int
	MaxHeight int
}

// DefaultMosaicConfig returns a usable 256-colour configuration for callers
// that already know the terminal profile.
func DefaultMosaicConfig(width, height int) MosaicConfig {
	return MosaicConfig{
		Width:      width,
		Height:     height,
		ColorDepth: ColorDepth256,
	}
}

// MosaicResult is the immutable value sent back to the TUI after rendering.
// Text contains only the preview region; card text and navigation remain the
// caller's responsibility.
type MosaicResult struct {
	Text      string
	Width     int
	Height    int
	Available bool
	Message   string
}

// String returns the rendered preview text, which is convenient when a
// result is passed directly to a transcript component.
func (r MosaicResult) String() string {
	return r.Text
}

// MosaicRenderer is the injectable, pure rendering boundary. It must not
// perform filesystem or terminal I/O.
type MosaicRenderer func(image.Image, MosaicConfig) (MosaicResult, error)

// ImageDecoder is the I/O boundary used by NewMosaicCmd. Tests and callers
// can inject a decoder without creating a temporary file.
type ImageDecoder func(string) (image.Image, error)

// MosaicCommandOptions supplies injectable work for NewMosaicCmd. Nil
// functions use the package defaults.
type MosaicCommandOptions struct {
	Config   MosaicConfig
	Decode   ImageDecoder
	Renderer MosaicRenderer
}

// MosaicMsg is returned by a mosaic tea.Cmd. The original path is an opaque
// correlation key; image bytes never cross the Bubble Tea message boundary.
type MosaicMsg struct {
	Path   string
	Result MosaicResult
	Err    error
}

// MosaicRenderedMsg is a descriptive alias for consumers that prefer a
// message name over the shorter MosaicMsg.
type MosaicRenderedMsg = MosaicMsg

var errNilImage = errors.New("render mosaic: image is nil")

// RenderMosaic converts an image into bounded Unicode upper-half blocks with
// indexed ANSI foreground/background colours. It is pure with respect to the
// supplied image and configuration. The output never exceeds the configured
// terminal cell bounds.
func RenderMosaic(img image.Image, config MosaicConfig) (MosaicResult, error) {
	if !config.ColorDepth.SupportsMosaic() {
		return MosaicResult{
			Available: false,
			Message:   MosaicUnavailableMessage,
		}, nil
	}
	if img == nil {
		return MosaicResult{}, errNilImage
	}

	width, height, ok := mosaicDimensions(img.Bounds(), config)
	if !ok {
		return MosaicResult{
			Available: false,
			Message:   "Preview unavailable (the preview area has no cells).",
		}, nil
	}

	var output strings.Builder
	// Each cell has two SGR colour clauses, one rune, and a reset. This is an
	// estimate only; strings.Builder grows safely if a terminal uses a longer
	// escape representation in the future.
	output.Grow(width * height * 32)
	for row := 0; row < height; row++ {
		if row > 0 {
			output.WriteByte('\n')
		}
		topY := sampleCoordinate(img.Bounds().Min.Y, img.Bounds().Dy(), row*2, height*2)
		bottomY := sampleCoordinate(img.Bounds().Min.Y, img.Bounds().Dy(), row*2+1, height*2)
		for column := 0; column < width; column++ {
			topX := sampleCoordinate(img.Bounds().Min.X, img.Bounds().Dx(), column, width)
			top := ansi256Index(img.At(topX, topY))
			bottom := ansi256Index(img.At(topX, bottomY))
			fmt.Fprintf(&output, "\x1b[38;5;%d;48;5;%dm▀\x1b[0m", top, bottom)
		}
	}

	return MosaicResult{
		Text:      output.String(),
		Width:     width,
		Height:    height,
		Available: true,
	}, nil
}

// RenderMosaicText is a compact convenience wrapper for callers that need
// only the preview text and already know the terminal has 256 colours.
func RenderMosaicText(img image.Image, width, height int) (string, error) {
	result, err := RenderMosaic(img, DefaultMosaicConfig(width, height))
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// DecodeImageFile decodes an image from a path. It is intended to be called
// by a tea.Cmd (such as NewMosaicCmd), keeping blocking file work out of
// Update and View.
func DecodeImageFile(path string) (image.Image, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("decode image: path is empty")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("decode image: open: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return img, nil
}

// NewMosaicCmd creates an asynchronous command that decodes and renders an
// image. Both decoding and rendering happen only when Bubble Tea executes the
// returned command; constructing it is non-blocking.
func NewMosaicCmd(path string, options MosaicCommandOptions) tea.Cmd {
	return func() tea.Msg {
		decoder := options.Decode
		if decoder == nil {
			decoder = DecodeImageFile
		}
		renderer := options.Renderer
		if renderer == nil {
			renderer = RenderMosaic
		}

		img, err := decoder(path)
		if err != nil {
			return MosaicMsg{Path: path, Err: err}
		}
		result, err := renderer(img, options.Config)
		return MosaicMsg{Path: path, Result: result, Err: err}
	}
}

// MosaicCmd is the default command wrapper. An optional renderer is provided
// for focused tests or callers with a custom image sampler.
func MosaicCmd(path string, config MosaicConfig, renderer ...MosaicRenderer) tea.Cmd {
	var injected MosaicRenderer
	if len(renderer) > 0 {
		injected = renderer[0]
	}
	return NewMosaicCmd(path, MosaicCommandOptions{
		Config:   config,
		Renderer: injected,
	})
}

func mosaicDimensions(bounds image.Rectangle, config MosaicConfig) (int, int, bool) {
	if config.Width <= 0 || config.Height <= 0 || bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return 0, 0, false
	}
	maxWidth := config.MaxWidth
	if maxWidth <= 0 || maxWidth > MaxMosaicWidth {
		maxWidth = MaxMosaicWidth
	}
	maxHeight := config.MaxHeight
	if maxHeight <= 0 || maxHeight > MaxMosaicHeight {
		maxHeight = MaxMosaicHeight
	}
	width := config.Width
	if width > maxWidth {
		width = maxWidth
	}
	height := config.Height
	if height > maxHeight {
		height = maxHeight
	}
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}

	// One upper-half block represents two source-pixel rows. Fit the source
	// aspect ratio into the available cell rectangle while preserving both
	// requested bounds.
	sourceAspect := float64(bounds.Dx()) / float64(bounds.Dy())
	cellAspect := float64(width) / float64(height*2)
	if cellAspect > sourceAspect {
		width = maxInt(1, int(math.Round(sourceAspect*float64(height*2))))
		if width > maxInt(1, config.Width) {
			width = config.Width
		}
		if width > maxWidth {
			width = maxWidth
		}
	} else {
		height = maxInt(1, int(math.Round(float64(width)/(sourceAspect*2))))
		if height > config.Height {
			height = config.Height
		}
		if height > maxHeight {
			height = maxHeight
		}
	}
	return width, height, width > 0 && height > 0
}

func sampleCoordinate(minimum, size, index, count int) int {
	if size <= 1 || count <= 0 {
		return minimum
	}
	// Sample at the centre of each output bucket. int64 avoids overflow if a
	// caller supplies a large, but still bounded, source image rectangle.
	coordinate := minimum + int((int64(index)*2+1)*int64(size)/(2*int64(count)))
	maximum := minimum + size - 1
	if coordinate < minimum {
		return minimum
	}
	if coordinate > maximum {
		return maximum
	}
	return coordinate
}

func ansi256Index(value color.Color) int {
	nrgba, ok := color.NRGBAModel.Convert(value).(color.NRGBA)
	if !ok {
		return 0
	}
	// Composite transparency against the terminal's conventional black
	// background so a transparent PNG cannot accidentally render as white.
	alpha := int(nrgba.A)
	r := int(nrgba.R) * alpha / 255
	g := int(nrgba.G) * alpha / 255
	b := int(nrgba.B) * alpha / 255

	bestIndex := 0
	bestDistance := int(^uint(0) >> 1)
	for red := 0; red < 6; red++ {
		for green := 0; green < 6; green++ {
			for blue := 0; blue < 6; blue++ {
				index := 16 + red*36 + green*6 + blue
				candidateR := cubeValue(red)
				candidateG := cubeValue(green)
				candidateB := cubeValue(blue)
				distance := colorDistance(r, g, b, candidateR, candidateG, candidateB)
				if distance < bestDistance {
					bestIndex = index
					bestDistance = distance
				}
			}
		}
	}
	for gray := 0; gray < 24; gray++ {
		value := 8 + gray*10
		distance := colorDistance(r, g, b, value, value, value)
		if distance < bestDistance {
			bestIndex = 232 + gray
			bestDistance = distance
		}
	}
	return bestIndex
}

func colorDistance(r1, g1, b1, r2, g2, b2 int) int {
	red := r1 - r2
	green := g1 - g2
	blue := b1 - b2
	return red*red + green*green + blue*blue
}

func cubeValue(index int) int {
	if index == 0 {
		return 0
	}
	return 55 + index*40
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
