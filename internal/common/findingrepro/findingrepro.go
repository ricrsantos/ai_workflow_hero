// Package findingrepro canonicalizes the Go repro identity attached to a
// validation finding. Reports and the store share these rules so a decoder
// rejection and a persist rejection mean the same thing.
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
	if !SourceDeclaresTest(source, test) {
		return fmt.Errorf("repro source must declare func %s(", test)
	}
	return nil
}
