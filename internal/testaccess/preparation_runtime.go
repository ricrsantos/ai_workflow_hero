package testaccess

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	preparationProbeTimeout = 5 * time.Second
	maxVersionOutput        = 4 << 10
)

var observedVersionPattern = regexp.MustCompile(`(?i)(?:^|[^0-9])v?([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)`)

// PreparationHarness is the selected harness descriptor used for a bounded
// availability probe. The live browser permission and same-context capability
// remain the responsibility of the dispatched stage session.
type PreparationHarness interface {
	Name() string
	IsAvailable(context.Context) error
}

// PreparationHTTPClient and PreparationCommandRunner are narrow seams for
// deterministic tests. Production probes use a bounded HTTP client and fixed
// argv execution without a shell.
type PreparationHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type PreparationCommandRunner interface {
	Run(context.Context, string, []string) ([]byte, error)
}

// RuntimePreparationCapability implements local, pre-dispatch checks. It never
// creates a browser, installs a tool, provisions fixtures, or exports account
// values. Credentials remain behind CredentialSource/SafeStore.
type RuntimePreparationCapability struct {
	ProjectDir  string
	HarnessID   string
	Harness     PreparationHarness
	Credentials CredentialSource
	HTTPClient  PreparationHTTPClient
	Commands    PreparationCommandRunner
	Logger      *slog.Logger
	Plan        *BrowserPlan
}

func (c *RuntimePreparationCapability) CheckPrerequisite(ctx context.Context, recipe Recipe, prerequisite PreparationPrerequisite) PrerequisiteObservation {
	if ctx == nil || ctx.Err() != nil {
		return PrerequisiteObservation{Reason: PreparationReasonInterrupted}
	}
	switch prerequisite {
	case PrerequisiteServiceReadiness:
		return c.checkReadiness(ctx, recipe)
	case PrerequisiteBrowserPermission:
		return c.checkHarnessAvailability(ctx)
	case PrerequisiteSelectedAccount:
		return c.checkSelectedAccounts(ctx, recipe)
	case PrerequisiteFixtures:
		return c.checkFixtures(recipe)
	default:
		return PrerequisiteObservation{Reason: PreparationReasonCheckFailed}
	}
}

func (c *RuntimePreparationCapability) AdmitPlannedMethod(ctx context.Context, recipe Recipe) PrerequisiteObservation {
	if c == nil || ctx == nil || ctx.Err() != nil {
		return PrerequisiteObservation{Reason: PreparationReasonInterrupted}
	}
	if recipe.Method == MethodHTTP || strings.TrimSpace(recipe.ToolName) == "" || len(recipe.ToolVersionCommand) != 2 {
		return PrerequisiteObservation{Reason: PreparationReasonMethodUnavailable}
	}
	runner := c.Commands
	if runner == nil {
		runner = boundedPreparationCommandRunner{}
	}
	probeCtx, cancel := context.WithTimeout(ctx, preparationProbeTimeout)
	defer cancel()
	output, err := runner.Run(probeCtx, c.ProjectDir, append([]string(nil), recipe.ToolVersionCommand...))
	if err != nil {
		return PrerequisiteObservation{Reason: PreparationReasonToolUnavailable}
	}
	version := observedVersion(output)
	if version == "" {
		return PrerequisiteObservation{Reason: PreparationReasonToolVersion}
	}
	capabilities := RunnerCapabilities{
		ToolAvailable:     true,
		ToolName:          recipe.ToolName,
		ToolVersion:       version,
		PlaywrightVersion: version,
		Method:            recipe.Method,
		MethodSupported:   true,
	}
	return PrerequisiteObservation{Ready: true, Capabilities: &capabilities}
}

func (c *RuntimePreparationCapability) checkReadiness(ctx context.Context, recipe Recipe) PrerequisiteObservation {
	if c == nil || strings.TrimSpace(recipe.BaseURL) == "" {
		return PrerequisiteObservation{Reason: PreparationReasonServiceUnavailable}
	}
	client := c.HTTPClient
	if client == nil {
		client = boundedPreparationHTTPClient()
	}
	probeCtx, cancel := context.WithTimeout(ctx, preparationProbeTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, recipe.BaseURL, nil)
	if err != nil {
		return PrerequisiteObservation{Reason: PreparationReasonServiceUnavailable}
	}
	response, err := client.Do(request)
	if err != nil {
		return PrerequisiteObservation{Reason: PreparationReasonServiceUnavailable, Retryable: probeCtx.Err() == nil}
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Debug("test-access readiness response close failed")
		}
	}()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
	if response.StatusCode >= http.StatusInternalServerError {
		return PrerequisiteObservation{Reason: PreparationReasonServiceUnavailable, Retryable: true}
	}
	return PrerequisiteObservation{Ready: true}
}

func (c *RuntimePreparationCapability) checkHarnessAvailability(ctx context.Context) PrerequisiteObservation {
	if c == nil || c.Harness == nil || strings.TrimSpace(c.HarnessID) == "" || strings.TrimSpace(c.Harness.Name()) == "" {
		return PrerequisiteObservation{Reason: PreparationReasonBrowserPermission}
	}
	probeCtx, cancel := context.WithTimeout(ctx, preparationProbeTimeout)
	defer cancel()
	if err := c.Harness.IsAvailable(probeCtx); err != nil {
		return PrerequisiteObservation{Reason: PreparationReasonBrowserPermission, Retryable: probeCtx.Err() == nil}
	}
	return PrerequisiteObservation{Ready: true}
}

func (c *RuntimePreparationCapability) checkSelectedAccounts(ctx context.Context, recipe Recipe) PrerequisiteObservation {
	if recipe.Authentication == AuthenticationNone {
		return PrerequisiteObservation{Ready: true}
	}
	if c == nil || !filepath.IsAbs(c.ProjectDir) || filepath.Clean(c.ProjectDir) != c.ProjectDir {
		return PrerequisiteObservation{Reason: PreparationReasonSelectedAccount}
	}
	plan := c.Plan
	if plan == nil {
		loaded, err := LoadBrowserPlan(c.ProjectDir)
		if err != nil {
			return PrerequisiteObservation{Reason: PreparationReasonSelectedAccount}
		}
		plan = &loaded
	}
	users := make([]string, 0, len(plan.Coverage))
	seen := make(map[string]struct{}, len(plan.Coverage))
	for _, item := range plan.Coverage {
		user := strings.TrimSpace(item.UserID)
		if user == "" {
			continue
		}
		if _, exists := seen[user]; exists {
			continue
		}
		seen[user] = struct{}{}
		users = append(users, user)
	}
	if len(users) == 0 {
		return PrerequisiteObservation{Reason: PreparationReasonSelectedAccount}
	}

	credentials := c.Credentials
	var ownedStore *SafeStore
	if credentials == nil {
		opened, err := OpenSafeStore(c.ProjectDir, c.Logger)
		if err != nil {
			return PrerequisiteObservation{Reason: PreparationReasonSelectedAccount}
		}
		ownedStore = opened
		credentials = opened
	}
	if ownedStore != nil {
		defer func() {
			if err := ownedStore.Close(); err != nil && c.Logger != nil {
				c.Logger.Error("test-access preparation store close failed")
			}
		}()
	}
	for _, user := range users {
		if ctx.Err() != nil {
			return PrerequisiteObservation{Reason: PreparationReasonInterrupted}
		}
		if _, err := credentials.Snapshot(ctx, user); err != nil {
			return PrerequisiteObservation{Reason: PreparationReasonSelectedAccount}
		}
	}
	return PrerequisiteObservation{Ready: true}
}

func (c *RuntimePreparationCapability) checkFixtures(recipe Recipe) PrerequisiteObservation {
	if c == nil {
		return PrerequisiteObservation{Reason: PreparationReasonFixturesUnavailable}
	}
	for _, fixture := range recipe.Fixtures {
		if !looksLikeProjectPath(fixture) {
			continue // Label-only fixtures are confirmed by the stage session.
		}
		if !projectFixtureExists(c.ProjectDir, fixture) {
			return PrerequisiteObservation{Reason: PreparationReasonFixturesUnavailable}
		}
	}
	return PrerequisiteObservation{Ready: true}
}

func looksLikeProjectPath(value string) bool {
	value = strings.TrimSpace(value)
	return filepath.IsAbs(value) || strings.HasPrefix(value, ".") || strings.ContainsAny(value, `/\\`) || filepath.Ext(value) != ""
}

func projectFixtureExists(projectDir, fixture string) bool {
	if !filepath.IsAbs(projectDir) || filepath.Clean(projectDir) != projectDir || filepath.IsAbs(fixture) || strings.ContainsRune(fixture, '\\') {
		return false
	}
	rel := filepath.Clean(filepath.FromSlash(fixture))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	root, err := os.OpenRoot(projectDir)
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()
	current := root
	var nested []*os.Root
	defer func() {
		for i := len(nested) - 1; i >= 0; i-- {
			_ = nested[i].Close()
		}
	}()
	parts := strings.Split(rel, string(filepath.Separator))
	for index, component := range parts {
		if component == "" || component == "." || component == ".." {
			return false
		}
		info, err := current.Lstat(component)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if index == len(parts)-1 {
			return info.Mode().IsRegular() || info.IsDir()
		}
		if !info.IsDir() {
			return false
		}
		child, err := current.OpenRoot(component)
		if err != nil {
			return false
		}
		opened, err := child.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			_ = child.Close()
			return false
		}
		nested = append(nested, child)
		current = child
	}
	return false
}

func observedVersion(output []byte) string {
	match := observedVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return ""
	}
	if _, err := parseVersion(string(match[1])); err != nil {
		return ""
	}
	return string(match[1])
}

func boundedPreparationHTTPClient() *http.Client {
	return &http.Client{
		Timeout: preparationProbeTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type boundedPreparationCommandRunner struct{}

func (boundedPreparationCommandRunner) Run(ctx context.Context, projectDir string, argv []string) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil || len(argv) != 2 || strings.TrimSpace(argv[0]) == "" || argv[1] != "--version" {
		return nil, errors.New("invalid bounded version command")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1])
	cmd.Dir = projectDir
	var stdout limitedPreparationBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, errors.New("planned version command failed")
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}

type limitedPreparationBuffer struct{ bytes.Buffer }

func (b *limitedPreparationBuffer) Write(data []byte) (int, error) {
	if b.Len()+len(data) > maxVersionOutput {
		return 0, errors.New("planned version output exceeds the limit")
	}
	return b.Buffer.Write(data)
}
