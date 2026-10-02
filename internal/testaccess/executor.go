package testaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MinimumPlaywrightVersion is the minimum compatible Playwright package
// version established during Planning.
const MinimumPlaywrightVersion = "1.63.0"

// AuthenticationFlow identifies the planned authentication mechanism.
type AuthenticationFlow string

const (
	AuthenticationNone    AuthenticationFlow = "none"
	AuthenticationForm    AuthenticationFlow = "form"
	AuthenticationMFA     AuthenticationFlow = "mfa"
	AuthenticationCAPTCHA AuthenticationFlow = "captcha"
	AuthenticationSSO     AuthenticationFlow = "sso"
)

// ValidationPurpose identifies why a browser execution is planned.
type ValidationPurpose string

const (
	PurposeRepeatableE2E  ValidationPurpose = "repeatable_e2e"
	PurposeBrowserControl ValidationPurpose = "browser_control"
)

// ExecutionMethod is selected during Planning. HTTP is included only so an
// invalid silent fallback can be reported as a blocker instead of accepted.
type ExecutionMethod string

const (
	MethodPlaywrightTestSuite  ExecutionMethod = "playwright_test_suite"
	MethodPlaywrightCLI        ExecutionMethod = "playwright_cli"
	MethodPlaywrightCLINoSkill ExecutionMethod = "playwright_cli_no_skill"
	MethodMCP                  ExecutionMethod = "mcp"
	MethodHTTP                 ExecutionMethod = "http"
)

// ReportMethod is the stable method label safe for reports and events.
type ReportMethod string

const (
	ReportMethodPlaywrightTest ReportMethod = "playwright_test"
	ReportMethodCLISkill       ReportMethod = "cli_skill"
	ReportMethodCLI            ReportMethod = "cli"
	ReportMethodMCP            ReportMethod = "mcp"
	ReportMethodHTTP           ReportMethod = "http"
	ReportMethodUnknown        ReportMethod = "unknown"
)

// AccessExpectation defines the planned result for a protected target.
type AccessExpectation string

const (
	AccessAllowed AccessExpectation = "allowed"
	AccessDenied  AccessExpectation = "denied"
)

// LoginFormRecipe contains only non-secret selectors and destinations supplied
// by Planning. Credential values are passed separately to the private session.
type LoginFormRecipe struct {
	EntryURL        string `json:"entry_url"`
	LoginLocator    string `json:"login_locator"`
	PasswordLocator string `json:"password_locator"`
	SubmitLocator   string `json:"submit_locator"`
}

// ProtectedTargetRecipe identifies the role check required after login.
type ProtectedTargetRecipe struct {
	URL            string            `json:"url"`
	ExpectedRole   string            `json:"expected_role"`
	ExpectedAccess AccessExpectation `json:"expected_access"`
}

// Recipe is the shared, non-secret contract for one browser preparation and
// validation attempt. Application-specific values must come from Planning.
// It intentionally excludes credentials, cookies and browser storage state.
type Recipe struct {
	Stage                     ValidationStage
	Environment               string
	BaseURL                   string
	ApprovedOrigins           []string
	Authentication            AuthenticationFlow
	Login                     LoginFormRecipe
	ProtectedTarget           ProtectedTargetRecipe
	UserID                    string
	UserProfile               string
	CoverageIDs               []string
	CoverageRequirement       string
	CoverageScreenJourney     string
	CoverageMandatory         bool
	CoverageExpectedResult    string
	EvidenceRequirements      []string
	Purpose                   ValidationPurpose
	Method                    ExecutionMethod
	ToolName                  string
	ToolVersion               string
	ToolVersionCommand        []string
	PlaywrightVersion         string
	ExistingPlaywrightSuite   bool
	OfficialCLISkillAvailable bool
	PersistentBrowserNeeded   bool
	VerifiedCLICapabilityGap  bool
	StartCommand              string
	ReadinessCommand          string
	E2ECommand                string
	ActionTimeout             time.Duration
	TestTimeout               time.Duration
	Fixtures                  []string
	EvidencePaths             []string
	UnrestrictedHarness       bool
}

func (Recipe) String() string   { return "testaccess.Recipe[redacted]" }
func (Recipe) GoString() string { return "testaccess.Recipe[redacted]" }
func (Recipe) MarshalJSON() ([]byte, error) {
	return []byte(`{"redacted":true}`), nil
}

// CredentialSource privately loads the selected account for this attempt.
// SafeStore implements this interface.
type CredentialSource interface {
	Snapshot(context.Context, string) (CredentialSnapshot, error)
}

// BrowserProvider probes the selected method and creates one fresh,
// isolated in-memory session per Execute call. Implementations must not install
// or upgrade tools during validation.
type BrowserProvider interface {
	Capabilities(context.Context, Recipe) (RunnerCapabilities, error)
	NewIsolatedSession(context.Context, Recipe) (BrowserSession, error)
}

// RunnerCapabilities reports verified runtime features, not claims inferred
// from a package name or global installation.
type RunnerCapabilities struct {
	ToolAvailable                 bool
	ToolName                      string
	ToolVersion                   string
	PlaywrightVersion             string
	Method                        ExecutionMethod
	MethodSupported               bool
	FreshIsolatedContext          bool
	SameContextLoginAndTests      bool
	PrivateCredentialChannel      bool
	ApprovedOriginsEnforced       bool
	AuthStateMemoryOnly           bool
	CredentialArtifactSuppression bool
}

// BrowserSession keeps login and tests in the same isolated in-memory context.
// There is intentionally no cookie/storage-state export or credential accessor.
type BrowserSession interface {
	// SetCredentialArtifactSuppression must suspend/resume screenshots, traces,
	// video, snapshots and raw login-response capture for this session.
	SetCredentialArtifactSuppression(context.Context, bool) error
	Login(context.Context, LoginFormRecipe, Account) (LoginOutcome, error)
	VerifyProtectedTarget(context.Context, ProtectedTargetRecipe) (AccessOutcome, error)
	Run(context.Context, Recipe) (RunOutcome, error)
	Close() error
}

// LoginOutcome is a sanitized observation from the private login operation.
type LoginOutcome struct {
	Authenticated  bool
	InvalidAccount bool
}

// AccessOutcome reports only authentication state, verified role and whether
// the protected target allowed access.
type AccessOutcome struct {
	Authenticated bool
	VerifiedRole  string
	TargetAllowed bool
}

// RunOutcome is a sanitized execution status; implementations must not return
// raw page content, provider messages, logs, commands or credential values.
type RunOutcome string

const (
	RunPassed  RunOutcome = "passed"
	RunFailed  RunOutcome = "failed"
	RunBlocked RunOutcome = "blocked"
)

// PreparationStatus is the safe result status visible to orchestration layers.
type PreparationStatus string

const (
	PreparationReady   PreparationStatus = "ready"
	PreparationBlocked PreparationStatus = "blocked"
	PreparationFailed  PreparationStatus = "failed"
)

// BlockReason is a stable, value-free reason for prerequisite blocks.
type BlockReason string

const (
	BlockInvalidRecipe        BlockReason = "invalid_recipe"
	BlockUnsupportedAuth      BlockReason = "unsupported_authentication"
	BlockMethodPreference     BlockReason = "method_preference"
	BlockToolUnavailable      BlockReason = "tool_unavailable"
	BlockToolVersion          BlockReason = "tool_version"
	BlockMethodUnavailable    BlockReason = "method_unavailable"
	BlockContextUnavailable   BlockReason = "context_unavailable"
	BlockCredentialsMissing   BlockReason = "credentials_unavailable"
	BlockUnsafeRecipe         BlockReason = "unsafe_recipe"
	BlockInvalidAccount       BlockReason = "invalid_account"
	BlockLoginUnverified      BlockReason = "login_unverified"
	BlockRoleUnverified       BlockReason = "role_unverified"
	BlockProtectedAccess      BlockReason = "protected_access_unverified"
	BlockExecutionUnavailable BlockReason = "execution_unavailable"
	BlockSessionCleanup       BlockReason = "session_cleanup"
	BlockArtifactSuppression  BlockReason = "artifact_suppression"
	BlockInterrupted          BlockReason = "interrupted"
)

// PreparationResult contains no credential values, raw browser output or
// runner errors. IDs, profile, outcomes and next action are safe summaries.
type PreparationResult struct {
	Status       PreparationStatus `json:"status"`
	UserID       string            `json:"user_id,omitempty"`
	Profile      string            `json:"profile,omitempty"`
	Method       ReportMethod      `json:"method,omitempty"`
	VerifiedRole string            `json:"verified_role,omitempty"`
	CoverageIDs  []string          `json:"coverage_ids,omitempty"`
	Outcome      string            `json:"outcome,omitempty"`
	Reason       BlockReason       `json:"reason,omitempty"`
	NextAction   string            `json:"next_action,omitempty"`
	Disclosure   string            `json:"disclosure,omitempty"`
}

func (r PreparationResult) String() string   { return "testaccess.PreparationResult[redacted]" }
func (r PreparationResult) GoString() string { return "testaccess.PreparationResult[redacted]" }

// Executor authenticates and runs one selected browser attempt without
// exposing credential values to the planned runner or result surfaces.
type Executor struct {
	credentials CredentialSource
	browser     BrowserProvider
	logger      *slog.Logger
}

// NewExecutor creates a deterministic test-access executor.
func NewExecutor(credentials CredentialSource, browser BrowserProvider, logger *slog.Logger) *Executor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Executor{credentials: credentials, browser: browser, logger: logger}
}

// Execute snapshots the selected account, creates a fresh isolated session,
// authenticates, verifies the protected role and runs tests in that same
// session. It never serializes credential values or exports auth state.
func (e *Executor) Execute(ctx context.Context, recipe Recipe) (result PreparationResult) {
	result = resultForRecipe(recipe)
	if ctx == nil {
		return e.block(result, BlockInterrupted, nextAction(BlockInterrupted))
	}
	if err := ctx.Err(); err != nil {
		return e.block(result, BlockInterrupted, "Retry explicitly after confirming the stage has remaining budget.")
	}
	if reason := recipeBlockReason(recipe); reason != "" {
		return e.block(result, reason, nextAction(reason))
	}
	if e == nil || e.browser == nil || (recipe.Authentication != AuthenticationNone && e.credentials == nil) {
		return e.block(result, BlockMethodUnavailable, "Configure the planned browser method and test-access source during Planning.")
	}

	var account Account
	if recipe.Authentication == AuthenticationForm {
		snapshot, err := e.credentials.Snapshot(ctx, recipe.UserID)
		if err != nil || ctx.Err() != nil {
			if ctx.Err() != nil {
				return e.block(result, BlockInterrupted, nextAction(BlockInterrupted))
			}
			return e.block(result, BlockCredentialsMissing, nextAction(BlockCredentialsMissing))
		}
		account = snapshot.Account()
		if recipeContainsCredential(recipe, account) {
			result.UserID = ""
			result.Profile = ""
			result.CoverageIDs = nil
			return e.block(result, BlockUnsafeRecipe, nextAction(BlockUnsafeRecipe))
		}
		if account.ID() != recipe.UserID || account.Profile() != recipe.UserProfile || ValidateRequiredAccount(account) != nil {
			return e.block(result, BlockInvalidAccount, nextAction(BlockInvalidAccount))
		}
	}
	// Publish identifiers only after the private account read establishes that
	// the recipe did not put credential values into supposedly safe metadata.
	result.UserID = recipe.UserID
	result.Profile = recipe.UserProfile
	result.CoverageIDs = append([]string(nil), recipe.CoverageIDs...)
	if e.logger != nil && recipe.UserID != "" {
		e.logger.Debug("test-access private account admitted", "user_id", result.UserID)
	}

	capabilities, err := e.browser.Capabilities(ctx, recipe)
	if err != nil || ctx.Err() != nil {
		if ctx.Err() != nil {
			return e.block(result, BlockInterrupted, nextAction(BlockInterrupted))
		}
		return e.block(result, BlockMethodUnavailable, nextAction(BlockMethodUnavailable))
	}
	if reason := capabilityBlockReason(recipe, capabilities); reason != "" {
		return e.block(result, reason, nextAction(reason))
	}

	session, err := e.browser.NewIsolatedSession(ctx, recipe)
	if err != nil || session == nil {
		if ctx.Err() != nil {
			return e.block(result, BlockInterrupted, nextAction(BlockInterrupted))
		}
		return e.block(result, BlockContextUnavailable, nextAction(BlockContextUnavailable))
	}
	closed := false
	defer func() {
		if !closed {
			if err := session.Close(); err != nil {
				result = e.block(result, BlockSessionCleanup, nextAction(BlockSessionCleanup))
			}
		}
	}()

	if recipe.Authentication == AuthenticationForm {
		if err := session.SetCredentialArtifactSuppression(ctx, true); err != nil {
			if ctx.Err() != nil {
				return e.block(result, BlockInterrupted, nextAction(BlockInterrupted))
			}
			return e.block(result, BlockArtifactSuppression, nextAction(BlockArtifactSuppression))
		}
		login, loginErr := session.Login(ctx, recipe.Login, account)
		resumeErr := session.SetCredentialArtifactSuppression(ctx, false)
		if resumeErr != nil {
			if ctx.Err() != nil {
				return e.block(result, BlockInterrupted, nextAction(BlockInterrupted))
			}
			return e.block(result, BlockArtifactSuppression, nextAction(BlockArtifactSuppression))
		}
		if loginErr != nil || ctx.Err() != nil {
			if ctx.Err() != nil {
				return e.block(result, BlockInterrupted, nextAction(BlockInterrupted))
			}
			return e.block(result, BlockLoginUnverified, nextAction(BlockLoginUnverified))
		}
		if login.InvalidAccount {
			return e.block(result, BlockInvalidAccount, nextAction(BlockInvalidAccount))
		}
		if !login.Authenticated {
			return e.block(result, BlockLoginUnverified, nextAction(BlockLoginUnverified))
		}
	}

	access, err := session.VerifyProtectedTarget(ctx, recipe.ProtectedTarget)
	if err != nil || ctx.Err() != nil {
		if ctx.Err() != nil {
			return e.block(result, BlockInterrupted, nextAction(BlockInterrupted))
		}
		return e.block(result, BlockRoleUnverified, nextAction(BlockRoleUnverified))
	}
	if access.Authenticated != (recipe.Authentication == AuthenticationForm) || access.VerifiedRole != recipe.ProtectedTarget.ExpectedRole {
		return e.block(result, BlockRoleUnverified, nextAction(BlockRoleUnverified))
	}
	if recipe.ProtectedTarget.ExpectedAccess == AccessAllowed && !access.TargetAllowed {
		return e.block(result, BlockProtectedAccess, nextAction(BlockProtectedAccess))
	}
	if recipe.ProtectedTarget.ExpectedAccess == AccessDenied && access.TargetAllowed {
		return e.block(result, BlockProtectedAccess, "Review the negative-authorization expectation and protected-target policy.")
	}

	run, err := session.Run(ctx, recipe)
	if err != nil || ctx.Err() != nil {
		if ctx.Err() != nil {
			return e.block(result, BlockInterrupted, nextAction(BlockInterrupted))
		}
		return e.block(result, BlockExecutionUnavailable, nextAction(BlockExecutionUnavailable))
	}
	if err := session.Close(); err != nil {
		closed = true
		return e.block(result, BlockSessionCleanup, nextAction(BlockSessionCleanup))
	}
	closed = true

	switch run {
	case RunPassed:
		result.Status = PreparationReady
		result.VerifiedRole = access.VerifiedRole
		if recipe.ProtectedTarget.ExpectedAccess == AccessDenied {
			result.Outcome = "expected_denial_and_tests_passed"
		} else if recipe.Authentication == AuthenticationNone {
			result.Outcome = "public_tests_passed"
		} else {
			result.Outcome = "authenticated_tests_passed"
		}
		return e.logResult(result)
	case RunFailed:
		result.Status = PreparationFailed
		result.VerifiedRole = access.VerifiedRole
		result.Outcome = "tests_failed"
		result.NextAction = "Review the sanitized validation result and evidence; do not infer an authentication defect without independent verification."
		return e.logResult(result)
	case RunBlocked:
		return e.block(result, BlockExecutionUnavailable, nextAction(BlockExecutionUnavailable))
	default:
		return e.block(result, BlockExecutionUnavailable, nextAction(BlockExecutionUnavailable))
	}
}

func resultForRecipe(recipe Recipe) PreparationResult {
	return PreparationResult{
		Status:     PreparationBlocked,
		Method:     safeReportMethod(recipe),
		Disclosure: disclosureFor(recipe),
	}
}

func safeReportMethod(recipe Recipe) ReportMethod {
	switch recipe.Method {
	case MethodPlaywrightTestSuite:
		return ReportMethodPlaywrightTest
	case MethodPlaywrightCLI:
		return ReportMethodCLISkill
	case MethodPlaywrightCLINoSkill:
		return ReportMethodCLI
	case MethodMCP:
		return ReportMethodMCP
	case MethodHTTP:
		return ReportMethodHTTP
	default:
		return ReportMethodUnknown
	}
}

func (e *Executor) block(result PreparationResult, reason BlockReason, action string) PreparationResult {
	result.Status = PreparationBlocked
	result.Reason = reason
	result.NextAction = action
	return e.logResult(result)
}

func (e *Executor) logResult(result PreparationResult) PreparationResult {
	if e == nil || e.logger == nil {
		return result
	}
	attrs := []any{"status", result.Status, "outcome", result.Outcome}
	if result.UserID != "" {
		attrs = append(attrs, "user_id", result.UserID)
	}
	if result.Profile != "" {
		attrs = append(attrs, "profile", result.Profile)
	}
	if result.Reason != "" {
		attrs = append(attrs, "reason", result.Reason)
	}
	if result.Status == PreparationFailed {
		e.logger.Error("test-access validation failed", attrs...)
	} else {
		e.logger.Info("test-access execution completed", attrs...)
	}
	return result
}

func recipeBlockReason(recipe Recipe) BlockReason {
	if recipe.Stage == StageQAEndToEnd && recipe.Purpose == PurposeRepeatableE2E && recipe.Method == MethodHTTP {
		return BlockMethodPreference
	}
	if recipe.Authentication != AuthenticationForm && recipe.Authentication != AuthenticationNone {
		return BlockUnsupportedAuth
	}
	if err := validateRecipe(recipe); err != nil {
		return BlockInvalidRecipe
	}
	if recipe.PlaywrightVersion != "" {
		plannedPlaywright, err := parseVersion(recipe.PlaywrightVersion)
		if err != nil || compareVersion(plannedPlaywright, minimumVersion()) < 0 {
			return BlockToolVersion
		}
	}
	if recipe.Method == MethodHTTP {
		return BlockMethodPreference
	}
	if recipe.Purpose == PurposeRepeatableE2E {
		if recipe.ExistingPlaywrightSuite {
			if recipe.Method != MethodPlaywrightTestSuite {
				return BlockMethodPreference
			}
			return ""
		}
		if recipe.Method == MethodPlaywrightTestSuite {
			return BlockMethodPreference
		}
	}
	if recipe.Purpose != PurposeBrowserControl && recipe.Purpose != PurposeRepeatableE2E {
		return BlockInvalidRecipe
	}
	switch recipe.Method {
	case MethodPlaywrightTestSuite:
		return BlockMethodPreference
	case MethodPlaywrightCLI:
		if !recipe.OfficialCLISkillAvailable {
			return BlockMethodPreference
		}
	case MethodPlaywrightCLINoSkill:
		if recipe.OfficialCLISkillAvailable {
			return BlockMethodPreference
		}
	case MethodMCP:
		if !recipe.PersistentBrowserNeeded && !recipe.VerifiedCLICapabilityGap {
			return BlockMethodPreference
		}
	default:
		return BlockMethodPreference
	}
	return ""
}

func validateRecipe(recipe Recipe) error {
	fields := map[string]string{
		"environment":       recipe.Environment,
		"tool name":         recipe.ToolName,
		"expected role":     recipe.ProtectedTarget.ExpectedRole,
		"start command":     recipe.StartCommand,
		"readiness command": recipe.ReadinessCommand,
	}
	for name, value := range fields {
		if strings.TrimSpace(value) == "" || hasControl(value) {
			return fmt.Errorf("invalid %s", name)
		}
	}
	if strings.TrimSpace(recipe.ToolVersion) == "" || hasControl(recipe.ToolVersion) {
		return errors.New("invalid tool version")
	}
	if _, err := parseVersion(recipe.ToolVersion); err != nil {
		return errors.New("invalid planned tool version")
	}
	if err := validateToolVersionCommand(recipe.ToolName, recipe.ToolVersionCommand); err != nil {
		return errors.New("invalid planned tool version command")
	}
	if recipe.PlaywrightVersion != "" {
		if _, err := parseVersion(recipe.PlaywrightVersion); err != nil {
			return errors.New("invalid planned Playwright version")
		}
	}
	switch recipe.Authentication {
	case AuthenticationForm:
		for name, value := range map[string]string{
			"user ID":          recipe.UserID,
			"user profile":     recipe.UserProfile,
			"login locator":    recipe.Login.LoginLocator,
			"password locator": recipe.Login.PasswordLocator,
			"submit locator":   recipe.Login.SubmitLocator,
		} {
			if strings.TrimSpace(value) == "" || hasControl(value) {
				return fmt.Errorf("invalid %s", name)
			}
		}
		if _, err := ValidateUserID(recipe.UserID); err != nil {
			return errors.New("invalid selected user ID")
		}
		if recipe.UserProfile != recipe.ProtectedTarget.ExpectedRole {
			return errors.New("selected profile and expected role must match")
		}
	case AuthenticationNone:
		if recipe.UserID != "" || recipe.Login != (LoginFormRecipe{}) {
			return errors.New("public browser execution cannot include a user or login recipe")
		}
		if recipe.UserProfile == "" || recipe.UserProfile != recipe.ProtectedTarget.ExpectedRole {
			return errors.New("public browser execution must declare its anonymous role")
		}
	default:
		return errors.New("authentication flow is unsupported")
	}
	if recipe.Purpose == PurposeRepeatableE2E && strings.TrimSpace(recipe.E2ECommand) == "" {
		return errors.New("repeatable E2E requires a planned command")
	}
	if recipe.ActionTimeout <= 0 || recipe.TestTimeout <= 0 {
		return errors.New("action and test timeouts must be positive")
	}
	if recipe.ProtectedTarget.ExpectedAccess != AccessAllowed && recipe.ProtectedTarget.ExpectedAccess != AccessDenied {
		return errors.New("protected target access expectation is invalid")
	}
	if _, err := parseVersion(MinimumPlaywrightVersion); err != nil {
		return errors.New("invalid Playwright minimum version")
	}
	allowed := make(map[string]struct{}, len(recipe.ApprovedOrigins))
	for _, raw := range recipe.ApprovedOrigins {
		origin, err := normalizeOrigin(raw, true)
		if err != nil {
			return errors.New("invalid approved origin")
		}
		if _, exists := allowed[origin]; exists {
			return errors.New("duplicate approved origin")
		}
		allowed[origin] = struct{}{}
	}
	if len(allowed) == 0 {
		return errors.New("at least one approved origin is required")
	}
	destinations := []string{recipe.BaseURL, recipe.ProtectedTarget.URL}
	if recipe.Authentication == AuthenticationForm {
		destinations = append(destinations, recipe.Login.EntryURL)
	}
	for _, raw := range destinations {
		origin, err := normalizeOrigin(raw, false)
		if err != nil {
			return errors.New("invalid planned browser destination")
		}
		if _, ok := allowed[origin]; !ok {
			return errors.New("planned browser destination is outside approved origins")
		}
	}
	if len(recipe.CoverageIDs) == 0 {
		return errors.New("at least one affected coverage ID is required")
	}
	seen := make(map[string]struct{}, len(recipe.CoverageIDs))
	for _, id := range recipe.CoverageIDs {
		if strings.TrimSpace(id) == "" || hasControl(id) {
			return errors.New("invalid coverage ID")
		}
		if _, ok := seen[id]; ok {
			return errors.New("duplicate coverage ID")
		}
		seen[id] = struct{}{}
	}
	for _, item := range append(append([]string(nil), recipe.Fixtures...), recipe.EvidencePaths...) {
		if strings.TrimSpace(item) == "" || hasControl(item) {
			return errors.New("invalid fixture or evidence reference")
		}
	}
	return nil
}

func capabilityBlockReason(recipe Recipe, capabilities RunnerCapabilities) BlockReason {
	if !capabilities.ToolAvailable {
		return BlockToolUnavailable
	}
	if !strings.EqualFold(strings.TrimSpace(capabilities.ToolName), strings.TrimSpace(recipe.ToolName)) || capabilities.Method != recipe.Method || !capabilities.MethodSupported {
		return BlockMethodUnavailable
	}
	playwright, err := parseVersion(capabilities.PlaywrightVersion)
	if err != nil || compareVersion(playwright, minimumVersion()) < 0 {
		return BlockToolVersion
	}
	actual, err := parseVersion(capabilities.ToolVersion)
	if err != nil {
		return BlockToolVersion
	}
	planned, err := parseVersion(recipe.ToolVersion)
	if err != nil || compareVersion(actual, planned) < 0 {
		return BlockToolVersion
	}
	if !capabilities.FreshIsolatedContext || !capabilities.SameContextLoginAndTests || !capabilities.ApprovedOriginsEnforced {
		return BlockMethodUnavailable
	}
	if recipe.Authentication == AuthenticationForm && (!capabilities.PrivateCredentialChannel || !capabilities.AuthStateMemoryOnly || !capabilities.CredentialArtifactSuppression) {
		return BlockMethodUnavailable
	}
	if recipe.PlaywrightVersion != "" {
		plannedPlaywright, err := parseVersion(recipe.PlaywrightVersion)
		if err != nil || compareVersion(playwright, plannedPlaywright) < 0 {
			return BlockToolVersion
		}
	}
	return ""
}

func recipeContainsCredential(recipe Recipe, account Account) bool {
	values := []string{
		recipe.Environment, recipe.BaseURL, recipe.Login.EntryURL,
		recipe.Login.LoginLocator, recipe.Login.PasswordLocator, recipe.Login.SubmitLocator,
		recipe.ProtectedTarget.URL, recipe.ProtectedTarget.ExpectedRole, recipe.UserID,
		recipe.UserProfile, recipe.ToolName, recipe.ToolVersion, recipe.PlaywrightVersion, recipe.StartCommand,
		recipe.ReadinessCommand, recipe.E2ECommand,
		recipe.CoverageRequirement, recipe.CoverageScreenJourney,
		recipe.CoverageExpectedResult,
	}
	values = append(values, recipe.ApprovedOrigins...)
	values = append(values, recipe.ToolVersionCommand...)
	values = append(values, recipe.CoverageIDs...)
	values = append(values, recipe.Fixtures...)
	values = append(values, recipe.EvidencePaths...)
	values = append(values, recipe.EvidenceRequirements...)
	login, password := account.Login(), account.Password()
	for _, value := range values {
		if (login != "" && strings.Contains(value, login)) || (password != "" && strings.Contains(value, password)) {
			return true
		}
	}
	return false
}

func disclosureFor(recipe Recipe) string {
	if !recipe.UnrestrictedHarness {
		return ""
	}
	return "The selected privileged harness can read local files; project-root credentials remain accessible to trusted local processes and are not sandboxed."
}

func nextAction(reason BlockReason) string {
	switch reason {
	case BlockUnsupportedAuth:
		return "Plan an approved test account using the supported login/password form flow; interactive MFA, CAPTCHA and SSO are not supported."
	case BlockMethodPreference:
		return "Update Planning to select an existing Playwright Test suite for repeatable E2E, Playwright CLI with its official skill (or skills-less mode when unavailable), or MCP only for persistent inspection or a verified CLI capability gap. HTTP cannot replace browser validation."
	case BlockToolUnavailable:
		return "Install or enable the planned browser tool outside QA, then run /hero-continue to recheck it. Hero will not install or upgrade tools during QA."
	case BlockToolVersion:
		return "Use a compatible Playwright version (at least 1.63.0) and update the planned tool version before /hero-continue."
	case BlockCredentialsMissing:
		return "Enable Config → Test users and configure the selected account in project-root .env.hero, then run /hero-continue."
	case BlockUnsafeRecipe:
		return "Remove credential values from the non-secret login recipe, commands and evidence references, then retry with the planned recipe."
	case BlockInvalidAccount:
		return "Verify the selected account ID, login, password and profile in project-root .env.hero, then run /hero-continue."
	case BlockLoginUnverified:
		return "Verify the selected account locally and confirm the planned form locators; correct configuration before retrying. No application defect is inferred."
	case BlockRoleUnverified, BlockProtectedAccess:
		return "Verify the selected account profile and protected-target expectation during Planning, then correct the local account or plan before retrying."
	case BlockExecutionUnavailable:
		return "Resolve the selected method's same-context execution prerequisite and run /hero-continue. Do not pass credentials to prompts or command arguments."
	case BlockSessionCleanup:
		return "Close the isolated browser session safely before starting another attempt; do not reuse or export authentication state."
	case BlockInterrupted:
		return "Retry explicitly after confirming the stage has remaining budget."
	case BlockContextUnavailable:
		return "Resolve the isolated in-memory browser-context prerequisite during Planning; do not export authentication state."
	case BlockArtifactSuppression:
		return "Enable reliable credential-artifact suppression for screenshots, traces, video, snapshots and raw login responses before retrying; do not continue with capture enabled."
	default:
		return "Correct the non-secret test plan and retry explicitly."
	}
}

func normalizeOrigin(raw string, originOnly bool) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return "", errors.New("invalid browser origin")
	}
	if originOnly && (u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "") {
		return "", errors.New("approved origin must not include a path or query")
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port != "" && (u.Scheme != "http" || port != "80") && (u.Scheme != "https" || port != "443") {
		host = netJoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return strings.ToLower(u.Scheme) + "://" + host, nil
}

func netJoinHostPort(host, port string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}

func parseVersion(value string) (version, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	main := strings.SplitN(value, "+", 2)[0]
	parts := strings.SplitN(main, "-", 2)
	numbers := strings.Split(parts[0], ".")
	if len(numbers) != 3 {
		return version{}, errors.New("invalid semantic version")
	}
	parsed := version{}
	values := []*int{&parsed.major, &parsed.minor, &parsed.patch}
	for i, part := range numbers {
		if part == "" {
			return version{}, errors.New("invalid semantic version")
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return version{}, errors.New("invalid semantic version")
			}
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return version{}, errors.New("invalid semantic version")
		}
		*values[i] = n
	}
	if len(parts) == 2 {
		parsed.prerelease = parts[1]
		if parsed.prerelease == "" {
			return version{}, errors.New("invalid semantic version")
		}
	}
	return parsed, nil
}

type version struct {
	major, minor, patch int
	prerelease          string
}

func minimumVersion() version {
	minimum, _ := parseVersion(MinimumPlaywrightVersion)
	return minimum
}

func compareVersion(left, right version) int {
	for _, pair := range [][2]int{{left.major, right.major}, {left.minor, right.minor}, {left.patch, right.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if left.prerelease == right.prerelease {
		return 0
	}
	if left.prerelease != "" && right.prerelease == "" {
		return -1
	}
	if left.prerelease == "" && right.prerelease != "" {
		return 1
	}
	if left.prerelease < right.prerelease {
		return -1
	}
	return 1
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// MarshalJSON is explicitly safe for orchestration, logging and persistence.
func (r PreparationResult) MarshalJSON() ([]byte, error) {
	type safeResult PreparationResult
	return json.Marshal(safeResult(r))
}
