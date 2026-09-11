package media

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// MaxInlinePreviewBytes prevents a protocol preview from filling the
	// terminal output with an unbounded base64 payload.
	MaxInlinePreviewBytes = 4 << 20
	protocolRasterWidth   = 128
	protocolRasterHeight  = 64
)

// InlineProtocol identifies a terminal raster protocol.
type InlineProtocol string

const (
	ProtocolNone   InlineProtocol = ""
	ProtocolKitty  InlineProtocol = "kitty"
	ProtocolSixel  InlineProtocol = "sixel"
	ProtocolITerm2 InlineProtocol = "iterm2"
	// ProtocolIterm2 is kept as a Go-style spelling alias for callers that do
	// not use the product's capital-T name.
	ProtocolIterm2 = ProtocolITerm2
)

// PreviewProtocol is an alias that makes the protocol type read naturally in
// configuration structs.
type PreviewProtocol = InlineProtocol

// ProtocolOverride controls the opt-in gate. The zero value is automatic
// (disabled unless Enabled is true); enable and disable are explicit policy
// overrides useful for tests and user configuration.
type ProtocolOverride uint8

const (
	ProtocolOverrideAuto ProtocolOverride = iota
	ProtocolOverrideEnable
	ProtocolOverrideDisable
	// OverrideAuto/OverrideEnable/OverrideDisable are concise aliases for
	// configuration readers.
	OverrideAuto    = ProtocolOverrideAuto
	OverrideEnable  = ProtocolOverrideEnable
	OverrideDisable = ProtocolOverrideDisable
)

// ProtocolConfig is deliberately zero-value safe: no advanced sequence is
// emitted unless Enabled or an explicit enable override is set. Selecting a
// Protocol is itself a manual override and therefore does not require the
// environment detector to recognize the terminal. With ProtocolNone, the
// detector chooses a protocol from Environment (or the process environment).
type ProtocolConfig struct {
	Enabled  bool
	Protocol InlineProtocol
	Force    bool
	Override ProtocolOverride

	// Environment is injectable for deterministic detection tests. Nil reads
	// the current process environment when automatic detection is requested.
	Environment map[string]string

	// Width and Height are optional terminal-cell hints for iTerm2 metadata.
	// They do not change the mandatory card's layout.
	Width  int
	Height int

	// MaxBytes tightens the default encoded protocol payload limit when set.
	MaxBytes int
}

// ProtocolPreview is the result of an optional inline render. CardVisible is
// always true when this package returns a value, including after a clear or a
// capability/configuration failure.
type ProtocolPreview struct {
	Protocol InlineProtocol
	Sequence string
	// Output mirrors Sequence for consumers that name terminal output rather
	// than a protocol sequence.
	Output      string
	Available   bool
	Detected    bool
	CardVisible bool
	Cleared     bool
	Reason      string
}

// String returns the terminal sequence, if one was emitted.
func (p ProtocolPreview) String() string {
	return p.Sequence
}

// ProtocolRenderer is the injectable pure encoder boundary.
type ProtocolRenderer func(image.Image, ProtocolConfig) (ProtocolPreview, error)

// ProtocolCommandOptions supplies injectable decode and encode functions to
// NewProtocolPreviewCmd.
type ProtocolCommandOptions struct {
	Config   ProtocolConfig
	Decode   ImageDecoder
	Renderer ProtocolRenderer
}

// ProtocolPreviewMsg is the asynchronous result delivered to a Bubble Tea
// model. Card metadata remains outside this message; only a bounded sequence
// and its state are returned.
type ProtocolPreviewMsg struct {
	Path    string
	Preview ProtocolPreview
	// Result mirrors Preview for callers using result-oriented message naming.
	Result ProtocolPreview
	Err    error
}

// InlinePreviewMsg is a descriptive alias for ProtocolPreviewMsg.
type InlinePreviewMsg = ProtocolPreviewMsg

// PreviewClearReason describes a viewport event that invalidates terminal
// raster placement.
type PreviewClearReason string

const (
	PreviewClearScroll PreviewClearReason = "scroll"
	PreviewClearResize PreviewClearReason = "resize"
)

// DetectProtocols returns supported protocol candidates in deterministic
// priority order. Detection is intentionally conservative and only informs an
// explicitly enabled preview; it never enables one by itself.
func DetectProtocols(environment map[string]string) []InlineProtocol {
	environment = normalizeEnvironment(environment)
	termProgram := strings.ToLower(environment["term_program"])
	term := strings.ToLower(environment["term"])
	lcTerminal := strings.ToLower(environment["lc_terminal"])

	protocols := make([]InlineProtocol, 0, 3)
	if termProgram == "iterm.app" || termProgram == "iterm2" || lcTerminal == "iterm.app" || lcTerminal == "iterm2" || environment["iterm_session_id"] != "" {
		protocols = append(protocols, ProtocolITerm2)
	}
	if termProgram == "kitty" || environment["kitty_window_id"] != "" || strings.Contains(term, "xterm-kitty") {
		protocols = append(protocols, ProtocolKitty)
	}
	if strings.Contains(term, "sixel") || strings.Contains(term, "mlterm") || strings.Contains(term, "mintty") {
		protocols = append(protocols, ProtocolSixel)
	}
	return protocols
}

// DetectProtocol returns the first best-effort terminal protocol candidate.
func DetectProtocol(environment map[string]string) InlineProtocol {
	protocols := DetectProtocols(environment)
	if len(protocols) == 0 {
		return ProtocolNone
	}
	return protocols[0]
}

// ResolveProtocol applies the explicit opt-in gate and returns the selected
// protocol plus whether it may emit output. A selected Protocol is treated as
// a manual user override; automatic detection is used only when it is empty.
func ResolveProtocol(config ProtocolConfig) (InlineProtocol, bool) {
	protocol, enabled, _ := resolveProtocol(config)
	return protocol, enabled
}

// RenderProtocolPreview encodes an image for an explicitly enabled terminal
// protocol. The text card remains represented by CardVisible even when the
// protocol is disabled, undetected, unsupported, or cleared.
func RenderProtocolPreview(img image.Image, config ProtocolConfig) (ProtocolPreview, error) {
	preview := ProtocolPreview{CardVisible: true}
	protocol, enabled, detected := resolveProtocol(config)
	preview.Protocol = protocol
	preview.Detected = detected
	if !enabled {
		preview.Reason = protocolDisabledReason(config)
		return preview, nil
	}
	if img == nil {
		return preview, errors.New("render protocol preview: image is nil")
	}

	sequence, err := encodeProtocol(img, protocol, config)
	if err != nil {
		preview.Reason = err.Error()
		return preview, err
	}
	preview.Sequence = sequence
	preview.Output = sequence
	preview.Available = sequence != ""
	if !preview.Available {
		preview.Reason = "protocol preview produced no output"
	}
	return preview, nil
}

// RenderInlinePreview is an alias for callers that use the UI vocabulary.
func RenderInlinePreview(img image.Image, config ProtocolConfig) (ProtocolPreview, error) {
	return RenderProtocolPreview(img, config)
}

// NewProtocolPreviewCmd creates an asynchronous image decode and protocol
// encode command. A disabled or undetected configuration returns immediately
// without opening the image path.
func NewProtocolPreviewCmd(path string, options ProtocolCommandOptions) tea.Cmd {
	return func() tea.Msg {
		if _, enabled := ResolveProtocol(options.Config); !enabled {
			preview, err := RenderProtocolPreview(nil, options.Config)
			return ProtocolPreviewMsg{Path: path, Preview: preview, Result: preview, Err: err}
		}

		decoder := options.Decode
		if decoder == nil {
			decoder = DecodeImageFile
		}
		renderer := options.Renderer
		if renderer == nil {
			renderer = RenderProtocolPreview
		}
		img, err := decoder(path)
		if err != nil {
			return ProtocolPreviewMsg{Path: path, Err: err}
		}
		preview, err := renderer(img, options.Config)
		return ProtocolPreviewMsg{Path: path, Preview: preview, Result: preview, Err: err}
	}
}

// ProtocolPreviewCmd is the default protocol command wrapper. The optional
// renderer makes the asynchronous boundary straightforward to test.
func ProtocolPreviewCmd(path string, config ProtocolConfig, renderer ...ProtocolRenderer) tea.Cmd {
	var injected ProtocolRenderer
	if len(renderer) > 0 {
		injected = renderer[0]
	}
	return NewProtocolPreviewCmd(path, ProtocolCommandOptions{
		Config:   config,
		Renderer: injected,
	})
}

// ClearProtocolPreview returns a conservative erase sequence for a protocol
// preview. The next normal TUI render redraws the mandatory card. Kitty has an
// explicit delete-all operation; Sixel and iTerm2 use a line erase so the
// raster is not left as the active card moves during scroll or resize.
func ClearProtocolPreview(protocol InlineProtocol) string {
	switch protocol {
	case ProtocolKitty:
		return "\x1b_Ga=d,d=a\x1b\\\x1b[2K\r"
	case ProtocolSixel, ProtocolITerm2:
		return "\x1b[2K\r"
	default:
		return ""
	}
}

// Clear marks a protocol preview invalid while preserving card navigation.
func (p ProtocolPreview) Clear(reason PreviewClearReason) ProtocolPreview {
	p.Sequence = ClearProtocolPreview(p.Protocol)
	p.Output = p.Sequence
	p.Available = false
	p.Cleared = true
	p.CardVisible = true
	if reason == "" {
		reason = PreviewClearResize
	}
	p.Reason = "inline preview cleared on " + string(reason)
	return p
}

// HandleViewportEvent clears inline pixels for the events that can invalidate
// their terminal placement. Other events leave the preview unchanged.
func (p ProtocolPreview) HandleViewportEvent(event PreviewClearReason) ProtocolPreview {
	if event == PreviewClearScroll || event == PreviewClearResize {
		return p.Clear(event)
	}
	return p
}

// ClearInlinePreviewOnScroll is a small explicit helper for TUI Update code.
func ClearInlinePreviewOnScroll(preview ProtocolPreview) ProtocolPreview {
	return preview.Clear(PreviewClearScroll)
}

// ClearInlinePreviewOnResize is a small explicit helper for TUI Update code.
func ClearInlinePreviewOnResize(preview ProtocolPreview) ProtocolPreview {
	return preview.Clear(PreviewClearResize)
}

// ClearProtocolPreviewCmd wraps viewport invalidation in a tea.Cmd-shaped
// message boundary so a caller can use the same non-blocking update path as a
// freshly rendered preview.
func ClearProtocolPreviewCmd(path string, preview ProtocolPreview, reason PreviewClearReason) tea.Cmd {
	return func() tea.Msg {
		cleared := preview.Clear(reason)
		return ProtocolPreviewMsg{Path: path, Preview: cleared, Result: cleared}
	}
}

func resolveProtocol(config ProtocolConfig) (InlineProtocol, bool, bool) {
	if config.Override == ProtocolOverrideDisable {
		return ProtocolNone, false, false
	}
	if !config.Enabled && config.Override != ProtocolOverrideEnable && !config.Force {
		return ProtocolNone, false, false
	}
	if config.Protocol != ProtocolNone {
		if !validProtocol(config.Protocol) {
			return ProtocolNone, false, false
		}
		return config.Protocol, true, false
	}
	protocol := DetectProtocol(config.Environment)
	if protocol == ProtocolNone {
		return ProtocolNone, false, false
	}
	return protocol, true, true
}

func protocolDisabledReason(config ProtocolConfig) string {
	if config.Override == ProtocolOverrideDisable || (!config.Enabled && !config.Force && config.Override != ProtocolOverrideEnable) {
		return "advanced inline preview disabled"
	}
	if config.Protocol != ProtocolNone && !validProtocol(config.Protocol) {
		return "advanced inline preview protocol is unsupported"
	}
	return "no supported inline image protocol detected"
}

func validProtocol(protocol InlineProtocol) bool {
	switch protocol {
	case ProtocolKitty, ProtocolSixel, ProtocolITerm2:
		return true
	default:
		return false
	}
}

func normalizeEnvironment(environment map[string]string) map[string]string {
	if environment == nil {
		environment = make(map[string]string)
		for _, entry := range os.Environ() {
			key, value, ok := strings.Cut(entry, "=")
			if ok {
				environment[strings.ToLower(key)] = value
			}
		}
		return environment
	}
	result := make(map[string]string, len(environment))
	for key, value := range environment {
		result[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return result
}

func encodeProtocol(img image.Image, protocol InlineProtocol, config ProtocolConfig) (string, error) {
	maxBytes := config.MaxBytes
	if maxBytes <= 0 {
		maxBytes = MaxInlinePreviewBytes
	}
	switch protocol {
	case ProtocolKitty:
		return encodeKitty(img, maxBytes)
	case ProtocolSixel:
		return encodeSixel(img, maxBytes)
	case ProtocolITerm2:
		return encodeITerm2(img, config, maxBytes)
	default:
		return "", errors.New("render protocol preview: unsupported protocol")
	}
}

func encodeKitty(img image.Image, maxBytes int) (string, error) {
	payload, err := encodePNG(img, maxBytes)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(payload)
	bounds := img.Bounds()
	sequence := "\x1b_Ga=T,f=100,t=d,m=0,s=" + strconv.Itoa(bounds.Dx()) + ",v=" + strconv.Itoa(bounds.Dy()) + ";" + encoded + "\x1b\\"
	return boundedSequence(sequence, maxBytes)
}

func encodeITerm2(img image.Image, config ProtocolConfig, maxBytes int) (string, error) {
	payload, err := encodePNG(img, maxBytes)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(payload)
	metadata := "inline=1;preserveAspectRatio=1;size=" + strconv.Itoa(len(payload))
	if config.Width > 0 {
		metadata += ";width=" + strconv.Itoa(config.Width)
	}
	if config.Height > 0 {
		metadata += ";height=" + strconv.Itoa(config.Height)
	}
	sequence := "\x1b]1337;File=" + metadata + ":" + encoded + "\x07"
	return boundedSequence(sequence, maxBytes)
}

func encodePNG(img image.Image, maxBytes int) ([]byte, error) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return nil, fmt.Errorf("encode protocol preview as PNG: %w", err)
	}
	// Base64 expands by four thirds. Reject an oversized source before doing
	// that allocation; the caller can fall back to the bounded mosaic.
	if buffer.Len() > maxBytes*3/4 {
		return nil, fmt.Errorf("protocol preview exceeds %d-byte limit", maxBytes)
	}
	return buffer.Bytes(), nil
}

func encodeSixel(img image.Image, maxBytes int) (string, error) {
	width, height := rasterDimensions(img.Bounds(), protocolRasterWidth, protocolRasterHeight)
	pixels := make([]int, width*height)
	used := make([]bool, 256)
	for y := 0; y < height; y++ {
		sourceY := sampleCoordinate(img.Bounds().Min.Y, img.Bounds().Dy(), y, height)
		for x := 0; x < width; x++ {
			sourceX := sampleCoordinate(img.Bounds().Min.X, img.Bounds().Dx(), x, width)
			index := ansi256Index(img.At(sourceX, sourceY))
			pixels[y*width+x] = index
			used[index] = true
		}
	}

	var output strings.Builder
	output.Grow(width * height / 2)
	output.WriteString("\x1bPq\"1;1;")
	output.WriteString(strconv.Itoa(width))
	output.WriteByte(';')
	output.WriteString(strconv.Itoa(height))
	for index, present := range used {
		if !present {
			continue
		}
		red, green, blue := ansi256RGB(index)
		fmt.Fprintf(&output, "#%d;2;%d;%d;%d", index, red*100/255, green*100/255, blue*100/255)
	}

	for band := 0; band < height; band += 6 {
		bandHeight := height - band
		if bandHeight > 6 {
			bandHeight = 6
		}
		for index, present := range used {
			if !present {
				continue
			}
			output.WriteByte('#')
			output.WriteString(strconv.Itoa(index))
			for x := 0; x < width; x++ {
				mask := 0
				for offset := 0; offset < bandHeight; offset++ {
					if pixels[(band+offset)*width+x] == index {
						mask |= 1 << offset
					}
				}
				output.WriteByte(byte('?' + mask))
			}
			output.WriteByte('$')
		}
		output.WriteByte('-')
	}
	output.WriteString("\x1b\\")
	return boundedSequence(output.String(), maxBytes)
}

func boundedSequence(sequence string, maxBytes int) (string, error) {
	if len(sequence) > maxBytes {
		return "", fmt.Errorf("protocol preview exceeds %d-byte limit", maxBytes)
	}
	return sequence, nil
}

func rasterDimensions(bounds image.Rectangle, maxWidth, maxHeight int) (int, int) {
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return 1, 1
	}
	if width > maxWidth {
		height = maxInt(1, int(mathRound(float64(height)*float64(maxWidth)/float64(width))))
		width = maxWidth
	}
	if height > maxHeight {
		width = maxInt(1, int(mathRound(float64(width)*float64(maxHeight)/float64(height))))
		height = maxHeight
	}
	return width, height
}

func mathRound(value float64) float64 {
	if value < 0 {
		return value - 0.5
	}
	return value + 0.5
}

func ansi256RGB(index int) (int, int, int) {
	index = maxInt(0, index)
	if index < 16 {
		palette := [16][3]int{
			{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
			{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
			{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
			{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
		}
		return palette[index][0], palette[index][1], palette[index][2]
	}
	if index < 232 {
		index -= 16
		return cubeValue(index / 36), cubeValue((index / 6) % 6), cubeValue(index % 6)
	}
	gray := 8 + (index-232)*10
	return gray, gray, gray
}
