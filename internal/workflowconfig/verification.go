package workflowconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
)

// DefaultEvidenceStages are the validation stages whose failures may always be
// recorded as evidence-only findings. A rendering or visual-diff failure has no
// deterministic unit-test re-run, so requiring one would only force the agent
// to invent a test that proves nothing.
var DefaultEvidenceStages = []string{"browser_ui_validation"}

// ReproVerification is the `verification.repro` block of workflow-config.yml.
// The block is optional and is not owned by the TUI Config screen, so it is
// read directly instead of through ManagedConfig.
type ReproVerification struct {
	Mode           string   `yaml:"mode"`
	Command        []string `yaml:"command"`
	AllowEvidence  *bool    `yaml:"allow_evidence"`
	EvidenceStages []string `yaml:"evidence_stages"`
}

// Verification is the `verification` block of workflow-config.yml.
type Verification struct {
	Repro ReproVerification `yaml:"repro"`
}

type verificationRoot struct {
	Verification Verification `yaml:"verification"`
}

// ParseVerification decodes the optional verification block from raw YAML.
func ParseVerification(raw []byte) (Verification, error) {
	var root verificationRoot
	if len(raw) == 0 {
		return Verification{}, nil
	}
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return Verification{}, fmt.Errorf("parse verification block: %w", err)
	}
	return root.Verification, nil
}

// LoadVerification reads the active cycle's verification block. A missing file
// is not an error: the caller falls back to auto-detection.
func LoadVerification(projectDir string) (Verification, error) {
	path := filepath.Join(projectDir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Verification{}, nil
		}
		return Verification{}, fmt.Errorf("read workflow-config.yml: %w", err)
	}
	return ParseVerification(raw)
}

// BuildReproPolicy resolves the modes a cycle may use. hasGoModule reports
// whether the project root holds a go.mod, which is what makes the built-in
// `go test` gate usable without any configuration.
func BuildReproPolicy(v Verification, hasGoModule bool) findingrepro.Policy {
	configured, _ := findingrepro.CanonicalMode(v.Repro.Mode)
	command := make([]string, 0, len(v.Repro.Command))
	for _, arg := range v.Repro.Command {
		if strings.TrimSpace(arg) != "" {
			command = append(command, arg)
		}
	}

	modes := map[string]bool{}
	if hasGoModule || configured == findingrepro.ModeGoTest {
		modes[findingrepro.ModeGoTest] = true
	}
	if len(command) > 0 {
		modes[findingrepro.ModeCommand] = true
	}

	evidenceStages := append([]string(nil), DefaultEvidenceStages...)
	if v.Repro.EvidenceStages != nil {
		evidenceStages = append([]string(nil), v.Repro.EvidenceStages...)
	}
	switch {
	case v.Repro.AllowEvidence != nil && *v.Repro.AllowEvidence:
		modes[findingrepro.ModeEvidence] = true
	case v.Repro.AllowEvidence != nil && !*v.Repro.AllowEvidence:
		// Explicitly disabled everywhere, including the per-stage allowance.
		evidenceStages = nil
	case !modes[findingrepro.ModeGoTest] && !modes[findingrepro.ModeCommand]:
		// No automated gate is available at all; evidence keeps findings flowing
		// instead of rejecting every validation report.
		modes[findingrepro.ModeEvidence] = true
	}

	policy := findingrepro.Policy{
		Modes:          modes,
		Command:        command,
		EvidenceStages: evidenceStages,
	}
	switch {
	case configured != "" && (modes[configured] || configured == findingrepro.ModeEvidence):
		policy.DefaultMode = configured
	case modes[findingrepro.ModeGoTest]:
		policy.DefaultMode = findingrepro.ModeGoTest
	case modes[findingrepro.ModeCommand]:
		policy.DefaultMode = findingrepro.ModeCommand
	default:
		policy.DefaultMode = findingrepro.ModeEvidence
	}
	return policy
}

// ProjectHasGoModule reports whether projectDir holds a go.mod file.
func ProjectHasGoModule(projectDir string) bool {
	if strings.TrimSpace(projectDir) == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(projectDir, "go.mod"))
	return err == nil && info.Mode().IsRegular()
}

// ReproPolicyForProject resolves the effective policy for a project directory,
// preferring an explicit `verification` block and falling back to detection.
// An unknown project directory keeps Hero's original Go-only contract instead
// of guessing from the process working directory.
func ReproPolicyForProject(projectDir string, snapshot []byte) findingrepro.Policy {
	v, err := ParseVerification(snapshot)
	if strings.TrimSpace(projectDir) == "" {
		if err != nil || !verificationConfigured(v) {
			return findingrepro.DefaultGoPolicy()
		}
		return BuildReproPolicy(v, true)
	}
	if err != nil || !verificationConfigured(v) {
		if loaded, lerr := LoadVerification(projectDir); lerr == nil {
			v = loaded
		}
	}
	return BuildReproPolicy(v, ProjectHasGoModule(projectDir))
}

func verificationConfigured(v Verification) bool {
	return strings.TrimSpace(v.Repro.Mode) != "" ||
		len(v.Repro.Command) > 0 ||
		v.Repro.AllowEvidence != nil ||
		v.Repro.EvidenceStages != nil
}
