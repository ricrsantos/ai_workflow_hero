package reprotest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/findingrepro"
)

const defaultTimeout = 2 * time.Minute

// Spec is the repro the scheduler re-runs before accepting find-* done. Mode
// decides how: a Go test, or the project-configured command rendered into Argv.
type Spec struct {
	Mode    string
	Package string
	Test    string
	Argv    []string
}

// ErrNotRun means go test did not execute the named test (missing file or filter miss).
var ErrNotRun = errors.New("repro test did not run")

type event struct {
	Action string `json:"Action"`
	Test   string `json:"Test"`
	Output string `json:"Output"`
}

// Run re-runs one repro in projectRoot. A compile failure, a failed test, a
// missing test, or a non-zero command exit is an error.
func Run(ctx context.Context, projectRoot string, spec Spec) error {
	if strings.TrimSpace(projectRoot) == "" {
		return errors.New("project root is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}
	switch spec.Mode {
	case findingrepro.ModeEvidence:
		return nil
	case findingrepro.ModeCommand:
		return runCommand(ctx, projectRoot, spec)
	default:
		return runGoTest(ctx, projectRoot, spec)
	}
}

// runCommand executes the project-configured repro command. The argv comes
// from workflow-config.yml; only the package and test tokens are agent-supplied
// and both are charset-restricted, and no shell is involved.
func runCommand(ctx context.Context, projectRoot string, spec Spec) error {
	if len(spec.Argv) == 0 {
		return errors.New("repro command is not configured")
	}
	cmd := exec.CommandContext(ctx, spec.Argv[0], spec.Argv[1:]...)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(lastLines(string(out), 20))
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("repro command %s failed: %s", strings.Join(spec.Argv, " "), detail)
}

func runGoTest(ctx context.Context, projectRoot string, spec Spec) error {
	pkg, err := findingrepro.CanonicalPackage(spec.Package)
	if err != nil || pkg == "" {
		return fmt.Errorf("invalid repro package %q", spec.Package)
	}
	test, err := findingrepro.CanonicalTest(spec.Test)
	if err != nil || test == "" {
		return fmt.Errorf("invalid repro test %q", spec.Test)
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-json", "-count=1", "-run", "^"+test+"$", pkg)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	ran := false
	failed := false
	var failOut strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var ev event
		if json.Unmarshal(line, &ev) != nil {
			continue
		}
		if ev.Test != test {
			if ev.Action == "fail" && ev.Test == "" {
				failed = true
				failOut.WriteString(ev.Output)
			}
			continue
		}
		switch ev.Action {
		case "run":
			ran = true
		case "fail":
			ran = true
			failed = true
			failOut.WriteString(ev.Output)
		case "pass":
			ran = true
		}
	}
	if !ran {
		if err != nil {
			return fmt.Errorf("%w: %s", ErrNotRun, firstLine(string(out)))
		}
		return fmt.Errorf("%w: go test -run ^%s$ %s matched no tests", ErrNotRun, test, pkg)
	}
	if failed {
		detail := strings.TrimSpace(failOut.String())
		if detail == "" {
			detail = firstLine(string(out))
		}
		if err != nil && detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("repro test %s %s failed: %s", pkg, test, detail)
	}
	return nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// lastLines returns at most n trailing non-empty lines of s.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	kept := make([]string, 0, n)
	for i := len(lines) - 1; i >= 0 && len(kept) < n; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		kept = append([]string{lines[i]}, kept...)
	}
	return strings.Join(kept, "\n")
}
