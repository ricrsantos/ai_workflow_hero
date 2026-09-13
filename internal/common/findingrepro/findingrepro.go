// Package findingrepro canonicalizes the repro identity attached to a
// validation finding. Reports and the store share these rules so a decoder
// rejection and a persist rejection mean the same thing. A repro is either an
// automated re-run (a Go test, or a project-configured command) or an
// evidence-only record for failures no deterministic re-run can express.
package findingrepro

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxSourceBytes is the fail-closed ceiling for repro.source in a report.
const MaxSourceBytes = 64 * 1024

var (
	packageRE = regexp.MustCompile(`^\./[A-Za-z0-9][A-Za-z0-9._/\-]*$`)
	testRE    = regexp.MustCompile(`^Test[A-Za-z0-9_]+$`)
)

// CanonicalPackage slash-normalizes a relative Go package path for `go test`.
// Empty input stays empty. Non-empty values always start with "./".
func CanonicalPackage(pkg string) (string, error) {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return "", nil
	}
	if filepath.IsAbs(pkg) || strings.HasPrefix(pkg, "/") || (len(pkg) >= 2 && pkg[1] == ':') {
		return "", fmt.Errorf("repro package %q is not a relative Go package path", pkg)
	}
	pkg = filepath.ToSlash(pkg)
	pkg = strings.TrimSuffix(pkg, "/")
	for strings.HasPrefix(pkg, "./") {
		pkg = strings.TrimPrefix(pkg, "./")
	}
	if pkg == "" || pkg == "." || strings.Contains(pkg, "..") {
		return "", fmt.Errorf("repro package %q is not a relative Go package path", pkg)
	}
	pkg = "./" + pkg
	if !packageRE.MatchString(pkg) {
		return "", fmt.Errorf("repro package %q is not a relative Go package path", pkg)
	}
	return pkg, nil
}

// CanonicalTest accepts a Go test function name (Test prefix, no regexp).
func CanonicalTest(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	if !testRE.MatchString(name) {
		return "", fmt.Errorf("repro test %q is not a Go Test* function name", name)
	}
	return name, nil
}

// SourceDeclaresTest reports whether source contains `func TestName(`.
func SourceDeclaresTest(source, test string) bool {
	test = strings.TrimSpace(test)
	if test == "" {
		return false
	}
	return strings.Contains(source, "func "+test+"(")
}

// ValidateSource checks size, encoding, and that source declares test.
func ValidateSource(source, test string) error {
	if err := ValidateSourceContent(source); err != nil {
		return err
	}
	if !SourceDeclaresTest(strings.TrimSpace(source), test) {
		return fmt.Errorf("repro source must declare func %s(", test)
	}
	return nil
}

// ValidateSourceContent checks size, encoding, and hygiene of a repro source
// without requiring a Go test declaration.
func ValidateSourceContent(source string) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return fmt.Errorf("repro source is required")
	}
	if !utf8.ValidString(source) {
		return fmt.Errorf("repro source is not valid UTF-8")
	}
	if len(source) > MaxSourceBytes {
		return fmt.Errorf("repro source exceeds %d bytes", MaxSourceBytes)
	}
	if strings.Contains(source, "\x00") {
		return fmt.Errorf("repro source contains invalid characters")
	}
	lower := strings.ToLower(source)
	if strings.Contains(lower, "data:") {
		return fmt.Errorf("repro source must not embed data URLs")
	}
	return nil
}

// Repro modes. A finding's repro identity is either an automated re-run
// (go_test / command) or an evidence-only record for failures that no
// deterministic re-run can express (visual diffs, coverage ratios, lint).
const (
	ModeGoTest   = "go_test"
	ModeCommand  = "command"
	ModeEvidence = "evidence"
)

var (
	commandTargetRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/\- ]*$`)
	commandTestRE   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9 _.\-/:#\[\]]*$`)
)

// MaxCommandTokenBytes caps the agent-supplied tokens interpolated into a
// project-configured repro command.
const MaxCommandTokenBytes = 160

// CanonicalMode normalizes a repro mode. Empty input returns empty so callers
// can apply their own default.
func CanonicalMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "":
		return "", nil
	case ModeGoTest, ModeCommand, ModeEvidence:
		return mode, nil
	default:
		return "", fmt.Errorf("repro mode %q is not one of go_test, command, evidence", mode)
	}
}

// CanonicalCommandTarget normalizes the repo-relative target a command-mode
// repro points at (a file, directory, or suite). Empty input stays empty.
func CanonicalCommandTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", nil
	}
	if len(target) > MaxCommandTokenBytes {
		return "", fmt.Errorf("repro package exceeds %d bytes", MaxCommandTokenBytes)
	}
	if filepath.IsAbs(target) || strings.HasPrefix(target, "/") || (len(target) >= 2 && target[1] == ':') {
		return "", fmt.Errorf("repro package %q must be repo-relative", target)
	}
	target = filepath.ToSlash(target)
	for strings.HasPrefix(target, "./") {
		target = strings.TrimPrefix(target, "./")
	}
	target = strings.TrimSuffix(target, "/")
	if target == "" || target == "." || strings.Contains(target, "..") {
		return "", fmt.Errorf("repro package %q must be repo-relative", target)
	}
	if !commandTargetRE.MatchString(target) {
		return "", fmt.Errorf("repro package %q contains characters that are not allowed in a repo-relative target", target)
	}
	return target, nil
}

// CanonicalCommandTest normalizes the filter token a command-mode repro passes
// to the project's configured test command. Empty input stays empty.
func CanonicalCommandTest(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	if len(name) > MaxCommandTokenBytes {
		return "", fmt.Errorf("repro test exceeds %d bytes", MaxCommandTokenBytes)
	}
	if !commandTestRE.MatchString(name) {
		return "", fmt.Errorf("repro test %q contains characters that are not allowed in a test filter", name)
	}
	return name, nil
}

// CanonicalIdentity normalizes package and test for the given mode.
func CanonicalIdentity(mode, pkg, test string) (string, string, error) {
	switch mode {
	case ModeCommand:
		cpkg, err := CanonicalCommandTarget(pkg)
		if err != nil {
			return "", "", err
		}
		ctest, err := CanonicalCommandTest(test)
		if err != nil {
			return "", "", err
		}
		return cpkg, ctest, nil
	case ModeEvidence:
		return "", "", nil
	default:
		cpkg, err := CanonicalPackage(pkg)
		if err != nil {
			return "", "", err
		}
		ctest, err := CanonicalTest(test)
		if err != nil {
			return "", "", err
		}
		return cpkg, ctest, nil
	}
}
