# ADR-C17-003 — Implementation Gate Records Progress per Claim

> Post-C17 architecture decision, taken after C17 Implementation stalled for five waves. Amends [ADR-075](ADR-C08-001-tui-stage-execute.md#adr-075-implementation-completion-is-gated-by-structured-reports-and-the-openspec-checklist) and extends [ADR-102](ADR-C17-001-tui-only-runtime-report-tolerance.md#adr-102-a-rejected-report-retries-with-feedback-then-escalates-it-never-parks-the-cycle-silently) to Implementation and Planning.

| # | Decision | Status |
|---|---|---|
| ADR-107 | The Implementation gate accepts each verified claim, schedules by declared dependencies, escalates instead of repeating a wave, and refuses unsatisfiable SDDs at Planning close | Accepted |

## ADR-107: The Implementation gate records progress per claim

Context: In C17 the `generic_agent` owned all 19 tasks. Two of them could never be met inside the change: task-19 required repository-wide lint to be green against 189 staticcheck and 149 golangci-lint diagnostics that pre-dated the cycle, and task-18 required editing `context/current-state.md`, which the C15 report contract forbids. Four facts turned this into a deadlock:

1. **Wave-level veto.** The TUI gate discarded every ID in `tasks_completed` when a report carried `tests_passed: false` or any false acceptance gate. Those flags describe the whole report, so an open task vetoed the verified work reported next to it. Two waves of verified work (7 and 8 tasks) were discarded.
2. **Dependency-blind waves.** The scheduler assigned every pending task in each wave, so the "after all" verification task travelled with every wave and kept the flags false.
3. **Blind retry.** A wave that recorded nothing returned to the orchestrator, which told the user to "fix and run /hero-start". The next wave received the same contract and produced the same report.
4. **Unchecked SDD.** Planning closed with unsatisfiable tasks, prose-only dependencies, no per-task verification, and decisions left open. Implementation agents asked those questions mid-wave instead.

Prose amendments to `tasks.md` did not help, because the agent's own contract ("set unmet gates false") outranked them.

Decision:

- **Per-claim acceptance.** `completed_tasks_verified`, `task_ownership_respected`, and `required_tests_passed` speak only for the IDs in `tasks_completed`. The scheduler records every claimed ID whose three gates are true, including in a `partial` report. `tests_passed` describes the whole suite and gates stage completion only. A claim with a false gate is not recorded.
- **Declared dependencies.** Task lines may carry `[after:task-a,task-b]`. A pending task is assigned only after every listed task is checked; otherwise it is deferred, which is not an error. An unknown or self dependency, or a cycle that leaves no task ready, invalidates the plan.
- **Escalate instead of repeating.** A valid, non-blocked wave that records nothing escalates Implementation with `implementation_no_progress`. A report with `sdd_ambiguity: true` (allowed only with `partial` or `blocked`; the questions travel in `blocker`) records its verified claims and then escalates with `implementation_sdd_ambiguity`. The escalation copy and the orchestrator preamble never suggest `/hero-start`.
- **Deterministic back-step.** `hero stage reopen-planning --from implementation|judge --reason …` returns Planning and every later enabled stage to Waiting. It keeps iteration counters and stores the reason as the Planning summary, which the TUI hands to `planning_agent` as its assignment. `/hero-back` is available when Judge is pending approval or Implementation is Escalated; it runs this verb and `hero stage start --name planning`. The orchestrator no longer dispatches `planning_agent` itself.
- **Planning SDD check.** Before Planning closes, the TUI checks the linked change. It rejects plan errors (owners, IDs, dependencies), dependency cycles, tasks without a `Verify:` criterion, tasks that edit `context/current-state.md`, and unresolved-decision markers (`TBD`, "to be decided", "open question", "needs decision") in `tasks.md`, `design.md`, and `proposal.md`. A rejection re-dispatches Planning with the list, at most twice (ADR-102's retry budget), and then escalates with `planning_sdd_invalid`.
- **Contracts match the gate.** Implementation agent prompts define the gate semantics above and replace mid-wave questions with `sdd_ambiguity`. The `planning_agent` prompt requires `[after:]`, per-task `Verify:`, no `current-state.md` or repository-wide-gate tasks, and no open decisions.

Consequences: One unsatisfiable task can no longer erase the work around it, and a stalled stage reaches the user as an explicit escalation with a reason and the commands that actually move it forward. The SDD check is deliberately narrow: it rejects only contracts that cannot be met and never judges design quality. Its markers are textual, so Planning may occasionally need to reword a sentence. Accepting claims while the full suite is red relies on the stage-level `tests_passed` gate and the QA stage that follows. `tasks.md` checkboxes remain scheduler-written only.
