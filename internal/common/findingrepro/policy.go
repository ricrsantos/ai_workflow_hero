package findingrepro

import (
	"fmt"
	"sort"
	"strings"
)

// Policy is the per-cycle contract that decides which repro modes a validation
// report may use and how a command-mode repro is re-run. It is derived from
// `verification.repro` in workflow-config.yml plus project auto-detection, so a
// Go project keeps the strict `go test` gate while a project in another
// language can still emit deterministic findings.
type Policy struct {
	// DefaultMode is applied when a report omits repro.mode.
	DefaultMode string
	// Modes is the set of modes any stage may use.
	Modes map[string]bool
	// Command is the argv template re-run for command-mode repros. The tokens
	// {{package}} and {{test}} are replaced by the finding's repro identity.
	Command []string
	// EvidenceStages may always use evidence mode, even when Modes excludes it,
	// because their failures have no deterministic re-run (visual diffs).
	EvidenceStages []string
}

// DefaultGoPolicy is the fail-closed policy used when no cycle configuration is
// available. It preserves Hero's original Go-only contract.
func DefaultGoPolicy() Policy {
	return Policy{
		DefaultMode: ModeGoTest,
		Modes:       map[string]bool{ModeGoTest: true},
	}
}

// ModeAllowed reports whether mode may be used by sourceStage.
func (p Policy) ModeAllowed(mode, sourceStage string) bool {
	if mode == "" {
		return false
	}
	if p.Modes[mode] {
		return true
	}
	if mode == ModeEvidence {
		for _, s := range p.EvidenceStages {
			if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(sourceStage)) {
				return true
			}
		}
	}
	return false
}

// AllowedModesFor lists the modes sourceStage may use, sorted for stable diagnostics.
func (p Policy) AllowedModesFor(sourceStage string) []string {
	seen := map[string]struct{}{}
	for _, mode := range []string{ModeGoTest, ModeCommand, ModeEvidence} {
		if p.ModeAllowed(mode, sourceStage) {
			seen[mode] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for mode := range seen {
		out = append(out, mode)
	}
	sort.Strings(out)
	return out
}

// ResolveMode applies the default and validates the mode against sourceStage.
func (p Policy) ResolveMode(mode, sourceStage string) (string, error) {
	mode, err := CanonicalMode(mode)
	if err != nil {
		return "", err
	}
	if mode == "" {
		mode = p.DefaultMode
	}
	if mode == "" {
		mode = ModeGoTest
	}
	if !p.ModeAllowed(mode, sourceStage) {
		allowed := p.AllowedModesFor(sourceStage)
		if len(allowed) == 0 {
			return "", fmt.Errorf("repro mode %q is not enabled for this project; configure verification.repro in workflow-config.yml", mode)
		}
		return "", fmt.Errorf("repro mode %q is not enabled for %s; allowed: %s", mode, sourceStage, strings.Join(allowed, ", "))
	}
	if mode == ModeCommand && len(p.Command) == 0 {
		return "", fmt.Errorf("repro mode command requires verification.repro.command in workflow-config.yml")
	}
	return mode, nil
}

// CommandArgv renders the configured command template for one repro identity.
func (p Policy) CommandArgv(pkg, test string) ([]string, error) {
	if len(p.Command) == 0 {
		return nil, fmt.Errorf("verification.repro.command is not configured")
	}
	argv := make([]string, 0, len(p.Command))
	for _, arg := range p.Command {
		replaced := strings.ReplaceAll(arg, "{{package}}", pkg)
		replaced = strings.ReplaceAll(replaced, "{{test}}", test)
		if strings.TrimSpace(replaced) == "" {
			// A placeholder-only argument with an empty value is dropped so a
			// suite-wide repro does not pass an empty filter.
			continue
		}
		argv = append(argv, replaced)
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("verification.repro.command rendered an empty command")
	}
	return argv, nil
}

// ValidateSourceForMode validates repro.source for the given mode. Only
// go_test requires the source to declare the named test function; a
// command-mode source is an optional scaffold in the project's own language.
func ValidateSourceForMode(mode, source, test string) error {
	if mode == ModeGoTest {
		return ValidateSource(source, test)
	}
	return ValidateSourceContent(source)
}
