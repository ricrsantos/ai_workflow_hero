package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ImageFormat is the format detected from image bytes, never from a file
// extension or a caller-provided MIME type.
type ImageFormat string

const (
	ImageFormatPNG  ImageFormat = "png"
	ImageFormatJPEG ImageFormat = "jpeg"
	ImageFormatGIF  ImageFormat = "gif"
	ImageFormatWebP ImageFormat = "webp"
)

// MIME types for the supported image formats.
const (
	MIMETypePNG  = "image/png"
	MIMETypeJPEG = "image/jpeg"
	MIMETypeGIF  = "image/gif"
	MIMETypeWebP = "image/webp"
)

// WarningCode identifies a non-fatal validation warning.
type WarningCode string

const (
	// WarningExternalPath means the canonical source path is outside the
	// configured workspace and requires an explicit policy decision.
	WarningExternalPath WarningCode = "external_path"
)

// ValidationResult contains safe image metadata and the canonical source path
// needed for materialization. It contains no image bytes.
type ValidationResult struct {
	CanonicalPath       string
	Format              ImageFormat
	MIMEType            string
	Size                int64
	Width               int
	Height              int
	Pixels              int64
	SHA256              string
	ExternalPath        bool
	ExternalPathWarning bool
	Warnings            []WarningCode
}

// HasWarning reports whether validation emitted code.
func (r ValidationResult) HasWarning(code WarningCode) bool {
	for _, warning := range r.Warnings {
		if warning == code {
			return true
		}
	}
	return false
}

// ValidationError is a safe, structured image validation error. It never
// includes the source path or image bytes in its message.
type ValidationError struct {
	Reason   error
	Format   ImageFormat
	MIMEType string
	Size     int64
	Width    int
	Height   int
}

func (e *ValidationError) Error() string {
	if e == nil || e.Reason == nil {
		return "image validation failed"
	}
	return e.Reason.Error()
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Reason
}

// Validator validates image content and source paths according to options.
// It is stateless and safe to reuse between calls.
type Validator struct {
	options ValidationOptions
}

// NewValidator creates a validator with normalized secure defaults.
func NewValidator(options ValidationOptions) *Validator {
	return &Validator{options: options.normalized()}
}

// ValidateImage validates one image path and returns detected metadata. The
// function resolves symlinks before reading and emits an external-path warning
// when Workspace is configured and the resolved path escapes it.
func ValidateImage(ctx context.Context, sourcePath string, options ValidationOptions) (ValidationResult, error) {
	return NewValidator(options).Validate(ctx, sourcePath)
}

// ValidatePath is an alias for ValidateImage.
func ValidatePath(ctx context.Context, sourcePath string, options ValidationOptions) (ValidationResult, error) {
	return ValidateImage(ctx, sourcePath, options)
}

// Validate performs magic-byte and header validation without trusting the
// extension or declared MIME type. It only decodes image configuration, never
// a full pixel buffer, and therefore avoids allocating a decompression bomb.
func (v *Validator) Validate(ctx context.Context, sourcePath string) (ValidationResult, error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return ValidationResult{}, err
	}
	options := v.options.normalized()
	canonicalPath, err := canonicalPath(sourcePath)
	if err != nil {
		return ValidationResult{}, err
	}
	external, warning := externalPathWarning(canonicalPath, options.Workspace)

	file, err := os.Open(canonicalPath)
	if err != nil {
		return ValidationResult{}, wrapFilesystemError("opening image", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return ValidationResult{}, wrapFilesystemError("statting image", err)
	}
	if !info.Mode().IsRegular() {
		return ValidationResult{}, &ValidationError{Reason: ErrInvalidImage}
	}
	if info.Size() <= 0 {
		return ValidationResult{}, &ValidationError{Reason: ErrInvalidImage}
	}
	if info.Size() > options.MaxFileBytes {
		return ValidationResult{}, &ValidationError{Reason: ErrFileTooLarge, Size: info.Size()}
	}
	if err := ctx.Err(); err != nil {
		return ValidationResult{}, err
	}

	format, err := detectFormat(file)
	if err != nil {
		return ValidationResult{}, err
	}
	width, height, err := decodeHeader(file, format, options.MaxFileBytes)
	if err != nil {
		if width > 0 && height > 0 {
			if dimensionErr := validateDimensions(format, width, height, options); dimensionErr != nil {
				return ValidationResult{}, dimensionErr
			}
		}
		return ValidationResult{}, err
	}
	validationError := validateDimensions(format, width, height, options)
	if validationError != nil {
		return ValidationResult{}, validationError
	}
	if format != ImageFormatWebP {
		if err := validateDecodedHeader(file, format, options.MaxFileBytes); err != nil {
			return ValidationResult{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return ValidationResult{}, err
	}
	hash, err := hashFile(ctx, file, options.MaxFileBytes)
	if err != nil {
		return ValidationResult{}, err
	}
	after, err := file.Stat()
	if err != nil {
		return ValidationResult{}, wrapFilesystemError("statting image after validation", err)
	}
	if after.Size() != info.Size() {
		return ValidationResult{}, ErrSourceChanged
	}

	result := ValidationResult{
		CanonicalPath:       canonicalPath,
		Format:              format,
		MIMEType:            mimeTypeForFormat(format),
		Size:                info.Size(),
		Width:               width,
		Height:              height,
		Pixels:              safePixelCount(width, height),
		SHA256:              hash,
		ExternalPath:        external,
		ExternalPathWarning: warning,
	}
	if warning {
		result.Warnings = []WarningCode{WarningExternalPath}
	}
	return result, nil
}

func canonicalPath(sourcePath string) (string, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", fmt.Errorf("validating image: source path is required")
	}
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", wrapFilesystemError("canonicalizing image path", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", wrapFilesystemError("resolving image path", err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", wrapFilesystemError("canonicalizing image path", err)
	}
	return filepath.Clean(resolved), nil
}

func externalPathWarning(canonical, workspace string) (bool, bool) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return false, false
	}
	workspacePath, err := canonicalPath(workspace)
	if err != nil {
		abs, absErr := filepath.Abs(workspace)
		if absErr != nil {
			return false, false
		}
		workspacePath = filepath.Clean(abs)
	}
	relative, err := filepath.Rel(workspacePath, canonical)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return true, true
	}
	return false, false
}

func detectFormat(file *os.File) (ImageFormat, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", wrapFilesystemError("rewinding image", err)
	}
	prefix := make([]byte, 16)
	read, err := io.ReadFull(file, prefix)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", wrapFilesystemError("reading image header", err)
	}
	prefix = prefix[:read]
	switch {
	case len(prefix) >= 8 && bytes.Equal(prefix[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return ImageFormatPNG, nil
	case len(prefix) >= 3 && prefix[0] == 0xff && prefix[1] == 0xd8 && prefix[2] == 0xff:
		return ImageFormatJPEG, nil
	case len(prefix) >= 6 && (bytes.Equal(prefix[:6], []byte("GIF87a")) || bytes.Equal(prefix[:6], []byte("GIF89a"))):
		return ImageFormatGIF, nil
	case len(prefix) >= 12 && bytes.Equal(prefix[:4], []byte("RIFF")) && bytes.Equal(prefix[8:12], []byte("WEBP")):
		return ImageFormatWebP, nil
	default:
		return "", &ValidationError{Reason: ErrUnsupportedFormat}
	}
}

func decodeHeader(file *os.File, format ImageFormat, maxFileBytes int64) (int, int, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, 0, wrapFilesystemError("rewinding image", err)
	}
	if format == ImageFormatWebP {
		data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
		if err != nil {
			return 0, 0, wrapFilesystemError("reading WebP header", err)
		}
		if int64(len(data)) > maxFileBytes {
			return 0, 0, &ValidationError{Reason: ErrFileTooLarge}
		}
		return decodeWebPHeader(data)
	}
	if format == ImageFormatPNG {
		// PNG dimensions live in the fixed IHDR header. Read them before asking
		// image/png to inspect any later chunks so a hostile pixel count is
		// rejected before a decoder can allocate work for it.
		header := make([]byte, 24)
		read, err := io.ReadFull(file, header)
		if read >= 24 && bytes.Equal(header[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
			width := int(binary.BigEndian.Uint32(header[16:20]))
			height := int(binary.BigEndian.Uint32(header[20:24]))
			if err != nil || !bytes.Equal(header[12:16], []byte("IHDR")) {
				return width, height, &ValidationError{Reason: ErrInvalidImage, Format: format}
			}
			return width, height, nil
		}
		return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: format}
	}
	if format == ImageFormatGIF {
		header := make([]byte, 10)
		read, err := io.ReadFull(file, header)
		if read >= len(header) && (bytes.Equal(header[:6], []byte("GIF87a")) || bytes.Equal(header[:6], []byte("GIF89a"))) {
			width := int(binary.LittleEndian.Uint16(header[6:8]))
			height := int(binary.LittleEndian.Uint16(header[8:10]))
			if err != nil {
				return width, height, &ValidationError{Reason: ErrInvalidImage, Format: format}
			}
			return width, height, nil
		}
		return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: format}
	}
	config, detected, err := image.DecodeConfig(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: format}
	}
	if !formatMatchesDecoder(format, detected) {
		return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: format}
	}
	return config.Width, config.Height, nil
}

func validateDecodedHeader(file *os.File, format ImageFormat, maxFileBytes int64) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return wrapFilesystemError("rewinding image", err)
	}
	config, detected, err := image.DecodeConfig(io.LimitReader(file, maxFileBytes+1))
	if err != nil || !formatMatchesDecoder(format, detected) || config.Width <= 0 || config.Height <= 0 {
		return &ValidationError{Reason: ErrInvalidImage, Format: format}
	}
	return nil
}

func formatMatchesDecoder(format ImageFormat, detected string) bool {
	switch format {
	case ImageFormatPNG:
		return detected == "png"
	case ImageFormatJPEG:
		return detected == "jpeg"
	case ImageFormatGIF:
		return detected == "gif"
	default:
		return false
	}
}

func validateDimensions(format ImageFormat, width, height int, options ValidationOptions) error {
	if width <= 0 || height <= 0 {
		return &ValidationError{Reason: ErrInvalidImage, Format: format, Width: width, Height: height}
	}
	if width > options.MaxWidth || height > options.MaxHeight {
		return &ValidationError{Reason: ErrDimensionsExceeded, Format: format, Width: width, Height: height}
	}
	pixels := safePixelCount(width, height)
	if pixels <= 0 || pixels > options.MaxPixelsPerTurn {
		return &ValidationError{Reason: ErrPixelLimitExceeded, Format: format, Width: width, Height: height}
	}
	return nil
}

func mimeTypeForFormat(format ImageFormat) string {
	switch format {
	case ImageFormatPNG:
		return MIMETypePNG
	case ImageFormatJPEG:
		return MIMETypeJPEG
	case ImageFormatGIF:
		return MIMETypeGIF
	case ImageFormatWebP:
		return MIMETypeWebP
	default:
		return ""
	}
}

func hashFile(ctx context.Context, file *os.File, maxFileBytes int64) (string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", wrapFilesystemError("rewinding image", err)
	}
	hasher := sha256.New()
	buffer := make([]byte, 32*1024)
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			size += int64(read)
			if size > maxFileBytes {
				return "", &ValidationError{Reason: ErrFileTooLarge}
			}
			if _, err := hasher.Write(buffer[:read]); err != nil {
				return "", fmt.Errorf("hashing image: %w", err)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return "", wrapFilesystemError("reading image", readErr)
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func decodeWebPHeader(data []byte) (int, int, error) {
	if len(data) < 20 || !bytes.Equal(data[:4], []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WEBP")) {
		return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: ImageFormatWebP}
	}
	riffSize := uint64(binary.LittleEndian.Uint32(data[4:8])) + 8
	if riffSize > uint64(len(data)) || riffSize < 20 {
		return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: ImageFormatWebP}
	}
	end := int(riffSize)
	offset := 12
	for offset+8 <= end {
		chunkType := data[offset : offset+4]
		chunkSize := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkStart := offset + 8
		chunkEnd := uint64(chunkStart) + chunkSize
		if chunkEnd > uint64(end) {
			return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: ImageFormatWebP}
		}
		chunk := data[chunkStart:int(chunkEnd)]
		switch string(chunkType) {
		case "VP8X":
			if len(chunk) < 10 {
				return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: ImageFormatWebP}
			}
			width := 1 + (uint32(chunk[4]) | uint32(chunk[5])<<8 | uint32(chunk[6])<<16)
			height := 1 + (uint32(chunk[7]) | uint32(chunk[8])<<8 | uint32(chunk[9])<<16)
			return checkedWebPDimensions(width, height)
		case "VP8 ":
			if len(chunk) < 10 || !bytes.Equal(chunk[3:6], []byte{0x9d, 0x01, 0x2a}) {
				return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: ImageFormatWebP}
			}
			width := uint32(binary.LittleEndian.Uint16(chunk[6:8]) & 0x3fff)
			height := uint32(binary.LittleEndian.Uint16(chunk[8:10]) & 0x3fff)
			return checkedWebPDimensions(width, height)
		case "VP8L":
			if len(chunk) < 5 || chunk[0] != 0x2f {
				return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: ImageFormatWebP}
			}
			width := 1 + (uint32(chunk[1]) | uint32(chunk[2]&0x3f)<<8)
			height := 1 + (uint32(chunk[2]>>6) | uint32(chunk[3])<<2 | uint32(chunk[4]&0x0f)<<10)
			return checkedWebPDimensions(width, height)
		}
		next := int(chunkEnd)
		if chunkSize&1 != 0 {
			next++
		}
		if next > end {
			return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: ImageFormatWebP}
		}
		offset = next
	}
	return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: ImageFormatWebP}
}

func checkedWebPDimensions(width, height uint32) (int, int, error) {
	if width == 0 || height == 0 || uint64(width) > uint64(^uint(0)>>1) || uint64(height) > uint64(^uint(0)>>1) {
		return 0, 0, &ValidationError{Reason: ErrInvalidImage, Format: ImageFormatWebP}
	}
	return int(width), int(height), nil
}
