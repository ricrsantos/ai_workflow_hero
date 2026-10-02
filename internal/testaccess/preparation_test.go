package testaccess

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakePreparationClock struct {
	mu     sync.Mutex
	now    time.Time
	timers map[*fakePreparationTimer]struct{}
}

type fakePreparationTimer struct {
	clock   *fakePreparationClock
	due     time.Time
	channel chan time.Time
	fired   bool
}

func newFakePreparationClock(start time.Time) *fakePreparationClock {
	return &fakePreparationClock{now: start, timers: make(map[*fakePreparationTimer]struct{})}
}

func (c *fakePreparationClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakePreparationClock) NewTimer(delay time.Duration) PreparationTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	timer := &fakePreparationTimer{clock: c, due: c.now.Add(delay), channel: make(chan time.Time, 1)}
	if delay <= 0 {
		timer.fired = true
		timer.channel <- c.now
		return timer
	}
	c.timers[timer] = struct{}{}
	return timer
}

func (c *fakePreparationClock) Advance(delay time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(delay)
	var ready []*fakePreparationTimer
	for timer := range c.timers {
		if !timer.due.After(c.now) {
			delete(c.timers, timer)
			timer.fired = true
			ready = append(ready, timer)
		}
	}
	now := c.now
	c.mu.Unlock()
	for _, timer := range ready {
		timer.channel <- now
	}
}

func (t *fakePreparationTimer) C() <-chan time.Time { return t.channel }

func (t *fakePreparationTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.fired {
		return false
	}
	delete(t.clock.timers, t)
	t.fired = true
	return true
}

type preparationCall struct {
	prerequisite PreparationPrerequisite
	method       ExecutionMethod
	coverageIDs  []string
}

type fakePreparationCapability struct {
	mu             sync.Mutex
	outcomes       map[PreparationPrerequisite][]PrerequisiteObservation
	calls          []preparationCall
	onCheck        func(context.Context, Recipe, PreparationPrerequisite)
	methodCaps     *RunnerCapabilities
	provisionCalls int
}

func (c *fakePreparationCapability) CheckPrerequisite(ctx context.Context, recipe Recipe, prerequisite PreparationPrerequisite) PrerequisiteObservation {
	c.mu.Lock()
	callIndex := 0
	for _, call := range c.calls {
		if call.prerequisite == prerequisite {
			callIndex++
		}
	}
	c.calls = append(c.calls, preparationCall{prerequisite: prerequisite, method: recipe.Method, coverageIDs: append([]string(nil), recipe.CoverageIDs...)})
	onCheck := c.onCheck
	outcomes := append([]PrerequisiteObservation(nil), c.outcomes[prerequisite]...)
	c.mu.Unlock()
	if onCheck != nil {
		onCheck(ctx, recipe, prerequisite)
	}
	if callIndex < len(outcomes) {
		return outcomes[callIndex]
	}
	return PrerequisiteObservation{Ready: true}
}

func (c *fakePreparationCapability) AdmitMethod(ctx context.Context, recipe Recipe) PrerequisiteObservation {
	observation := c.CheckPrerequisite(ctx, recipe, PrerequisiteMethodAdmission)
	if !observation.Ready {
		return observation
	}
	if c.methodCaps != nil {
		capabilities := *c.methodCaps
		observation.Capabilities = &capabilities
		return observation
	}
	playwrightVersion := recipe.PlaywrightVersion
	if playwrightVersion == "" {
		playwrightVersion = MinimumPlaywrightVersion
	}
	observation.Capabilities = &RunnerCapabilities{
		ToolAvailable:                 true,
		ToolName:                      recipe.ToolName,
		ToolVersion:                   recipe.ToolVersion,
		PlaywrightVersion:             playwrightVersion,
		Method:                        recipe.Method,
		MethodSupported:               true,
		FreshIsolatedContext:          true,
		SameContextLoginAndTests:      true,
		PrivateCredentialChannel:      true,
		ApprovedOriginsEnforced:       true,
		AuthStateMemoryOnly:           true,
		CredentialArtifactSuppression: true,
	}
	return observation
}

func (c *fakePreparationCapability) snapshotCalls() []preparationCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]preparationCall(nil), c.calls...)
}

func preparationLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPreparationChecksDocumentedPrerequisitesWithinOneAttempt(t *testing.T) {
	clock := newFakePreparationClock(time.Unix(1_800_000_000, 0))
	capability := &fakePreparationCapability{}
	preparer := NewPreparer(capability, clock, preparationLogger())
	result := preparer.Prepare(context.Background(), validExecutorRecipe(), 5*time.Minute)

	if result.Status != PreparationReady {
		t.Fatalf("Prepare() status=%s reason=%s, want ready", result.Status, result.Reason)
	}
	if result.TimeLimit != DefaultPreparationLimit {
		t.Fatalf("Prepare() time limit=%s, want %s", result.TimeLimit, DefaultPreparationLimit)
	}
	if len(result.Checked) != len(preparationPrerequisites) {
		t.Fatalf("Prepare() checked %v, want all documented prerequisites", result.Checked)
	}
	calls := capability.snapshotCalls()
	if len(calls) != len(preparationPrerequisites) {
		t.Fatalf("capability calls=%d, want %d", len(calls), len(preparationPrerequisites))
	}
	for i, prerequisite := range preparationPrerequisites {
		if calls[i].prerequisite != prerequisite {
			t.Errorf("check %d=%s, want %s", i, calls[i].prerequisite, prerequisite)
		}
		if result.Attempts[prerequisite] != 1 {
			t.Errorf("attempts[%s]=%d, want 1", prerequisite, result.Attempts[prerequisite])
		}
	}
}

func TestPreparationDeadlineIsCappedByRemainingStageBudget(t *testing.T) {
	for _, test := range []struct {
		name      string
		remaining time.Duration
		wantLimit time.Duration
		wantCause PreparationReason
	}{
		{name: "default ceiling", remaining: 5 * time.Minute, wantLimit: DefaultPreparationLimit, wantCause: PreparationReasonAttemptTimedOut},
		{name: "stage budget cap", remaining: 17 * time.Second, wantLimit: 17 * time.Second, wantCause: PreparationReasonStageBudgetExpired},
	} {
		t.Run(test.name, func(t *testing.T) {
			start := time.Unix(1_800_000_000, 0)
			clock := newFakePreparationClock(start)
			capability := &fakePreparationCapability{onCheck: func(ctx context.Context, _ Recipe, _ PreparationPrerequisite) {
				deadline, ok := ctx.Deadline()
				if !ok || !deadline.Equal(start.Add(test.wantLimit)) {
					t.Errorf("check context deadline=%v, want %v", deadline, start.Add(test.wantLimit))
				}
				clock.Advance(test.wantLimit)
				<-ctx.Done()
			}}
			result := NewPreparer(capability, clock, preparationLogger()).Prepare(context.Background(), validExecutorRecipe(), test.remaining)
			if result.Status != PreparationBlocked || result.Reason != test.wantCause {
				t.Fatalf("Prepare() status=%s reason=%s, want blocked with %s", result.Status, result.Reason, test.wantCause)
			}
			if result.TimeLimit != test.wantLimit {
				t.Fatalf("Prepare() time limit=%s, want %s", result.TimeLimit, test.wantLimit)
			}
			if len(capability.snapshotCalls()) != 1 {
				t.Fatalf("checks continued after the deadline: %v", capability.snapshotCalls())
			}
		})
	}
}

func TestPreparationRetriesOnlyRetryablePrerequisiteAndStopsAtTwo(t *testing.T) {
	t.Run("retryable readiness succeeds on second bounded check", func(t *testing.T) {
		capability := &fakePreparationCapability{outcomes: map[PreparationPrerequisite][]PrerequisiteObservation{
			PrerequisiteServiceReadiness: {
				{Reason: PreparationReasonServiceUnavailable, Retryable: true},
				{Ready: true},
			},
		}}
		result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).Prepare(context.Background(), validExecutorRecipe(), time.Minute)
		if result.Status != PreparationReady || result.Attempts[PrerequisiteServiceReadiness] != 2 {
			t.Fatalf("Prepare() status=%s attempts=%d, want ready after exactly two readiness checks", result.Status, result.Attempts[PrerequisiteServiceReadiness])
		}
		if len(capability.snapshotCalls()) != len(preparationPrerequisites)+1 {
			t.Fatalf("capability calls=%v, want one retry and remaining checks", capability.snapshotCalls())
		}
	})

	t.Run("retryable blocker cannot exceed two checks", func(t *testing.T) {
		capability := &fakePreparationCapability{outcomes: map[PreparationPrerequisite][]PrerequisiteObservation{
			PrerequisiteServiceReadiness: {
				{Reason: PreparationReasonServiceUnavailable, Retryable: true},
				{Reason: PreparationReasonServiceUnavailable, Retryable: true},
			},
		}}
		result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).Prepare(context.Background(), validExecutorRecipe(), time.Minute)
		if result.Status != PreparationBlocked || result.BlockedOn != PrerequisiteServiceReadiness || result.Attempts[PrerequisiteServiceReadiness] != MaxPrerequisiteAttempts {
			t.Fatalf("Prepare() status=%s blocked_on=%s attempts=%d, want readiness blocked after two", result.Status, result.BlockedOn, result.Attempts[PrerequisiteServiceReadiness])
		}
		if !strings.Contains(result.NextAction, "after two bounded checks") {
			t.Fatalf("attempt limit was not included in the corrective action: %q", result.NextAction)
		}
		if len(capability.snapshotCalls()) != MaxPrerequisiteAttempts {
			t.Fatalf("checks exceeded the per-prerequisite limit: %v", capability.snapshotCalls())
		}
	})
}

func TestPreparationMethodAdmissionUsesSelectedMethodAndRejectsHTTPFallback(t *testing.T) {
	t.Run("HTTP is rejected before capability admission", func(t *testing.T) {
		capability := &fakePreparationCapability{}
		recipe := validExecutorRecipe()
		recipe.Method = MethodHTTP
		result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).Prepare(context.Background(), recipe, time.Minute)
		if result.Status != PreparationBlocked || result.Reason != PreparationReasonMethodUnavailable || len(capability.snapshotCalls()) != 0 {
			t.Fatalf("HTTP fallback was admitted or probed: result=%+v calls=%v", result, capability.snapshotCalls())
		}
	})

	t.Run("admission checks the planned method and blocks unavailable tool", func(t *testing.T) {
		capability := &fakePreparationCapability{outcomes: map[PreparationPrerequisite][]PrerequisiteObservation{
			PrerequisiteMethodAdmission: {{Reason: PreparationReasonToolUnavailable}},
		}}
		recipe := validExecutorRecipe()
		recipe.Method = MethodPlaywrightTestSuite
		result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).Prepare(context.Background(), recipe, time.Minute)
		if result.Status != PreparationBlocked || result.BlockedOn != PrerequisiteMethodAdmission || result.Reason != PreparationReasonToolUnavailable {
			t.Fatalf("unavailable selected method did not block: %+v", result)
		}
		calls := capability.snapshotCalls()
		last := calls[len(calls)-1]
		if last.prerequisite != PrerequisiteMethodAdmission || last.method != recipe.Method {
			t.Fatalf("method admission used %+v, want planned method %s", last, recipe.Method)
		}
		if !strings.Contains(result.NextAction, "outside QA") {
			t.Fatalf("missing setup instruction outside QA: %q", result.NextAction)
		}
	})
}

func TestMethodAdmissionVerifiesObservedMethodToolAndIsolation(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*RunnerCapabilities)
		want   PreparationReason
	}{
		{name: "different method", change: func(c *RunnerCapabilities) { c.Method = MethodMCP }, want: PreparationReasonMethodUnavailable},
		{name: "missing tool", change: func(c *RunnerCapabilities) { c.ToolAvailable = false }, want: PreparationReasonToolUnavailable},
		{name: "below minimum Playwright", change: func(c *RunnerCapabilities) { c.PlaywrightVersion = "1.62.9" }, want: PreparationReasonToolVersion},
		{name: "no isolated context", change: func(c *RunnerCapabilities) { c.FreshIsolatedContext = false }, want: PreparationReasonMethodUnavailable},
		{name: "no private credential channel", change: func(c *RunnerCapabilities) { c.PrivateCredentialChannel = false }, want: PreparationReasonMethodUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			recipe := validExecutorRecipe()
			capabilities := RunnerCapabilities{
				ToolAvailable:                 true,
				ToolName:                      recipe.ToolName,
				ToolVersion:                   recipe.ToolVersion,
				PlaywrightVersion:             MinimumPlaywrightVersion,
				Method:                        recipe.Method,
				MethodSupported:               true,
				FreshIsolatedContext:          true,
				SameContextLoginAndTests:      true,
				PrivateCredentialChannel:      true,
				ApprovedOriginsEnforced:       true,
				AuthStateMemoryOnly:           true,
				CredentialArtifactSuppression: true,
			}
			test.change(&capabilities)
			capability := &fakePreparationCapability{methodCaps: &capabilities}
			result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).Prepare(context.Background(), recipe, time.Minute)
			if result.Status != PreparationBlocked || result.BlockedOn != PrerequisiteMethodAdmission || result.Reason != test.want {
				t.Fatalf("active-session capability was not rejected as %s: %+v", test.want, result)
			}
			calls := capability.snapshotCalls()
			if len(calls) != len(preparationPrerequisites) || calls[len(calls)-1].method != recipe.Method {
				t.Fatalf("method admission did not receive the planned method: %+v", calls)
			}
		})
	}
}

type genericReadyPreparationCapability struct{ calls int }

func (c *genericReadyPreparationCapability) CheckPrerequisite(context.Context, Recipe, PreparationPrerequisite) PrerequisiteObservation {
	c.calls++
	return PrerequisiteObservation{Ready: true}
}

func TestMethodAdmissionCannotBeSkippedByGenericPrerequisiteReadiness(t *testing.T) {
	capability := &genericReadyPreparationCapability{}
	result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).Prepare(context.Background(), validExecutorRecipe(), time.Minute)
	if result.Status != PreparationBlocked || result.BlockedOn != PrerequisiteMethodAdmission || result.Reason != PreparationReasonMethodUnavailable {
		t.Fatalf("generic readiness substituted for live method admission: %+v", result)
	}
	if capability.calls != len(preparationPrerequisites)-1 {
		t.Fatalf("generic capability received %d checks, want only non-method prerequisites", capability.calls)
	}
}

func TestNoProvisioningOrSecretDiagnostics(t *testing.T) {
	const sentinel = "SENTINEL_PREPARATION_DIAGNOSTIC_7f31"
	capability := &fakePreparationCapability{outcomes: map[PreparationPrerequisite][]PrerequisiteObservation{
		PrerequisiteFixtures: {{Reason: PreparationReason(sentinel)}},
	}}
	recipe := validExecutorRecipe()
	recipe.ReadinessCommand = sentinel
	var logs strings.Builder
	preparer := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), slog.New(slog.NewTextHandler(&logs, nil)))
	result := preparer.Prepare(context.Background(), recipe, time.Minute)
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal("marshal sanitized preparation result")
	}
	if result.Status != PreparationBlocked || result.BlockedOn != PrerequisiteFixtures || result.Reason != PreparationReasonFixturesUnavailable {
		t.Fatalf("unknown diagnostic was not sanitized to fixture blocker: %+v", result)
	}
	if strings.Contains(string(encoded), sentinel) || strings.Contains(logs.String(), sentinel) {
		t.Fatal("untrusted diagnostic or plan value leaked into result or logs")
	}
	if capability.provisionCalls != 0 {
		t.Fatalf("preparation attempted provisioning %d times", capability.provisionCalls)
	}
	if !strings.Contains(result.NextAction, "will not provision") {
		t.Fatalf("fixture blocker omitted the no-provisioning instruction: %q", result.NextAction)
	}
}

func TestPreparationExhaustedBudgetAndInterruptedContextDoNotProbe(t *testing.T) {
	for _, test := range []struct {
		name   string
		ctx    func() context.Context
		budget time.Duration
		want   PreparationReason
	}{
		{name: "zero stage budget", ctx: context.Background, budget: 0, want: PreparationReasonStageBudgetExpired},
		{name: "cancelled stage", ctx: func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }, budget: time.Minute, want: PreparationReasonInterrupted},
	} {
		t.Run(test.name, func(t *testing.T) {
			capability := &fakePreparationCapability{}
			result := NewPreparer(capability, newFakePreparationClock(time.Unix(1_800_000_000, 0)), preparationLogger()).Prepare(test.ctx(), validExecutorRecipe(), test.budget)
			if result.Status != PreparationBlocked || result.Reason != test.want || len(capability.snapshotCalls()) != 0 {
				t.Fatalf("Prepare() result=%+v calls=%v, want %s without probes", result, capability.snapshotCalls(), test.want)
			}
		})
	}
}
