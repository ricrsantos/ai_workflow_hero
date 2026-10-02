package screenshots

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/ricrsantos/ai_workflow_hero/internal/media"
)

const stagedReferencePrefix = ".workflow-hero/cycles/current/screenshots/.staging/"

// CaptureAttestation is the stage report's value-free claim that one staged
// image was taken after assertions and with credential artifacts suppressed.
type CaptureAttestation struct {
	StablePointVerified               bool
	SensitiveFieldsAndTokensMasked    bool
	CredentialFlowArtifactsSuppressed bool
}

// NewStagedFileCaptureSource binds a report reference to one stage attempt.
// The service still performs its normal image validation before publication.
func NewStagedFileCaptureSource(projectDir, reference, stageName string, attempt int, attestation CaptureAttestation) (CaptureSource, error) {
	if _, err := stagedFileParts(reference, stageName, attempt); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(projectDir) || filepath.Clean(projectDir) != projectDir || projectDir == string(filepath.Separator) {
		return nil, ErrUnsafeProjectPath
	}
	return stagedFileCaptureSource{projectDir: projectDir, reference: reference, stageName: stageName, attempt: attempt, attestation: attestation}, nil
}

// IsStagedEvidenceReference reports whether reference names a single image in
// the exact stage and attempt staging directory.
func IsStagedEvidenceReference(reference, stageName string, attempt int) bool {
	_, err := stagedFileParts(reference, stageName, attempt)
	return err == nil
}

// RemoveStagedEvidence removes only a validated staged image for one stage
// attempt. It never follows links or removes a directory.
func RemoveStagedEvidence(projectDir, reference, stageName string, attempt int) error {
	parts, err := stagedFileParts(reference, stageName, attempt)
	if err != nil {
		return err
	}
	roots, current, err := openStagedDirectories(projectDir, parts[:len(parts)-1])
	if err != nil {
		return err
	}
	defer closeRoots(roots)
	info, err := current.Lstat(parts[len(parts)-1])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return ErrReadyAssetUnsafe
	}
	if !safeStagedFile(info) {
		return ErrReadyAssetUnsafe
	}
	if err := current.Remove(parts[len(parts)-1]); err != nil {
		return ErrReadyAssetUnsafe
	}
	return nil
}

type stagedFileCaptureSource struct {
	projectDir  string
	reference   string
	stageName   string
	attempt     int
	attestation CaptureAttestation
}

func (s stagedFileCaptureSource) CaptureScreenshot(ctx context.Context, policy CapturePolicy, destination io.Writer) (CaptureReceipt, error) {
	if ctx == nil || destination == nil || ctx.Err() != nil {
		return CaptureReceipt{}, context.Canceled
	}
	if !policy.MaskSensitiveFieldsAndTokens || !policy.SuppressScreenshotsDuringCredentialSubmission ||
		!policy.SuppressTracesDuringCredentialSubmission || !policy.SuppressVideoDuringCredentialSubmission ||
		!policy.SuppressSnapshotsDuringCredentialSubmission || !policy.SuppressRawLoginResponsesDuringCredentialSubmit {
		return CaptureReceipt{OmissionReason: "unsafe_capability"}, nil
	}
	if !s.attestation.CredentialFlowArtifactsSuppressed {
		return CaptureReceipt{OmissionReason: "credential_phase"}, nil
	}
	if !s.attestation.StablePointVerified || !s.attestation.SensitiveFieldsAndTokensMasked {
		return CaptureReceipt{OmissionReason: "unsafe_capability"}, nil
	}
	parts, err := stagedFileParts(s.reference, s.stageName, s.attempt)
	if err != nil {
		return CaptureReceipt{}, err
	}
	roots, current, err := openStagedDirectories(s.projectDir, parts[:len(parts)-1])
	if err != nil {
		return CaptureReceipt{}, err
	}
	defer closeRoots(roots)
	name := parts[len(parts)-1]
	info, err := current.Lstat(name)
	if err != nil || !safeStagedFile(info) || info.Size() <= 0 || info.Size() > media.DefaultMaxFileBytes {
		return CaptureReceipt{}, ErrReadyAssetUnsafe
	}
	file, err := current.Open(name)
	if err != nil {
		return CaptureReceipt{}, ErrReadyAssetUnsafe
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return CaptureReceipt{}, ErrReadyAssetUnsafe
	}
	written, copyErr := io.Copy(destination, io.LimitReader(file, media.DefaultMaxFileBytes+1))
	if copyErr != nil || written != info.Size() || written > media.DefaultMaxFileBytes {
		return CaptureReceipt{}, ErrReadyAssetUnsafe
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(opened, after) || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return CaptureReceipt{}, ErrReadyAssetUnsafe
	}
	return CaptureReceipt{
		StablePointVerified:               s.attestation.StablePointVerified,
		SensitiveFieldsAndTokensMasked:    s.attestation.SensitiveFieldsAndTokensMasked,
		CredentialFlowArtifactsSuppressed: s.attestation.CredentialFlowArtifactsSuppressed,
	}, nil
}

func stagedFileParts(reference, stageName string, attempt int) ([]string, error) {
	if (stageName != stageBrowserUI && stageName != stageE2E) || attempt <= 0 ||
		strings.ContainsAny(reference, "\\\x00\r\n$`") || path.Clean(reference) != reference || !strings.HasPrefix(reference, stagedReferencePrefix) {
		return nil, ErrInvalidCaptureRequest
	}
	relative := strings.TrimPrefix(reference, stagedReferencePrefix)
	parts := strings.Split(relative, "/")
	if len(parts) != 3 || parts[0] != stageName || parts[1] != strconv.Itoa(attempt) || !metadataPattern.MatchString(parts[2]) {
		return nil, ErrInvalidCaptureRequest
	}
	extension := strings.ToLower(filepath.Ext(parts[2]))
	switch extension {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
	default:
		return nil, ErrInvalidCaptureRequest
	}
	return []string{".workflow-hero", "cycles", "current", "screenshots", ".staging", stageName, strconv.Itoa(attempt), parts[2]}, nil
}

func openStagedDirectories(projectDir string, components []string) ([]*os.Root, *os.Root, error) {
	projectInfo, err := os.Lstat(projectDir)
	if err != nil || !projectInfo.IsDir() || projectInfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, ErrUnsafeProjectPath
	}
	root, err := os.OpenRoot(projectDir)
	if err != nil {
		return nil, nil, ErrUnsafeProjectPath
	}
	roots := []*os.Root{root}
	current := root
	for index, component := range components {
		info, err := current.Lstat(component)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !safeStagedDirectory(info, index) {
			closeRoots(roots)
			return nil, nil, ErrReadyAssetUnsafe
		}
		child, err := current.OpenRoot(component)
		if err != nil {
			closeRoots(roots)
			return nil, nil, ErrReadyAssetUnsafe
		}
		opened, err := child.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			_ = child.Close()
			closeRoots(roots)
			return nil, nil, ErrReadyAssetUnsafe
		}
		roots = append(roots, child)
		current = child
	}
	return roots, current, nil
}

func safeStagedDirectory(info os.FileInfo, index int) bool {
	if info == nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) {
		return false
	}
	// The project metadata ancestors may retain conventional 0755 modes, but
	// no ancestor may be writable by another user. Screenshot/staging leaves
	// and attempts must be private because agents create files beneath them.
	if index < 3 {
		return info.Mode().Perm()&0o022 == 0
	}
	return info.Mode().Perm()&0o077 == 0
}

func safeStagedFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Getuid()) && stat.Nlink == 1
}

func closeRoots(roots []*os.Root) {
	for i := len(roots) - 1; i >= 0; i-- {
		_ = roots[i].Close()
	}
}
