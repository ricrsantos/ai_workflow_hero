package testaccess

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

const (
	// DefaultPreparationLimit bounds one full prerequisite-preparation attempt.
	DefaultPreparationLimit = 120 * time.Second
	// MaxPrerequisiteAttempts limits bounded retries within one preparation attempt.
	MaxPrerequisiteAttempts = 2
)

// PreparationPrerequisite names a documented prerequisite checked before
// browser validation can begin.
type PreparationPrerequisite string

const (
	PrerequisiteServiceReadiness  PreparationPrerequisite = "service_readiness"
	PrerequisiteBrowserPermission PreparationPrerequisite = "browser_permission"
	PrerequisiteSelectedAccount   PreparationPrerequisite = "selected_account"
	PrerequisiteProtectedAccess   PreparationPrerequisite = "protected_access"
	PrerequisiteFixtures          PreparationPrerequisite = "fixtures"
	PrerequisiteMethodAdmission   PreparationPrerequisite = "method_admission"
)

var preparationPrerequisites = [...]PreparationPrerequisite{
	PrerequisiteServiceReadiness,
	PrerequisiteBrowserPermission,
	PrerequisiteSelectedAccount,
	PrerequisiteFixtures,
	PrerequisiteMethodAdmission,
}

// PreparationReason is a stable, sanitized reason for a prerequisite block.
type PreparationReason string

const (
	PreparationReasonServiceUnavailable    PreparationReason = "service_unavailable"
	PreparationReasonBrowserPermission     PreparationReason = "browser_permission_unavailable"
	PreparationReasonSelectedAccount       PreparationReason = "selected_account_unavailable"
	PreparationReasonProtectedAccess       PreparationReason = "protected_access_unverified"
	PreparationReasonFixturesUnavailable   PreparationReason = "fixtures_unavailable"
	PreparationReasonMethodUnavailable     PreparationReason = "method_unavailable"
	PreparationReasonToolUnavailable       PreparationReason = "tool_unavailable"
	PreparationReasonToolVersion           PreparationReason = "tool_version_incompatible"
	PreparationReasonUnsupportedAuth       PreparationReason = "unsupported_authentication"
	PreparationReasonInvalidRecipe         PreparationReason = "invalid_recipe"
	PreparationReasonTestAccessDisabled    PreparationReason = "test_access_disabled"
	PreparationReasonAttemptTimedOut       PreparationReason = "preparation_attempt_timed_out"
	PreparationReasonStageBudgetExpired    PreparationReason = "stage_budget_exhausted"
	PreparationReasonAttemptLimit          PreparationReason = "prerequisite_attempt_limit"
	PreparationReasonInterrupted           PreparationReason = "preparation_interrupted"
	PreparationReasonCapabilityUnavailable PreparationReason = "preparation_capability_unavailable"
	PreparationReasonCheckFailed           PreparationReason = "prerequisite_check_failed"
)

// PrerequisiteObservation contains only stable status and reason codes. It
// deliberately has no free-text field that could copy secrets or page data.
type PrerequisiteObservation struct {
	Ready        bool
	Reason       PreparationReason
	Retryable    bool
	Capabilities *RunnerCapabilities
}

// PreparationCapability checks local pre-dispatch prerequisites from the
// non-secret recipe and must honor context cancellation. Selected-account
// checks use testaccess's private credential boundary and return status only;
// protected-role verification stays in the dispatched stage session.
type PreparationCapability interface {
	CheckPrerequisite(context.Context, Recipe, PreparationPrerequisite) PrerequisiteObservation
}

// BrowserMethodAdmitter verifies the planned browser method in the active
// stage session. It must probe the exact method passed in Recipe, perform no
// provisioning, and return only observed capability flags and stable reasons.
type BrowserMethodAdmitter interface {
	AdmitMethod(context.Context, Recipe) PrerequisiteObservation
}

// PlannedMethodAdmitter performs deterministic, pre-dispatch admission of the
// Planning-selected tool and method. It must not claim live browser-session
// properties; the dispatched stage agent still proves those in its report.
type PlannedMethodAdmitter interface {
	AdmitPlannedMethod(context.Context, Recipe) PrerequisiteObservation
}

// PreparationTimer is the timer surface used to make preparation deadlines
// deterministic in tests.
type PreparationTimer interface {
	C() <-chan time.Time
	Stop() bool
}

// PreparationClock supplies the timer and time source for one bounded attempt.
type PreparationClock interface {
	Now() time.Time
	NewTimer(time.Duration) PreparationTimer
}

// BoundedPreparationResult reports sanitized preparation state. It contains
// no commands, URLs, account values, tool output, or arbitrary diagnostics.
type BoundedPreparationResult struct {
	Status     PreparationStatus               `json:"status"`
	Checked    []PreparationPrerequisite       `json:"checked,omitempty"`
	Attempts   map[PreparationPrerequisite]int `json:"attempts,omitempty"`
	BlockedOn  PreparationPrerequisite         `json:"blocked_on,omitempty"`
	Reason     PreparationReason               `json:"reason,omitempty"`
	NextAction string                          `json:"next_action,omitempty"`
	TimeLimit  time.Duration                   `json:"time_limit,omitempty"`
}

// Preparer applies the C17 preparation ceiling and prerequisite retry bound.
// Each call is one explicit preparation attempt; the caller supplies the
// current remaining stage budget so retries cannot reset that budget.
type Preparer struct {
	capability PreparationCapability
	clock      PreparationClock
	logger     *slog.Logger
}

// NewPreparer creates a bounded prerequisite preparer. A nil clock uses the
// wall clock; a nil capability fails closed without running checks.
func NewPreparer(capability PreparationCapability, clock PreparationClock, logger *slog.Logger) *Preparer {
	if clock == nil {
		clock = wallPreparationClock{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Preparer{capability: capability, clock: clock, logger: logger}
}

// Prepare checks documented prerequisites under one 120-second ceiling capped
// by remainingStageBudget. A retry occurs only when the trusted checker marks
// that specific outcome retryable; there are no sleeps, provisioning, installs,
// or transport fallbacks in this layer.
func (p *Preparer) Prepare(ctx context.Context, recipe Recipe, remainingStageBudget time.Duration) BoundedPreparationResult {
	result := BoundedPreparationResult{Status: PreparationBlocked}
	if ctx == nil || ctx.Err() != nil {
		return p.finish(result, PreparationReasonInterrupted, "", "Retry preparation explicitly after confirming the stage is active.")
	}
	if p == nil || p.capability == nil || p.clock == nil {
		return p.finish(result, PreparationReasonCapabilityUnavailable, "", "Connect the trusted preparation capability to the active stage session, then run /hero-continue.")
	}
	httpOnlyE2E := recipe.Stage == StageQAEndToEnd && recipe.Purpose == PurposeRepeatableE2E && recipe.Method == MethodHTTP
	if httpOnlyE2E {
		if !validHTTPReadinessRecipe(recipe) {
			return p.finish(result, PreparationReasonInvalidRecipe, "", preparationAction(PreparationReasonInvalidRecipe))
		}
	} else if reason := recipeBlockReason(recipe); reason != "" {
		return p.finish(result, preparationReasonForExecutorBlock(reason), "", preparationAction(preparationReasonForExecutorBlock(reason)))
	}
	if remainingStageBudget <= 0 {
		return p.finish(result, PreparationReasonStageBudgetExpired, "", preparationAction(PreparationReasonStageBudgetExpired))
	}
	limit := DefaultPreparationLimit
	if remainingStageBudget < limit {
		limit = remainingStageBudget
	}
	result.TimeLimit = limit
	start := p.clock.Now()
	checkCtx, cancel := withPreparationLimit(ctx, p.clock, limit)
	defer cancel()

	prerequisites := preparationPrerequisites[:]
	if httpOnlyE2E {
		// HTTP is admitted only as the explicit QA End-to-End plan mode. It
		// checks service readiness and never falls back from browser mode.
		prerequisites = []PreparationPrerequisite{PrerequisiteServiceReadiness}
	}
	result.Attempts = make(map[PreparationPrerequisite]int, len(prerequisites))
	for _, prerequisite := range prerequisites {
		ready := false
		for attempt := 1; attempt <= MaxPrerequisiteAttempts; attempt++ {
			if reason := preparationContextReason(ctx, checkCtx, p.clock, start, limit, remainingStageBudget); reason != "" {
				return p.finish(result, reason, prerequisite, preparationAction(reason))
			}
			result.Attempts[prerequisite] = attempt
			if p.logger != nil {
				p.logger.Debug("test-access prerequisite check", "prerequisite", prerequisite, "attempt", attempt)
			}
			var observation PrerequisiteObservation
			if prerequisite == PrerequisiteMethodAdmission {
				if admitter, ok := p.capability.(PlannedMethodAdmitter); ok {
					observation = admitter.AdmitPlannedMethod(checkCtx, recipe)
					if observation.Ready {
						if observation.Capabilities == nil {
							observation.Ready = false
							observation.Reason = PreparationReasonCapabilityUnavailable
						} else if reason := plannedCapabilityBlockReason(recipe, *observation.Capabilities); reason != "" {
							observation.Ready = false
							observation.Reason = preparationReasonForExecutorBlock(reason)
						}
					}
				} else {
					admitter, ok := p.capability.(BrowserMethodAdmitter)
					if !ok {
						return p.finish(result, PreparationReasonMethodUnavailable, prerequisite, preparationAction(PreparationReasonMethodUnavailable))
					}
					observation = admitter.AdmitMethod(checkCtx, recipe)
					if observation.Ready {
						if observation.Capabilities == nil {
							observation.Ready = false
							observation.Reason = PreparationReasonCapabilityUnavailable
						} else if reason := capabilityBlockReason(recipe, *observation.Capabilities); reason != "" {
							observation.Ready = false
							observation.Reason = preparationReasonForExecutorBlock(reason)
						}
					}
				}
			} else {
				observation = p.capability.CheckPrerequisite(checkCtx, recipe, prerequisite)
			}
			if reason := preparationContextReason(ctx, checkCtx, p.clock, start, limit, remainingStageBudget); reason != "" {
				return p.finish(result, reason, prerequisite, preparationAction(reason))
			}
			if observation.Ready {
				ready = true
				break
			}
			reason := validPreparationReason(observation.Reason)
			if reason == "" {
				reason = defaultReasonForPrerequisite(prerequisite)
			}
			if !observation.Retryable || attempt == MaxPrerequisiteAttempts {
				action := preparationAction(reason)
				if observation.Retryable && attempt == MaxPrerequisiteAttempts {
					action = "This prerequisite remained unavailable after two bounded checks. " + action
				}
				return p.finish(result, reason, prerequisite, action)
			}
		}
		if !ready {
			reason := defaultReasonForPrerequisite(prerequisite)
			return p.finish(result, reason, prerequisite, preparationAction(reason))
		}
		result.Checked = append(result.Checked, prerequisite)
	}
	if reason := preparationContextReason(ctx, checkCtx, p.clock, start, limit, remainingStageBudget); reason != "" {
		return p.finish(result, reason, "", preparationAction(reason))
	}
	result.Status = PreparationReady
	return p.logResult(result)
}

// PreparePlan reloads the current Planning artifact and applies the current
// Config opt-in before checking prerequisites. Callers should pass the current
// workflow-config value on every explicit continue/retry.
func (p *Preparer) PreparePlan(ctx context.Context, projectDir, coverageID string, testAccessEnabled bool, remainingStageBudget time.Duration) BoundedPreparationResult {
	plan, err := LoadBrowserPlan(projectDir)
	if err != nil {
		return p.finish(BoundedPreparationResult{Status: PreparationBlocked}, PreparationReasonInvalidRecipe, "", "Create or correct .workflow-hero/cycles/current/browser-plan.json during Planning, then run /hero-continue.")
	}
	var selected *CoverageItem
	for i := range plan.Coverage {
		if plan.Coverage[i].ID == coverageID {
			selected = &plan.Coverage[i]
			break
		}
	}
	if selected == nil {
		return p.finish(BoundedPreparationResult{Status: PreparationBlocked}, PreparationReasonInvalidRecipe, "", "Update Planning with the requested coverage ID, then run /hero-continue.")
	}
	if !plan.IsHTTPOnlyE2E() && plan.Authentication.Requirement == AuthenticationRequired && !testAccessEnabled {
		return p.finish(BoundedPreparationResult{Status: PreparationBlocked}, PreparationReasonTestAccessDisabled, PrerequisiteSelectedAccount, "Enable Config → Test users and configure the selected account in project-root .env.hero, then run /hero-continue.")
	}
	var recipe Recipe
	if plan.IsHTTPOnlyE2E() {
		recipe, err = plan.readinessRecipeForCoverage(coverageID)
	} else {
		recipe, err = plan.RecipeForCoverage(coverageID)
	}
	if err != nil {
		return p.finish(BoundedPreparationResult{Status: PreparationBlocked}, PreparationReasonInvalidRecipe, "", "Correct the non-secret browser plan during Planning, then run /hero-continue.")
	}
	return p.Prepare(ctx, recipe, remainingStageBudget)
}

func validHTTPReadinessRecipe(recipe Recipe) bool {
	return recipe.Stage == StageQAEndToEnd && recipe.Purpose == PurposeRepeatableE2E && recipe.Method == MethodHTTP && strings.TrimSpace(recipe.BaseURL) != ""
}

func plannedCapabilityBlockReason(recipe Recipe, capabilities RunnerCapabilities) BlockReason {
	if !capabilities.ToolAvailable || !strings.EqualFold(strings.TrimSpace(capabilities.ToolName), strings.TrimSpace(recipe.ToolName)) {
		return BlockToolUnavailable
	}
	if !capabilities.MethodSupported || capabilities.Method != recipe.Method {
		return BlockMethodUnavailable
	}
	actual, err := parseVersion(capabilities.PlaywrightVersion)
	if err != nil || compareVersion(actual, minimumVersion()) < 0 {
		return BlockToolVersion
	}
	return ""
}

func (p *Preparer) finish(result BoundedPreparationResult, reason PreparationReason, prerequisite PreparationPrerequisite, action string) BoundedPreparationResult {
	result.Status = PreparationBlocked
	result.Reason = reason
	result.BlockedOn = prerequisite
	result.NextAction = action
	return p.logResult(result)
}

func (p *Preparer) logResult(result BoundedPreparationResult) BoundedPreparationResult {
	if p == nil || p.logger == nil {
		return result
	}
	attrs := []any{"status", result.Status}
	if result.BlockedOn != "" {
		attrs = append(attrs, "prerequisite", result.BlockedOn)
	}
	if result.Reason != "" {
		attrs = append(attrs, "reason", result.Reason)
	}
	if result.TimeLimit > 0 {
		attrs = append(attrs, "time_limit_ms", result.TimeLimit.Milliseconds())
	}
	if result.Status == PreparationBlocked {
		p.logger.Error("test-access preparation blocked", attrs...)
	} else {
		p.logger.Info("test-access preparation completed", attrs...)
	}
	return result
}

func withPreparationLimit(parent context.Context, clock PreparationClock, limit time.Duration) (context.Context, context.CancelFunc) {
	base, cancel := context.WithCancel(parent)
	now := clock.Now()
	deadline := now.Add(limit)
	if parentDeadline, ok := parent.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	delay := deadline.Sub(now)
	if delay < 0 {
		delay = 0
	}
	timer := clock.NewTimer(delay)
	go func() {
		select {
		case <-timer.C():
			cancel()
		case <-base.Done():
			timer.Stop()
		}
	}()
	return preparationDeadlineContext{Context: base, deadline: deadline}, func() {
		timer.Stop()
		cancel()
	}
}

type preparationDeadlineContext struct {
	context.Context
	deadline time.Time
}

func (c preparationDeadlineContext) Deadline() (time.Time, bool) { return c.deadline, true }

func preparationContextReason(parent, attempt context.Context, clock PreparationClock, start time.Time, limit, remaining time.Duration) PreparationReason {
	if parent.Err() != nil {
		return PreparationReasonInterrupted
	}
	elapsed := clock.Now().Sub(start)
	if elapsed >= limit || attempt.Err() != nil {
		if remaining < DefaultPreparationLimit {
			return PreparationReasonStageBudgetExpired
		}
		return PreparationReasonAttemptTimedOut
	}
	return ""
}

func preparationReasonForExecutorBlock(reason BlockReason) PreparationReason {
	switch reason {
	case BlockUnsupportedAuth:
		return PreparationReasonUnsupportedAuth
	case BlockMethodPreference, BlockMethodUnavailable, BlockContextUnavailable:
		return PreparationReasonMethodUnavailable
	case BlockToolUnavailable:
		return PreparationReasonToolUnavailable
	case BlockToolVersion:
		return PreparationReasonToolVersion
	case BlockInvalidRecipe, BlockUnsafeRecipe:
		return PreparationReasonInvalidRecipe
	default:
		return PreparationReasonCheckFailed
	}
}

func validPreparationReason(reason PreparationReason) PreparationReason {
	switch reason {
	case PreparationReasonServiceUnavailable,
		PreparationReasonBrowserPermission,
		PreparationReasonSelectedAccount,
		PreparationReasonProtectedAccess,
		PreparationReasonFixturesUnavailable,
		PreparationReasonMethodUnavailable,
		PreparationReasonToolUnavailable,
		PreparationReasonToolVersion,
		PreparationReasonUnsupportedAuth,
		PreparationReasonInvalidRecipe,
		PreparationReasonTestAccessDisabled,
		PreparationReasonAttemptTimedOut,
		PreparationReasonStageBudgetExpired,
		PreparationReasonAttemptLimit,
		PreparationReasonInterrupted,
		PreparationReasonCapabilityUnavailable,
		PreparationReasonCheckFailed:
		return reason
	default:
		return ""
	}
}

func defaultReasonForPrerequisite(prerequisite PreparationPrerequisite) PreparationReason {
	switch prerequisite {
	case PrerequisiteServiceReadiness:
		return PreparationReasonServiceUnavailable
	case PrerequisiteBrowserPermission:
		return PreparationReasonBrowserPermission
	case PrerequisiteSelectedAccount:
		return PreparationReasonSelectedAccount
	case PrerequisiteProtectedAccess:
		return PreparationReasonProtectedAccess
	case PrerequisiteFixtures:
		return PreparationReasonFixturesUnavailable
	case PrerequisiteMethodAdmission:
		return PreparationReasonMethodUnavailable
	default:
		return PreparationReasonCheckFailed
	}
}

func preparationAction(reason PreparationReason) string {
	switch reason {
	case PreparationReasonServiceUnavailable:
		return "Start or repair the planned service outside QA, then run /hero-continue to recheck readiness."
	case PreparationReasonBrowserPermission:
		return "Install or enable the selected harness outside QA, then run /hero-continue. Browser permission is verified in the active stage session."
	case PreparationReasonSelectedAccount:
		return "Enable Config → Test users and configure the selected account in project-root .env.hero, then run /hero-continue."
	case PreparationReasonProtectedAccess:
		return "Verify the selected account's planned protected role and target, correct the account or plan, then run /hero-continue."
	case PreparationReasonFixturesUnavailable:
		return "Make the declared fixture available through the project plan before QA, then run /hero-continue. QA will not provision it."
	case PreparationReasonMethodUnavailable:
		return "Enable the planned browser method and same-context capability in the active stage session, then run /hero-continue. HTTP cannot replace browser validation."
	case PreparationReasonToolUnavailable:
		return "Install or enable the planned browser tool outside QA, then run /hero-continue. Hero will not install or upgrade tools during QA."
	case PreparationReasonToolVersion:
		return "Use a compatible Playwright version (at least 1.63.0) and update the planned tool version before /hero-continue."
	case PreparationReasonTestAccessDisabled:
		return "Enable Config → Test users and configure the selected account in project-root .env.hero, then run /hero-continue."
	case PreparationReasonUnsupportedAuth:
		return "Plan the supported login/password form flow; interactive MFA, CAPTCHA and SSO require a different supported setup."
	case PreparationReasonAttemptTimedOut:
		return "The 120-second preparation attempt expired. Resolve the reported prerequisite and run /hero-continue while stage budget remains."
	case PreparationReasonStageBudgetExpired:
		return "Stage budget is exhausted. Increase the remaining stage budget explicitly before running /hero-continue."
	case PreparationReasonAttemptLimit:
		return "The prerequisite reached its two-check limit for this preparation attempt. Resolve it outside QA, then run /hero-continue."
	case PreparationReasonInterrupted:
		return "Preparation was interrupted. Confirm the stage is active and run /hero-continue explicitly."
	case PreparationReasonCapabilityUnavailable:
		return "Connect the trusted preparation capability to the active stage session before retrying."
	case PreparationReasonInvalidRecipe:
		return "Correct the non-secret browser plan and selected method before running /hero-continue."
	default:
		return "Review the planned prerequisite configuration, then run /hero-continue."
	}
}

type wallPreparationClock struct{}

func (wallPreparationClock) Now() time.Time { return time.Now() }
func (wallPreparationClock) NewTimer(delay time.Duration) PreparationTimer {
	return wallPreparationTimer{timer: time.NewTimer(delay)}
}

type wallPreparationTimer struct{ timer *time.Timer }

func (t wallPreparationTimer) C() <-chan time.Time { return t.timer.C }
func (t wallPreparationTimer) Stop() bool          { return t.timer.Stop() }
