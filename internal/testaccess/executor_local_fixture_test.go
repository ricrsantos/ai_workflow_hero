package testaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	fixtureOperatorLogin    = "SENTINEL_FIXTURE_OPERATOR_LOGIN_61d7"
	fixtureOperatorPassword = "SENTINEL_FIXTURE_OPERATOR_PASSWORD_2a91"
	fixtureAdminLogin       = "SENTINEL_FIXTURE_ADMIN_LOGIN_1b8e"
	fixtureAdminPassword    = "SENTINEL_FIXTURE_ADMIN_PASSWORD_913f"
	fixtureInvalidPassword  = "SENTINEL_FIXTURE_INVALID_PASSWORD_03a6"
	fixtureTokenPrefix      = "SENTINEL_FIXTURE_TOKEN_"
)

type fixturePrincipal struct {
	login    string
	password string
	role     string
}

type fixtureRequest struct {
	token string
	path  string
	role  string
	code  int
}

// protectedFixture is a local form-login application used to exercise the
// executor's private credential and same-context contract without a browser
// installation or external service.
type protectedFixture struct {
	mu         sync.Mutex
	principals []fixturePrincipal
	sessions   map[string]string
	requests   []fixtureRequest
	loginRoles []string
	nextToken  int
	server     *httptest.Server
}

func newProtectedFixture() *protectedFixture {
	fixture := &protectedFixture{
		principals: []fixturePrincipal{
			{login: fixtureOperatorLogin, password: fixtureOperatorPassword, role: "operator"},
			{login: fixtureAdminLogin, password: fixtureAdminPassword, role: "admin"},
		},
		sessions: make(map[string]string),
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	return fixture
}

func (f *protectedFixture) close() { f.server.Close() }

func (f *protectedFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/login" {
		f.serveLogin(w, r)
		return
	}
	if r.Method != http.MethodGet || (r.URL.Path != "/operator" && r.URL.Path != "/admin") {
		http.NotFound(w, r)
		return
	}
	cookie, err := r.Cookie("hero_test_session")
	if err != nil {
		f.recordRequest("", r.URL.Path, "", http.StatusUnauthorized)
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	role, authenticated := f.sessions[cookie.Value]
	f.mu.Unlock()
	if !authenticated {
		f.recordRequest(cookie.Value, r.URL.Path, "", http.StatusUnauthorized)
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	w.Header().Set("X-Fixture-Role", role)
	if strings.TrimPrefix(r.URL.Path, "/") != role {
		f.recordRequest(cookie.Value, r.URL.Path, role, http.StatusForbidden)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	f.recordRequest(cookie.Value, r.URL.Path, role, http.StatusOK)
	w.WriteHeader(http.StatusOK)
}

func (f *protectedFixture) serveLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	var principal fixturePrincipal
	for _, candidate := range f.principals {
		if r.Form.Get("login") == candidate.login && r.Form.Get("password") == candidate.password {
			principal = candidate
			break
		}
	}
	if principal.role == "" {
		http.Error(w, "invalid account", http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	f.nextToken++
	token := fmt.Sprintf("%s%02d", fixtureTokenPrefix, f.nextToken)
	f.sessions[token] = principal.role
	f.loginRoles = append(f.loginRoles, principal.role)
	f.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     "hero_test_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (f *protectedFixture) recordRequest(token, path, role string, code int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, fixtureRequest{token: token, path: path, role: role, code: code})
}

func (f *protectedFixture) snapshot() ([]string, []fixtureRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.loginRoles...), append([]fixtureRequest(nil), f.requests...)
}

type fixtureCredentials struct {
	accounts  map[string]Account
	requested []string
}

func (s *fixtureCredentials) Snapshot(_ context.Context, userID string) (CredentialSnapshot, error) {
	s.requested = append(s.requested, userID)
	account, ok := s.accounts[userID]
	if !ok {
		return CredentialSnapshot{}, errors.New("synthetic account missing")
	}
	return CredentialSnapshot{account: account}, nil
}

type fixtureBrowserProvider struct {
	sessions []*fixtureBrowserSession
}

func (*fixtureBrowserProvider) Capabilities(_ context.Context, recipe Recipe) (RunnerCapabilities, error) {
	capabilities := validRunnerCapabilities()
	capabilities.Method = recipe.Method
	return capabilities, nil
}

func (p *fixtureBrowserProvider) NewIsolatedSession(_ context.Context, recipe Recipe) (BrowserSession, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, errors.New("create synthetic isolated cookie jar")
	}
	base, err := url.Parse(recipe.BaseURL)
	if err != nil {
		return nil, errors.New("parse synthetic fixture origin")
	}
	session := &fixtureBrowserSession{
		client:  &http.Client{Jar: jar, Timeout: 2 * time.Second},
		baseURL: base,
	}
	session.hadPriorAuth = len(jar.Cookies(base)) != 0
	p.sessions = append(p.sessions, session)
	return session, nil
}

type fixtureBrowserSession struct {
	client            *http.Client
	baseURL           *url.URL
	suppressed        bool
	closed            bool
	hadPriorAuth      bool
	sessionToken      string
	loginSeen         bool
	accessSeen        bool
	runSeen           bool
	accessRequestPath string
	runRequestPath    string
}

func (s *fixtureBrowserSession) SetCredentialArtifactSuppression(_ context.Context, suppress bool) error {
	if s.closed {
		return errors.New("synthetic context is closed")
	}
	s.suppressed = suppress
	return nil
}

func (s *fixtureBrowserSession) Login(ctx context.Context, recipe LoginFormRecipe, account Account) (LoginOutcome, error) {
	if !s.suppressed || s.closed {
		return LoginOutcome{}, errors.New("synthetic credential capture guard is not active")
	}
	loginField, ok := fixtureFieldName(recipe.LoginLocator)
	if !ok {
		return LoginOutcome{}, errors.New("synthetic login locator is unsupported")
	}
	passwordField, ok := fixtureFieldName(recipe.PasswordLocator)
	if !ok {
		return LoginOutcome{}, errors.New("synthetic password locator is unsupported")
	}
	form := url.Values{loginField: {account.Login()}, passwordField: {account.Password()}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, recipe.EntryURL, strings.NewReader(form.Encode()))
	if err != nil {
		return LoginOutcome{}, errors.New("create synthetic login request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := s.client.Do(request)
	if err != nil {
		return LoginOutcome{}, errors.New("submit synthetic login form")
	}
	if err := response.Body.Close(); err != nil {
		return LoginOutcome{}, errors.New("close synthetic login response")
	}
	s.loginSeen = true
	if response.StatusCode == http.StatusUnauthorized {
		return LoginOutcome{InvalidAccount: true}, nil
	}
	if response.StatusCode != http.StatusNoContent {
		return LoginOutcome{}, errors.New("synthetic login was not accepted")
	}
	for _, cookie := range s.client.Jar.Cookies(s.baseURL) {
		if cookie.Name == "hero_test_session" {
			s.sessionToken = cookie.Value
			break
		}
	}
	return LoginOutcome{Authenticated: s.sessionToken != ""}, nil
}

func (s *fixtureBrowserSession) VerifyProtectedTarget(ctx context.Context, target ProtectedTargetRecipe) (AccessOutcome, error) {
	if s.suppressed || s.closed || !s.loginSeen {
		return AccessOutcome{}, errors.New("synthetic protected check is outside the authenticated context")
	}
	response, err := s.client.Get(target.URL)
	if err != nil {
		return AccessOutcome{}, errors.New("request synthetic protected target")
	}
	if err := response.Body.Close(); err != nil {
		return AccessOutcome{}, errors.New("close synthetic protected response")
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusForbidden && response.StatusCode != http.StatusUnauthorized {
		return AccessOutcome{}, errors.New("synthetic protected target returned an unexpected status")
	}
	s.accessSeen = true
	s.accessRequestPath = response.Request.URL.Path
	return AccessOutcome{
		Authenticated: response.StatusCode != http.StatusUnauthorized,
		VerifiedRole:  response.Header.Get("X-Fixture-Role"),
		TargetAllowed: response.StatusCode == http.StatusOK,
	}, nil
}

func (s *fixtureBrowserSession) Run(ctx context.Context, recipe Recipe) (RunOutcome, error) {
	if s.suppressed || s.closed || !s.loginSeen || !s.accessSeen {
		return RunBlocked, errors.New("synthetic tests did not receive the authenticated context")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, recipe.ProtectedTarget.URL, nil)
	if err != nil {
		return RunBlocked, errors.New("create synthetic protected test request")
	}
	response, err := s.client.Do(request)
	if err != nil {
		return RunBlocked, errors.New("run synthetic protected test")
	}
	if err := response.Body.Close(); err != nil {
		return RunBlocked, errors.New("close synthetic protected test response")
	}
	s.runSeen = true
	s.runRequestPath = response.Request.URL.Path
	allowed := response.StatusCode == http.StatusOK
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusForbidden {
		return RunFailed, nil
	}
	if (recipe.ProtectedTarget.ExpectedAccess == AccessAllowed) != allowed {
		return RunFailed, nil
	}
	return RunPassed, nil
}

func (s *fixtureBrowserSession) Close() error {
	s.closed = true
	return nil
}

func fixtureFieldName(locator string) (string, bool) {
	const prefix = "[name="
	if !strings.HasPrefix(locator, prefix) || !strings.HasSuffix(locator, "]") {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(locator, prefix), "]")
	return name, name != "" && !strings.ContainsAny(name, "[]\r\n")
}

func TestExecutorSyntheticLocalFixtureFreshLoginSameContextAndExpectedDenial(t *testing.T) {
	fixture := newProtectedFixture()
	defer fixture.close()
	accounts := map[string]Account{}
	for _, accountData := range []struct {
		id, login, password, role string
	}{
		{"operator", fixtureOperatorLogin, fixtureOperatorPassword, "operator"},
		{"administrator", fixtureAdminLogin, fixtureAdminPassword, "admin"},
	} {
		account, err := NewAccount(accountData.id, accountData.login, accountData.password, accountData.role)
		if err != nil {
			t.Fatal("create synthetic fixture account")
		}
		accounts[accountData.id] = account
	}
	credentials := &fixtureCredentials{accounts: accounts}
	provider := &fixtureBrowserProvider{}
	var logs bytes.Buffer
	executor := NewExecutor(credentials, provider, slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))

	operatorRecipe := syntheticFixtureRecipe(fixture.server.URL, "operator", "operator", "operator", AccessAllowed)
	adminRecipe := syntheticFixtureRecipe(fixture.server.URL, "administrator", "admin", "admin", AccessAllowed)
	denialRecipe := syntheticFixtureRecipe(fixture.server.URL, "operator", "operator", "admin", AccessDenied)
	results := []PreparationResult{
		executor.Execute(context.Background(), operatorRecipe),
		executor.Execute(context.Background(), adminRecipe),
		executor.Execute(context.Background(), denialRecipe),
	}
	for i, result := range results {
		if result.Status != PreparationReady {
			t.Fatalf("synthetic execution %d blocked: status=%s reason=%s", i, result.Status, result.Reason)
		}
	}
	if results[0].VerifiedRole != "operator" || results[0].Outcome != "authenticated_tests_passed" {
		t.Fatalf("operator protected flow was not verified: %+v", results[0])
	}
	if results[1].VerifiedRole != "admin" || results[1].Outcome != "authenticated_tests_passed" {
		t.Fatalf("admin protected flow was not verified: %+v", results[1])
	}
	if results[2].VerifiedRole != "operator" || results[2].Outcome != "expected_denial_and_tests_passed" {
		t.Fatalf("operator denial on admin route was not treated as an expected pass: %+v", results[2])
	}
	invalidAccount, err := NewAccount("operator", fixtureOperatorLogin, fixtureInvalidPassword, "operator")
	if err != nil {
		t.Fatal("create synthetic invalid account")
	}
	credentials.accounts["operator"] = invalidAccount
	invalidResult := executor.Execute(context.Background(), operatorRecipe)
	if invalidResult.Status != PreparationBlocked || invalidResult.Reason != BlockInvalidAccount {
		t.Fatalf("synthetic invalid account was not blocked: status=%s reason=%s", invalidResult.Status, invalidResult.Reason)
	}
	results = append(results, invalidResult)

	loginRoles, requests := fixture.snapshot()
	if got, want := strings.Join(loginRoles, ","), "operator,admin,operator"; got != want {
		t.Fatalf("fixture login roles = %q, want %q", got, want)
	}
	if len(provider.sessions) != len(results) || len(credentials.requested) != len(results) {
		t.Fatalf("each execution must create a fresh session and snapshot: sessions=%d snapshots=%d", len(provider.sessions), len(credentials.requested))
	}
	seenTokens := make(map[string]bool, len(provider.sessions))
	for i, session := range provider.sessions[:3] {
		if session.hadPriorAuth || session.sessionToken == "" || seenTokens[session.sessionToken] {
			t.Fatalf("execution %d reused or lacked isolated authentication state", i)
		}
		seenTokens[session.sessionToken] = true
		if !session.loginSeen || !session.accessSeen || !session.runSeen || !session.closed {
			t.Fatalf("execution %d did not login, verify, run and close in one session", i)
		}
		if session.accessRequestPath != session.runRequestPath {
			t.Fatalf("execution %d changed protected target between check and test", i)
		}
	}
	failedSession := provider.sessions[3]
	if failedSession.sessionToken != "" || !failedSession.loginSeen || failedSession.accessSeen || failedSession.runSeen || !failedSession.closed {
		t.Fatal("invalid login did not block before protected access or close its fresh context")
	}
	for i := 0; i < len(requests); i += 2 {
		if i+1 >= len(requests) || requests[i].token == "" || requests[i].token != requests[i+1].token {
			t.Fatalf("fixture requests %d and %d did not share an in-memory session", i, i+1)
		}
		if requests[i].path != requests[i+1].path {
			t.Fatalf("fixture requests %d and %d used different target paths", i, i+1)
		}
	}

	var resultJSON bytes.Buffer
	for _, result := range results {
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal("marshal sanitized synthetic result")
		}
		resultJSON.Write(encoded)
	}
	publicSurfaces := strings.Join([]string{
		resultJSON.String(),
		fmt.Sprintf("%v %+v %#v", results, results, results),
		operatorRecipe.String(), fmt.Sprintf("%+v %#v", operatorRecipe, operatorRecipe),
		adminRecipe.String(), fmt.Sprintf("%+v %#v", adminRecipe, adminRecipe),
		denialRecipe.String(), fmt.Sprintf("%+v %#v", denialRecipe, denialRecipe),
		logs.String(),
	}, "\n")
	for _, secret := range []string{
		fixtureOperatorLogin, fixtureOperatorPassword, fixtureAdminLogin, fixtureAdminPassword,
		fixtureInvalidPassword,
		fixtureTokenPrefix + "01", fixtureTokenPrefix + "02", fixtureTokenPrefix + "03",
	} {
		if strings.Contains(publicSurfaces, secret) {
			t.Fatal("synthetic credential or session-token sentinel leaked to result, recipe, or log output")
		}
	}
}

func syntheticFixtureRecipe(baseURL, userID, profile, targetRole string, expected AccessExpectation) Recipe {
	return Recipe{
		Environment:     "synthetic local protected fixture",
		BaseURL:         baseURL,
		ApprovedOrigins: []string{baseURL},
		Authentication:  AuthenticationForm,
		Login: LoginFormRecipe{
			EntryURL:        baseURL + "/login",
			LoginLocator:    "[name=login]",
			PasswordLocator: "[name=password]",
			SubmitLocator:   "button[type=submit]",
		},
		ProtectedTarget: ProtectedTargetRecipe{
			URL:            baseURL + "/" + targetRole,
			ExpectedRole:   profile,
			ExpectedAccess: expected,
		},
		UserID:                  userID,
		UserProfile:             profile,
		CoverageIDs:             []string{"fixture-" + userID + "-" + targetRole},
		Purpose:                 PurposeRepeatableE2E,
		Method:                  MethodPlaywrightTestSuite,
		ToolName:                "playwright",
		ToolVersion:             MinimumPlaywrightVersion,
		ToolVersionCommand:      []string{"playwright", "--version"},
		ExistingPlaywrightSuite: true,
		StartCommand:            "in-process synthetic fixture",
		ReadinessCommand:        "httptest server ready",
		E2ECommand:              "synthetic protected-route assertion",
		ActionTimeout:           2 * time.Second,
		TestTimeout:             5 * time.Second,
		Fixtures:                []string{"synthetic operator and admin accounts"},
		EvidencePaths:           []string{"memory-only synthetic evidence"},
	}
}
