package testaccess

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// BrowserPlanRelativePath is the Planning-owned, non-secret browser contract.
	BrowserPlanRelativePath = ".workflow-hero/cycles/current/browser-plan.json"
	// BrowserPlanSchemaVersion identifies the shared browser plan schema.
	BrowserPlanSchemaVersion = 1
	maxBrowserPlanSize       = 1 << 20
)

var coverageIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,127}$`)

// ValidationStage identifies the workflow stage that owns the planned method.
type ValidationStage string

const (
	StageBrowserUIValidation ValidationStage = "browser_ui_validation"
	StageQAEndToEnd          ValidationStage = "qa_end_to_end"
)

// AuthenticationRequirement makes public, protected, and unresolved access
// explicit in the Planning contract.
type AuthenticationRequirement string

const (
	AuthenticationNotNeeded  AuthenticationRequirement = "not_needed"
	AuthenticationRequired   AuthenticationRequirement = "required"
	AuthenticationUnresolved AuthenticationRequirement = "unresolved"
)

// BrowserPlan is the shared JSON contract produced during Planning. It contains
// no credential fields; those remain in the private .env.hero store.
type BrowserPlan struct {
	SchemaVersion  int                       `json:"schema_version"`
	Execution      BrowserExecutionContract  `json:"execution"`
	Authentication BrowserAuthenticationPlan `json:"authentication"`
	Method         BrowserMethodPlan         `json:"method"`
	Coverage       []CoverageItem            `json:"coverage"`
}

func (BrowserPlan) String() string   { return "testaccess.BrowserPlan[redacted]" }
func (BrowserPlan) GoString() string { return "testaccess.BrowserPlan[redacted]" }

// BrowserExecutionContract contains Planning-resolved project details. Time
// limits use Go duration syntax so they remain readable in the JSON artifact.
type BrowserExecutionContract struct {
	Environment         string   `json:"environment"`
	BaseURL             string   `json:"base_url"`
	ApprovedOrigins     []string `json:"approved_origins"`
	StartCommand        string   `json:"start_command"`
	ReadinessCommand    string   `json:"readiness_command"`
	E2ECommand          string   `json:"e2e_command,omitempty"`
	ActionTimeout       string   `json:"action_timeout"`
	TestTimeout         string   `json:"test_timeout"`
	Fixtures            []string `json:"fixtures"`
	EvidencePaths       []string `json:"evidence_paths"`
	UnrestrictedHarness bool     `json:"unrestricted_harness,omitempty"`
}

// BrowserAuthenticationPlan describes whether browser coverage needs login
// and, when required, the non-secret form-login recipe.
type BrowserAuthenticationPlan struct {
	Requirement AuthenticationRequirement `json:"requirement"`
	Flow        AuthenticationFlow        `json:"flow"`
	Login       LoginFormRecipe           `json:"login"`
}

// BrowserMethodPlan records the approved runner and Planning-time capability
// choices. Preparation still verifies the selected method in the live session.
type BrowserMethodPlan struct {
	Stage                     ValidationStage   `json:"stage,omitempty"`
	Purpose                   ValidationPurpose `json:"purpose"`
	Method                    ExecutionMethod   `json:"method"`
	ToolName                  string            `json:"tool_name"`
	ToolVersion               string            `json:"tool_version"`
	ToolVersionCommand        []string          `json:"tool_version_command,omitempty"`
	PlaywrightVersion         string            `json:"playwright_version"`
	ExistingPlaywrightSuite   bool              `json:"existing_playwright_suite,omitempty"`
	OfficialCLISkillAvailable bool              `json:"official_cli_skill_available,omitempty"`
	PersistentBrowserNeeded   bool              `json:"persistent_browser_needed,omitempty"`
	VerifiedCLICapabilityGap  bool              `json:"verified_cli_capability_gap,omitempty"`
}

// CoverageItem is one stable denominator row shared by preparation, reports,
// and the later coverage gate.
type CoverageItem struct {
	ID                      string                `json:"id"`
	RequirementRef          string                `json:"requirement_ref"`
	ScreenOrJourney         string                `json:"screen_or_journey"`
	UserID                  string                `json:"user_id,omitempty"`
	Profile                 string                `json:"profile"`
	Mandatory               bool                  `json:"mandatory"`
	ExpectedResult          string                `json:"expected_result"`
	EvidenceRequirements    []string              `json:"evidence_requirements"`
	OptionalReferenceWidths []int                 `json:"optional_reference_widths,omitempty"`
	ProtectedTarget         ProtectedTargetRecipe `json:"protected_target"`
}

// LoadBrowserPlan securely reads and validates the current cycle's plan. The
// path is anchored beneath projectDir and every managed path component must be
// a real directory; symlink substitution and oversized plans are rejected.
func LoadBrowserPlan(projectDir string) (BrowserPlan, error) {
	if !filepath.IsAbs(projectDir) || filepath.Clean(projectDir) != projectDir || projectDir == string(filepath.Separator) {
		return BrowserPlan{}, errors.New("browser plan requires an absolute, clean project root")
	}
	rootInfo, err := os.Lstat(projectDir)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return BrowserPlan{}, errors.New("browser plan project root must be a real directory")
	}
	root, err := os.OpenRoot(projectDir)
	if err != nil {
		return BrowserPlan{}, errors.New("cannot open browser plan project root")
	}
	defer func() { _ = root.Close() }()

	current := root
	var nested []*os.Root
	defer func() {
		for i := len(nested) - 1; i >= 0; i-- {
			_ = nested[i].Close()
		}
	}()
	for _, component := range []string{".workflow-hero", "cycles", "current"} {
		info, err := current.Lstat(component)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return BrowserPlan{}, errors.New("browser plan path contains a missing or unsafe directory")
		}
		child, err := current.OpenRoot(component)
		if err != nil {
			return BrowserPlan{}, errors.New("cannot open browser plan directory")
		}
		opened, err := child.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			_ = child.Close()
			return BrowserPlan{}, errors.New("browser plan directory changed while opening")
		}
		nested = append(nested, child)
		current = child
	}

	info, err := current.Lstat("browser-plan.json")
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return BrowserPlan{}, errors.New("browser-plan.json is missing or is not a regular file")
	}
	file, err := current.Open("browser-plan.json")
	if err != nil {
		return BrowserPlan{}, errors.New("cannot read browser-plan.json")
	}
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		_ = file.Close()
		return BrowserPlan{}, errors.New("browser-plan.json changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBrowserPlanSize+1))
	closeErr := file.Close()
	if err != nil || len(data) > maxBrowserPlanSize {
		return BrowserPlan{}, errors.New("browser-plan.json is unreadable or exceeds the size limit")
	}
	if closeErr != nil {
		return BrowserPlan{}, errors.New("browser-plan.json could not be closed after reading")
	}
	plan, err := decodeBrowserPlan(data)
	if err != nil {
		return BrowserPlan{}, err
	}
	if err := plan.Validate(); err != nil {
		return BrowserPlan{}, err
	}
	return plan, nil
}

// Validate checks schema shape, stable coverage identity, and the selected
// method policy without probing tools or reading credentials.
func (p BrowserPlan) Validate() error {
	if p.SchemaVersion != BrowserPlanSchemaVersion {
		return errors.New("browser plan schema version is unsupported")
	}
	if len(p.Coverage) == 0 {
		return errors.New("browser plan must declare coverage")
	}
	if err := validatePlanText(p.Execution.Environment, p.Execution.BaseURL, p.Execution.StartCommand, p.Execution.ReadinessCommand, p.Execution.ActionTimeout, p.Execution.TestTimeout); err != nil {
		return errors.New("browser plan execution contract is incomplete")
	}
	actionTimeout, actionErr := time.ParseDuration(p.Execution.ActionTimeout)
	testTimeout, testErr := time.ParseDuration(p.Execution.TestTimeout)
	if actionErr != nil || actionTimeout <= 0 || testErr != nil || testTimeout <= 0 {
		return errors.New("browser plan timeouts must be positive durations")
	}
	if p.Method.Purpose != PurposeRepeatableE2E && p.Method.Purpose != PurposeBrowserControl {
		return errors.New("browser plan purpose is invalid")
	}
	switch p.Method.Method {
	case MethodPlaywrightTestSuite, MethodPlaywrightCLI, MethodPlaywrightCLINoSkill, MethodMCP, MethodHTTP:
	default:
		return errors.New("browser plan method is invalid")
	}
	if p.Method.Stage != "" && p.Method.Stage != StageBrowserUIValidation && p.Method.Stage != StageQAEndToEnd {
		return errors.New("browser plan stage is invalid")
	}
	if p.Method.Stage == StageBrowserUIValidation && p.Method.Purpose != PurposeBrowserControl {
		return errors.New("browser UI validation requires the browser-control purpose")
	}
	if p.Method.Stage == StageQAEndToEnd && p.Method.Purpose != PurposeRepeatableE2E {
		return errors.New("QA end-to-end requires the repeatable E2E purpose")
	}
	if p.Method.Method == MethodHTTP {
		if !p.IsHTTPOnlyE2E() {
			return errors.New("HTTP is valid only as an explicitly selected QA end-to-end method")
		}
		if p.Method.ToolName != "" || p.Method.ToolVersion != "" || len(p.Method.ToolVersionCommand) != 0 || p.Method.PlaywrightVersion != "" || p.Method.ExistingPlaywrightSuite || p.Method.OfficialCLISkillAvailable || p.Method.PersistentBrowserNeeded || p.Method.VerifiedCLICapabilityGap {
			return errors.New("HTTP-only QA end-to-end plan cannot declare browser tools or capabilities")
		}
	} else {
		if err := validatePlanText(p.Method.ToolName, p.Method.ToolVersion); err != nil {
			return errors.New("browser plan tool selection is incomplete")
		}
		if _, err := parseVersion(p.Method.ToolVersion); err != nil {
			return errors.New("browser plan tool version is invalid")
		}
		if _, err := parseVersion(p.Method.PlaywrightVersion); err != nil {
			return errors.New("browser plan has an invalid Playwright version")
		}
		if err := validateToolVersionCommand(p.Method.ToolName, p.Method.ToolVersionCommand); err != nil {
			return errors.New("browser plan tool version command is invalid")
		}
	}
	if p.Method.Purpose == PurposeRepeatableE2E && strings.TrimSpace(p.Execution.E2ECommand) == "" {
		return errors.New("repeatable E2E must declare its planned command")
	}
	if p.Authentication.Requirement != AuthenticationRequired && p.Authentication.Requirement != AuthenticationNotNeeded {
		return errors.New("browser plan authentication requirement is unresolved")
	}
	switch p.Authentication.Requirement {
	case AuthenticationRequired:
		if p.Authentication.Flow != AuthenticationForm && p.Authentication.Flow != AuthenticationMFA && p.Authentication.Flow != AuthenticationCAPTCHA && p.Authentication.Flow != AuthenticationSSO {
			return errors.New("browser plan must declare its authentication flow")
		}
	case AuthenticationNotNeeded:
		if p.Authentication.Flow != AuthenticationNone {
			return errors.New("browser plan without login must declare the none flow")
		}
		if p.Authentication.Login != (LoginFormRecipe{}) {
			return errors.New("browser plan without login cannot declare login locators")
		}
	}

	for _, fixture := range p.Execution.Fixtures {
		if err := validatePlanText(fixture); err != nil {
			return errors.New("browser plan fixture is invalid")
		}
	}
	for _, evidencePath := range p.Execution.EvidencePaths {
		if err := validatePlanText(evidencePath); err != nil {
			return errors.New("browser plan evidence path is invalid")
		}
	}
	seen := make(map[string]struct{}, len(p.Coverage))
	allowedOrigins := make(map[string]struct{}, len(p.Execution.ApprovedOrigins))
	for _, raw := range p.Execution.ApprovedOrigins {
		origin, err := normalizeOrigin(raw, true)
		if err != nil {
			return errors.New("browser plan approved origin is invalid")
		}
		if _, exists := allowedOrigins[origin]; exists {
			return errors.New("browser plan contains a duplicate approved origin")
		}
		allowedOrigins[origin] = struct{}{}
	}
	if len(allowedOrigins) == 0 {
		return errors.New("browser plan must declare approved origins")
	}
	baseOrigin, err := normalizeOrigin(p.Execution.BaseURL, false)
	if err != nil {
		return errors.New("browser plan base URL is invalid")
	}
	if _, ok := allowedOrigins[baseOrigin]; !ok {
		return errors.New("browser plan base URL is outside its approved origins")
	}
	for _, item := range p.Coverage {
		if !coverageIDPattern.MatchString(item.ID) {
			return errors.New("browser plan contains an invalid coverage ID")
		}
		if _, exists := seen[item.ID]; exists {
			return errors.New("browser plan contains a duplicate coverage ID")
		}
		seen[item.ID] = struct{}{}
		if err := validatePlanText(item.RequirementRef, item.ScreenOrJourney, item.Profile, item.ExpectedResult); err != nil {
			return errors.New("browser plan coverage metadata is incomplete or invalid")
		}
		if p.Authentication.Requirement == AuthenticationRequired {
			if _, err := ValidateUserID(item.UserID); err != nil {
				return errors.New("browser plan coverage has an invalid selected user ID")
			}
		} else if item.UserID != "" || item.Profile != "anonymous" || item.ProtectedTarget.ExpectedRole != "anonymous" {
			return errors.New("browser plan without login must use the anonymous profile")
		}
		if len(item.EvidenceRequirements) == 0 && item.Mandatory {
			return errors.New("mandatory browser coverage must declare evidence requirements")
		}
		for _, requirement := range item.EvidenceRequirements {
			if err := validatePlanText(requirement); err != nil {
				return errors.New("browser plan evidence requirement is invalid")
			}
		}
		seenWidths := make(map[int]struct{}, len(item.OptionalReferenceWidths))
		for _, width := range item.OptionalReferenceWidths {
			if width != 1280 && width != 768 && width != 375 {
				return errors.New("browser plan contains an unsupported optional reference width")
			}
			if _, exists := seenWidths[width]; exists {
				return errors.New("browser plan contains a duplicate optional reference width")
			}
			seenWidths[width] = struct{}{}
		}
		if item.ProtectedTarget.ExpectedAccess != AccessAllowed && item.ProtectedTarget.ExpectedAccess != AccessDenied {
			return errors.New("browser plan coverage must declare its protected-target expectation")
		}
		targetOrigin, err := normalizeOrigin(item.ProtectedTarget.URL, false)
		if err != nil {
			return errors.New("browser plan protected target is invalid")
		}
		if _, ok := allowedOrigins[targetOrigin]; !ok {
			return errors.New("browser plan protected target is outside its approved origins")
		}
		if p.Authentication.Requirement == AuthenticationRequired && p.Authentication.Flow == AuthenticationForm {
			if err := validatePlanText(p.Authentication.Login.EntryURL, p.Authentication.Login.LoginLocator, p.Authentication.Login.PasswordLocator, p.Authentication.Login.SubmitLocator); err != nil {
				return errors.New("browser plan form-login recipe is incomplete")
			}
			loginOrigin, err := normalizeOrigin(p.Authentication.Login.EntryURL, false)
			if err != nil {
				return errors.New("browser plan login destination is invalid")
			}
			if _, ok := allowedOrigins[loginOrigin]; !ok {
				return errors.New("browser plan login destination is outside its approved origins")
			}
		}
	}
	if p.Method.Method != MethodHTTP {
		for _, item := range p.Coverage {
			recipe, err := p.recipeForCoverage(item)
			if err != nil {
				return err
			}
			if recipe.Authentication == AuthenticationForm || recipe.Authentication == AuthenticationNone {
				if err := validateRecipe(recipe); err != nil {
					return errors.New("browser plan execution recipe is invalid")
				}
			}
		}
	}
	return nil
}

// IsHTTPOnlyE2E reports whether this plan explicitly selects the non-browser
// HTTP mode for QA end-to-end. That mode is schema-only; Hero has no HTTP
// executor and must not use it as a browser-validation substitute.
func (p BrowserPlan) IsHTTPOnlyE2E() bool {
	return p.Method.Stage == StageQAEndToEnd && p.Method.Purpose == PurposeRepeatableE2E && p.Method.Method == MethodHTTP
}

// RecipeForCoverage resolves one coverage row into the private executor input.
// The user/profile and all application-specific values come from Planning.
func (p BrowserPlan) RecipeForCoverage(coverageID string) (Recipe, error) {
	if err := p.Validate(); err != nil {
		return Recipe{}, err
	}
	if p.IsHTTPOnlyE2E() {
		return Recipe{}, errors.New("HTTP-only QA end-to-end plans do not have a browser executor")
	}
	for _, item := range p.Coverage {
		if item.ID == coverageID {
			return p.recipeForCoverage(item)
		}
	}
	return Recipe{}, errors.New("browser plan coverage ID is unknown")
}

// readinessRecipeForCoverage carries only the service URL for the explicitly
// planned HTTP-only QA End-to-End path. It does not construct a browser recipe,
// select credentials, or imply browser capability.
func (p BrowserPlan) readinessRecipeForCoverage(coverageID string) (Recipe, error) {
	if err := p.Validate(); err != nil {
		return Recipe{}, err
	}
	if !p.IsHTTPOnlyE2E() {
		return Recipe{}, errors.New("HTTP readiness is valid only for explicitly planned QA End-to-End")
	}
	for _, item := range p.Coverage {
		if item.ID == coverageID {
			return Recipe{
				Stage:       p.Method.Stage,
				Environment: p.Execution.Environment,
				BaseURL:     p.Execution.BaseURL,
				CoverageIDs: []string{item.ID},
				Purpose:     p.Method.Purpose,
				Method:      p.Method.Method,
			}, nil
		}
	}
	return Recipe{}, errors.New("browser plan coverage ID is unknown")
}

func (p BrowserPlan) recipeForCoverage(item CoverageItem) (Recipe, error) {
	actionTimeout, err := time.ParseDuration(p.Execution.ActionTimeout)
	if err != nil || actionTimeout <= 0 {
		return Recipe{}, errors.New("browser plan action timeout must be positive")
	}
	testTimeout, err := time.ParseDuration(p.Execution.TestTimeout)
	if err != nil || testTimeout <= 0 {
		return Recipe{}, errors.New("browser plan test timeout must be positive")
	}
	recipe := Recipe{
		Stage:                     p.Method.Stage,
		Environment:               p.Execution.Environment,
		BaseURL:                   p.Execution.BaseURL,
		ApprovedOrigins:           append([]string(nil), p.Execution.ApprovedOrigins...),
		Authentication:            p.Authentication.Flow,
		Login:                     p.Authentication.Login,
		ProtectedTarget:           item.ProtectedTarget,
		UserID:                    item.UserID,
		UserProfile:               item.Profile,
		CoverageIDs:               []string{item.ID},
		CoverageRequirement:       item.RequirementRef,
		CoverageScreenJourney:     item.ScreenOrJourney,
		CoverageMandatory:         item.Mandatory,
		CoverageExpectedResult:    item.ExpectedResult,
		EvidenceRequirements:      append([]string(nil), item.EvidenceRequirements...),
		Purpose:                   p.Method.Purpose,
		Method:                    p.Method.Method,
		ToolName:                  p.Method.ToolName,
		ToolVersion:               p.Method.ToolVersion,
		ToolVersionCommand:        append([]string(nil), p.Method.ToolVersionCommand...),
		PlaywrightVersion:         p.Method.PlaywrightVersion,
		ExistingPlaywrightSuite:   p.Method.ExistingPlaywrightSuite,
		OfficialCLISkillAvailable: p.Method.OfficialCLISkillAvailable,
		PersistentBrowserNeeded:   p.Method.PersistentBrowserNeeded,
		VerifiedCLICapabilityGap:  p.Method.VerifiedCLICapabilityGap,
		StartCommand:              p.Execution.StartCommand,
		ReadinessCommand:          p.Execution.ReadinessCommand,
		E2ECommand:                p.Execution.E2ECommand,
		ActionTimeout:             actionTimeout,
		TestTimeout:               testTimeout,
		Fixtures:                  append([]string(nil), p.Execution.Fixtures...),
		EvidencePaths:             append([]string(nil), p.Execution.EvidencePaths...),
		UnrestrictedHarness:       p.Execution.UnrestrictedHarness,
	}
	if p.Authentication.Requirement == AuthenticationNotNeeded && recipe.UserProfile == "" {
		recipe.UserProfile = "anonymous"
	}
	if recipe.Authentication == AuthenticationForm || recipe.Authentication == AuthenticationNone {
		if err := validateRecipe(recipe); err != nil {
			return Recipe{}, errors.New("browser plan execution recipe is invalid")
		}
	}
	return recipe, nil
}

func validateToolVersionCommand(toolName string, argv []string) error {
	if len(argv) != 2 || strings.TrimSpace(argv[0]) == "" || argv[1] != "--version" || hasControl(argv[0]) || hasControl(argv[1]) {
		return errors.New("version check must be one executable plus --version")
	}
	if argv[0] != strings.TrimSpace(toolName) || strings.ContainsAny(argv[0], `/\\$`+"`|;&<>\n\r") {
		return errors.New("version command executable must match the selected tool")
	}
	return nil
}

func validatePlanText(values ...string) error {
	for _, value := range values {
		if strings.TrimSpace(value) == "" || hasControl(value) {
			return errors.New("invalid browser plan text")
		}
	}
	return nil
}

func decodeBrowserPlan(data []byte) (BrowserPlan, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return BrowserPlan{}, errors.New("browser-plan.json is empty")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return BrowserPlan{}, errors.New("browser-plan.json has ambiguous JSON keys")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var plan BrowserPlan
	if err := decoder.Decode(&plan); err != nil {
		return BrowserPlan{}, errors.New("browser-plan.json does not match the shared schema")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return BrowserPlan{}, errors.New("browser-plan.json must contain exactly one JSON object")
	}
	return plan, nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("extra JSON value")
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("invalid JSON object key")
			}
			normalized := strings.ToLower(key)
			if _, exists := seen[normalized]; exists {
				return errors.New("duplicate JSON object key")
			}
			seen[normalized] = struct{}{}
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("unterminated JSON object")
		}
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("unterminated JSON array")
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	return nil
}
