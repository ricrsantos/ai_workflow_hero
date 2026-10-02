package testaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type executorCredentialSource struct {
	snapshot CredentialSnapshot
	err      error
	users    []string
}

func (s *executorCredentialSource) Snapshot(_ context.Context, user string) (CredentialSnapshot, error) {
	s.users = append(s.users, user)
	return s.snapshot, s.err
}

type executorBrowserProvider struct {
	capabilities  RunnerCapabilities
	capabilityErr error
	sessionErr    error
	sessions      []*executorBrowserSession
	probed        int
	planned       []Recipe
}

func (p *executorBrowserProvider) Capabilities(_ context.Context, recipe Recipe) (RunnerCapabilities, error) {
	p.probed++
	p.planned = append(p.planned, recipe)
	return p.capabilities, p.capabilityErr
}

func (p *executorBrowserProvider) NewIsolatedSession(_ context.Context, recipe Recipe) (BrowserSession, error) {
	p.planned = append(p.planned, recipe)
	if p.sessionErr != nil {
		return nil, p.sessionErr
	}
	session := &executorBrowserSession{
		login:  LoginOutcome{Authenticated: true},
		access: AccessOutcome{Authenticated: true, VerifiedRole: recipe.ProtectedTarget.ExpectedRole, TargetAllowed: recipe.ProtectedTarget.ExpectedAccess == AccessAllowed},
		run:    RunPassed,
	}
	p.sessions = append(p.sessions, session)
	return session, nil
}

type executorBrowserSession struct {
	login            LoginOutcome
	access           AccessOutcome
	run              RunOutcome
	loginErr         error
	accessErr        error
	runErr           error
	closeErr         error
	suppressErr      error
	resumeErr        error
	suppressed       bool
	suppressionCalls []bool
	loginAccount     Account
	loginSeen        bool
	accessSeen       bool
	runSeen          bool
	closed           bool
	lastRecipe       Recipe
}

func (s *executorBrowserSession) Login(_ context.Context, _ LoginFormRecipe, account Account) (LoginOutcome, error) {
	if !s.suppressed {
		return LoginOutcome{}, errors.New("credential artifacts were not suppressed")
	}
	s.loginAccount = account
	s.loginSeen = true
	return s.login, s.loginErr
}

func (s *executorBrowserSession) VerifyProtectedTarget(_ context.Context, _ ProtectedTargetRecipe) (AccessOutcome, error) {
	if s.suppressed {
		return AccessOutcome{}, errors.New("credential artifact suppression was not resumed after login")
	}
	s.accessSeen = true
	return s.access, s.accessErr
}

func (s *executorBrowserSession) Run(_ context.Context, recipe Recipe) (RunOutcome, error) {
	if !s.loginSeen || !s.accessSeen || s.suppressed {
		return "", errors.New("runner was called before same-session authentication")
	}
	s.lastRecipe = recipe
	s.runSeen = true
	return s.run, s.runErr
}

func (s *executorBrowserSession) SetCredentialArtifactSuppression(_ context.Context, suppress bool) error {
	s.suppressionCalls = append(s.suppressionCalls, suppress)
	if suppress && s.suppressErr != nil {
		return s.suppressErr
	}
	if !suppress && s.resumeErr != nil {
		return s.resumeErr
	}
	s.suppressed = suppress
	return nil
}

func (s *executorBrowserSession) Close() error {
	s.closed = true
	return s.closeErr
}

func executorFixture() (Recipe, *executorCredentialSource, *executorBrowserProvider) {
	account, _ := NewAccount("operator", "synthetic-login", "synthetic-password", "operator")
	recipe := Recipe{
		Environment: "local fixture",
		BaseURL:     "http://127.0.0.1:8081",
		ApprovedOrigins: []string{
			"http://127.0.0.1:8081",
		},
		Authentication: AuthenticationForm,
		Login: LoginFormRecipe{
			EntryURL:        "http://127.0.0.1:8081/login",
			LoginLocator:    "[name=login]",
			PasswordLocator: "[name=password]",
			SubmitLocator:   "button[type=submit]",
		},
		ProtectedTarget: ProtectedTargetRecipe{
			URL:            "http://127.0.0.1:8081/operator",
			ExpectedRole:   "operator",
			ExpectedAccess: AccessAllowed,
		},
		UserID:                  "operator",
		UserProfile:             "operator",
		CoverageIDs:             []string{"screen-operator-01"},
		Purpose:                 PurposeRepeatableE2E,
		Method:                  MethodPlaywrightTestSuite,
		ToolName:                "playwright",
		ToolVersion:             "1.63.0",
		ToolVersionCommand:      []string{"playwright", "--version"},
		ExistingPlaywrightSuite: true,
		StartCommand:            "go run ./testfixture",
		ReadinessCommand:        "wait for fixture readiness endpoint",
		E2ECommand:              "go test ./testfixture -run TestOperatorFlow",
		ActionTimeout:           5 * time.Second,
		TestTimeout:             time.Minute,
		Fixtures:                []string{"fixture:operator"},
		EvidencePaths:           []string{"screenshots/screen-operator-01.png"},
	}
	credentials := &executorCredentialSource{snapshot: CredentialSnapshot{account: account}}
	provider := &executorBrowserProvider{capabilities: RunnerCapabilities{
		ToolAvailable:                 true,
		ToolName:                      "playwright",
		ToolVersion:                   "1.64.0",
		PlaywrightVersion:             "1.64.0",
		Method:                        MethodPlaywrightTestSuite,
		MethodSupported:               true,
		FreshIsolatedContext:          true,
		SameContextLoginAndTests:      true,
		PrivateCredentialChannel:      true,
		ApprovedOriginsEnforced:       true,
		AuthStateMemoryOnly:           true,
		CredentialArtifactSuppression: true,
	}}
	return recipe, credentials, provider
}

func TestExecutorAuthenticatesAndRunsInFreshSameContext(t *testing.T) {
	recipe, credentials, provider := executorFixture()
	executor := NewExecutor(credentials, provider, nil)

	first := executor.Execute(context.Background(), recipe)
	second := executor.Execute(context.Background(), recipe)

	for _, result := range []PreparationResult{first, second} {
		if result.Status != PreparationReady || result.VerifiedRole != "operator" || result.Outcome != "authenticated_tests_passed" {
			t.Fatalf("unexpected result: %+v", result)
		}
		if result.UserID != "operator" || result.Profile != "operator" || len(result.CoverageIDs) != 1 {
			t.Fatalf("sanitized result identifiers missing: %+v", result)
		}
	}
	if len(provider.sessions) != 2 || provider.sessions[0] == provider.sessions[1] {
		t.Fatal("each execution must receive a fresh isolated context")
	}
	for _, session := range provider.sessions {
		if !session.loginSeen || !session.accessSeen || !session.runSeen || !session.closed || session.suppressed {
			t.Fatal("login, protected verification, test run and close must use one context")
		}
		if len(session.suppressionCalls) != 2 || !session.suppressionCalls[0] || session.suppressionCalls[1] {
			t.Fatal("credential artifact suppression must bracket only login")
		}
	}
	if len(credentials.users) != 2 || credentials.users[0] != "operator" || credentials.users[1] != "operator" {
		t.Fatal("each execution must take a fresh selected-account snapshot")
	}
}

func TestExpectedDenialIsAValidProtectedAccessOutcome(t *testing.T) {
	recipe, credentials, provider := executorFixture()
	recipe.ProtectedTarget.ExpectedAccess = AccessDenied

	result := NewExecutor(credentials, provider, nil).Execute(context.Background(), recipe)
	if result.Status != PreparationReady || result.Outcome != "expected_denial_and_tests_passed" {
		t.Fatalf("expected denial should pass: %+v", result)
	}

	// A protected target that unexpectedly allows access fails the negative assertion.
	provider = &executorBrowserProvider{capabilities: validRunnerCapabilities()}
	providerWithUnexpectedAccess := &accessOverrideProvider{executorBrowserProvider: provider, allowed: true}
	result = NewExecutor(credentials, providerWithUnexpectedAccess, nil).Execute(context.Background(), recipe)
	if result.Status != PreparationBlocked || result.Reason != BlockProtectedAccess {
		t.Fatalf("unexpected access must fail a negative assertion: %+v", result)
	}
}

type accessOverrideProvider struct {
	*executorBrowserProvider
	allowed bool
}

func (p *accessOverrideProvider) NewIsolatedSession(_ context.Context, recipe Recipe) (BrowserSession, error) {
	session := &executorBrowserSession{
		login:  LoginOutcome{Authenticated: true},
		access: AccessOutcome{Authenticated: true, VerifiedRole: recipe.ProtectedTarget.ExpectedRole, TargetAllowed: p.allowed},
		run:    RunPassed,
	}
	p.sessions = append(p.sessions, session)
	return session, nil
}

func validRunnerCapabilities() RunnerCapabilities {
	return RunnerCapabilities{
		ToolAvailable:                 true,
		ToolName:                      "playwright",
		ToolVersion:                   "1.64.0",
		PlaywrightVersion:             "1.64.0",
		Method:                        MethodPlaywrightTestSuite,
		MethodSupported:               true,
		FreshIsolatedContext:          true,
		SameContextLoginAndTests:      true,
		PrivateCredentialChannel:      true,
		ApprovedOriginsEnforced:       true,
		AuthStateMemoryOnly:           true,
		CredentialArtifactSuppression: true,
	}
}

func TestExecutorBlocksUnsupportedAuthenticationAndInvalidAccount(t *testing.T) {
	for _, flow := range []AuthenticationFlow{AuthenticationMFA, AuthenticationCAPTCHA, AuthenticationSSO, "unknown"} {
		t.Run(string(flow), func(t *testing.T) {
			recipe, credentials, provider := executorFixture()
			recipe.Authentication = flow
			result := NewExecutor(credentials, provider, nil).Execute(context.Background(), recipe)
			if result.Status != PreparationBlocked || result.Reason != BlockUnsupportedAuth || provider.probed != 0 {
				t.Fatalf("unsupported authentication was not blocked before runner: %+v", result)
			}
		})
	}

	recipe, _, provider := executorFixture()
	wrongAccount, _ := NewAccount("operator", "synthetic-login", "synthetic-password", "administrator")
	credentials := &executorCredentialSource{snapshot: CredentialSnapshot{account: wrongAccount}}
	result := NewExecutor(credentials, provider, nil).Execute(context.Background(), recipe)
	if result.Status != PreparationBlocked || result.Reason != BlockInvalidAccount || len(provider.sessions) != 0 {
		t.Fatalf("invalid account profile was not blocked: %+v", result)
	}
}

func TestExecutorBlocksMissingToolsVersionsAndContextCapabilityGaps(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*RunnerCapabilities)
		want   BlockReason
	}{
		{"missing tool", func(c *RunnerCapabilities) { c.ToolAvailable = false }, BlockToolUnavailable},
		{"below minimum Playwright", func(c *RunnerCapabilities) { c.PlaywrightVersion = "1.62.9" }, BlockToolVersion},
		{"runtime older than planned tool", func(c *RunnerCapabilities) { c.ToolVersion = "1.62.9" }, BlockToolVersion},
		{"method unsupported", func(c *RunnerCapabilities) { c.MethodSupported = false }, BlockMethodUnavailable},
		{"no isolated context", func(c *RunnerCapabilities) { c.FreshIsolatedContext = false }, BlockMethodUnavailable},
		{"different login and test context", func(c *RunnerCapabilities) { c.SameContextLoginAndTests = false }, BlockMethodUnavailable},
		{"credential channel exposed", func(c *RunnerCapabilities) { c.PrivateCredentialChannel = false }, BlockMethodUnavailable},
		{"unrestricted origins", func(c *RunnerCapabilities) { c.ApprovedOriginsEnforced = false }, BlockMethodUnavailable},
		{"persistent auth state", func(c *RunnerCapabilities) { c.AuthStateMemoryOnly = false }, BlockMethodUnavailable},
		{"no artifact suppression", func(c *RunnerCapabilities) { c.CredentialArtifactSuppression = false }, BlockMethodUnavailable},
		{"wrong tool", func(c *RunnerCapabilities) { c.ToolName = "other" }, BlockMethodUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			recipe, credentials, provider := executorFixture()
			test.mutate(&provider.capabilities)
			result := NewExecutor(credentials, provider, nil).Execute(context.Background(), recipe)
			if result.Status != PreparationBlocked || result.Reason != test.want || len(provider.sessions) != 0 {
				t.Fatalf("capability gap not blocked: %+v", result)
			}
		})
	}
}

func TestExecutorFailsClosedWhenCredentialArtifactSuppressionFails(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*executorBrowserSession)
		login     bool
	}{
		{"cannot suspend", func(s *executorBrowserSession) { s.suppressErr = errors.New("synthetic suppression failure") }, false},
		{"cannot resume", func(s *executorBrowserSession) { s.resumeErr = errors.New("synthetic resume failure") }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			recipe, credentials, provider := executorFixture()
			session := &executorBrowserSession{
				login:  LoginOutcome{Authenticated: true},
				access: AccessOutcome{Authenticated: true, VerifiedRole: "operator", TargetAllowed: true},
				run:    RunPassed,
			}
			test.configure(session)
			provider.sessions = []*executorBrowserSession{session}
			providerInstance := &configuredSessionProvider{executorBrowserProvider: provider, session: session}
			result := NewExecutor(credentials, providerInstance, nil).Execute(context.Background(), recipe)
			if result.Status != PreparationBlocked || result.Reason != BlockArtifactSuppression || session.loginSeen != test.login || session.runSeen || !session.closed {
				t.Fatalf("suppression failure did not fail closed: %+v", result)
			}
		})
	}
}

type configuredSessionProvider struct {
	*executorBrowserProvider
	session *executorBrowserSession
}

func (p *configuredSessionProvider) NewIsolatedSession(context.Context, Recipe) (BrowserSession, error) {
	return p.session, nil
}

func TestExecutorEnforcesPlannedMethodPreference(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Recipe)
		want   BlockReason
	}{
		{"missing repeatable suite", func(r *Recipe) { r.ExistingPlaywrightSuite = false }, BlockMethodPreference},
		{"HTTP cannot replace browser", func(r *Recipe) { r.Method = MethodHTTP }, BlockMethodPreference},
		{"CLI without unavailable skill mode", func(r *Recipe) {
			r.Purpose = PurposeBrowserControl
			r.Method = MethodPlaywrightCLI
			r.OfficialCLISkillAvailable = false
		}, BlockMethodPreference},
		{"skills-less CLI when skill exists", func(r *Recipe) {
			r.Purpose = PurposeBrowserControl
			r.Method = MethodPlaywrightCLINoSkill
			r.OfficialCLISkillAvailable = true
		}, BlockMethodPreference},
		{"MCP without persistent need or verified gap", func(r *Recipe) { r.Purpose = PurposeBrowserControl; r.Method = MethodMCP }, BlockMethodPreference},
	} {
		t.Run(test.name, func(t *testing.T) {
			recipe, credentials, provider := executorFixture()
			test.change(&recipe)
			result := NewExecutor(credentials, provider, nil).Execute(context.Background(), recipe)
			if result.Status != PreparationBlocked || result.Reason != test.want || provider.probed != 0 {
				t.Fatalf("method policy was not enforced: %+v", result)
			}
		})
	}

	recipe, credentials, provider := executorFixture()
	recipe.Purpose = PurposeBrowserControl
	recipe.Method = MethodPlaywrightCLINoSkill
	provider.capabilities = validRunnerCapabilities()
	provider.capabilities.Method = recipe.Method
	result := NewExecutor(credentials, provider, nil).Execute(context.Background(), recipe)
	if result.Status != PreparationReady {
		t.Fatalf("skills-less CLI should be admitted when the official skill is unavailable: %+v", result)
	}
}

func TestExecutorSentinelAbsentFromSanitizedSurfaces(t *testing.T) {
	const sentinelLogin = "SENTINEL_EXECUTOR_LOGIN_7c9a"
	const sentinelPassword = "SENTINEL_EXECUTOR_PASSWORD_2d4f"
	account, _ := NewAccount("operator", sentinelLogin, sentinelPassword, "operator")
	recipe := validExecutorRecipe()
	credentials := &executorCredentialSource{snapshot: CredentialSnapshot{account: account}}
	provider := &executorBrowserProvider{capabilities: validRunnerCapabilities()}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	result := NewExecutor(credentials, provider, logger).Execute(context.Background(), recipe)
	if result.Status != PreparationReady {
		t.Fatalf("execution failed: %+v", result)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	recipeJSON, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	public := strings.Join([]string{
		result.String(),
		fmt.Sprintf("%+v %#v %s", result, result, result),
		string(resultJSON),
		recipe.String(),
		fmt.Sprintf("%+v %#v %s", recipe, recipe, recipe),
		string(recipeJSON),
		logs.String(),
		provider.planned[0].String(),
		fmt.Sprintf("%+v %#v %s", provider.planned[0], provider.planned[0], provider.planned[0]),
		provider.sessions[0].lastRecipe.String(),
		fmt.Sprintf("%+v %#v %s", provider.sessions[0].lastRecipe, provider.sessions[0].lastRecipe, provider.sessions[0].lastRecipe),
	}, "\n")
	if strings.Contains(public, sentinelLogin) || strings.Contains(public, sentinelPassword) {
		t.Fatal("credential sentinel leaked into result, recipe, runner arguments, formatting or logs")
	}
	if provider.sessions[0].loginAccount.Login() != sentinelLogin || provider.sessions[0].loginAccount.Password() != sentinelPassword {
		t.Fatal("private login did not receive the selected account")
	}
}

func TestSentinelInInvalidRecipeMetadataNeverReachesSurfaces(t *testing.T) {
	const secret = "SENTINEL_INVALID_PROFILE_SECRET"
	for _, invalidRecipe := range []bool{false, true} {
		recipe, credentials, provider := executorFixture()
		account, err := NewAccount("operator", "synthetic-login", secret, "operator")
		if err != nil {
			t.Fatal(err)
		}
		credentials.snapshot = CredentialSnapshot{account: account}
		recipe.UserProfile = secret
		recipe.ProtectedTarget.ExpectedRole = secret
		if invalidRecipe {
			recipe.ToolVersion = "malformed"
		}
		var logs bytes.Buffer
		result := NewExecutor(credentials, provider, slog.New(slog.NewTextHandler(&logs, nil))).Execute(context.Background(), recipe)
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != PreparationBlocked || strings.Contains(string(data), secret) || strings.Contains(logs.String(), secret) {
			t.Fatal("invalid account/recipe metadata leaked private credential sentinel")
		}
	}
}

func TestExecutorRejectsSentinelInPlannedCoverageMetadata(t *testing.T) {
	const sentinel = "SENTINEL_COVERAGE_PASSWORD_0a61"
	mutations := []struct {
		name   string
		change func(*Recipe)
	}{
		{name: "requirement reference", change: func(recipe *Recipe) { recipe.CoverageRequirement = sentinel }},
		{name: "screen or journey", change: func(recipe *Recipe) { recipe.CoverageScreenJourney = sentinel }},
		{name: "expected result", change: func(recipe *Recipe) { recipe.CoverageExpectedResult = sentinel }},
		{name: "evidence requirement", change: func(recipe *Recipe) { recipe.EvidenceRequirements = []string{sentinel} }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			recipe, credentials, provider := executorFixture()
			account, err := NewAccount("operator", "synthetic-login", sentinel, "operator")
			if err != nil {
				t.Fatal("create synthetic sentinel account")
			}
			credentials.snapshot = CredentialSnapshot{account: account}
			mutation.change(&recipe)
			var logs bytes.Buffer
			result := NewExecutor(credentials, provider, slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))).Execute(context.Background(), recipe)
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal("marshal sanitized blocker")
			}
			public := strings.Join([]string{string(encoded), result.String(), fmt.Sprintf("%+v %#v", result, result), logs.String()}, "\n")
			if result.Status != PreparationBlocked || result.Reason != BlockUnsafeRecipe || strings.Contains(public, sentinel) || provider.probed != 0 {
				t.Fatalf("credential-bearing plan metadata was admitted or disclosed: status=%s reason=%s", result.Status, result.Reason)
			}
		})
	}
}

func TestExecutorRepeatableE2EWithoutSuiteUsesPlannedCLI(t *testing.T) {
	recipe, credentials, provider := executorFixture()
	recipe.ExistingPlaywrightSuite = false
	recipe.Method = MethodPlaywrightCLI
	recipe.OfficialCLISkillAvailable = true
	provider.capabilities.Method = recipe.Method
	result := NewExecutor(credentials, provider, nil).Execute(context.Background(), recipe)
	if result.Status != PreparationReady {
		t.Fatalf("CLI fallback without existing suite blocked: %v", result.Reason)
	}
}

func TestExecutorSanitizesRunnerErrorsAndDisclosesTrustedHarnessBoundary(t *testing.T) {
	const sentinel = "SENTINEL_RUNNER_FAILURE_87ab"
	recipe, credentials, provider := executorFixture()
	recipe.UnrestrictedHarness = true
	provider.capabilityErr = fmt.Errorf("untrusted runner details: %s", sentinel)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	result := NewExecutor(credentials, provider, logger).Execute(context.Background(), recipe)
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Reason != BlockMethodUnavailable || !strings.Contains(result.Disclosure, "not sandboxed") {
		t.Fatalf("missing sanitized blocker or trusted-harness disclosure: %+v", result)
	}
	if strings.Contains(fmt.Sprintf("%+v %#v %s", result, result, result), sentinel) || strings.Contains(string(serialized), sentinel) || strings.Contains(logs.String(), sentinel) {
		t.Fatal("runner error value leaked through results or logs")
	}
}

func TestExecutorRejectsUnapprovedOriginAndCredentialBearingRecipe(t *testing.T) {
	recipe, credentials, provider := executorFixture()
	recipe.ProtectedTarget.URL = "http://unapproved.invalid/private"
	result := NewExecutor(credentials, provider, nil).Execute(context.Background(), recipe)
	if result.Reason != BlockInvalidRecipe || provider.probed != 0 {
		t.Fatalf("unapproved protected origin accepted: %+v", result)
	}

	recipe, credentials, provider = executorFixture()
	account := credentials.snapshot.Account()
	recipe.ReadinessCommand = account.Password()
	result = NewExecutor(credentials, provider, nil).Execute(context.Background(), recipe)
	if result.Reason != BlockUnsafeRecipe || provider.probed != 0 || len(provider.sessions) != 0 {
		t.Fatalf("credential-bearing recipe accepted: %+v", result)
	}
}

func TestExecutorClosesSessionOnBlockedLogin(t *testing.T) {
	recipe, credentials, provider := executorFixture()
	providerFactory := &loginOverrideProvider{executorBrowserProvider: provider, outcome: LoginOutcome{InvalidAccount: true}}
	result := NewExecutor(credentials, providerFactory, nil).Execute(context.Background(), recipe)
	if result.Status != PreparationBlocked || result.Reason != BlockInvalidAccount || len(providerFactory.sessions) != 1 || !providerFactory.sessions[0].closed {
		t.Fatalf("invalid account did not block and close context: %+v", result)
	}
}

type loginOverrideProvider struct {
	*executorBrowserProvider
	outcome LoginOutcome
}

func (p *loginOverrideProvider) NewIsolatedSession(_ context.Context, _ Recipe) (BrowserSession, error) {
	session := &executorBrowserSession{login: p.outcome, access: AccessOutcome{Authenticated: true, VerifiedRole: "operator", TargetAllowed: true}, run: RunPassed}
	p.sessions = append(p.sessions, session)
	return session, nil
}

func validExecutorRecipe() Recipe {
	recipe, _, _ := executorFixture()
	return recipe
}
