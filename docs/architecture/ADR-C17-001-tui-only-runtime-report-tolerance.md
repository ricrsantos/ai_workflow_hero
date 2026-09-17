# ADR-C17-001 — TUI-Only Cycle Runtime and Cycle-Preserving Report Contracts

> Cycle C17 architecture decisions. Supersedes the IDE-chat runtime assumption behind [ADR-031](ADR-C04-001-multi-harness.md#adr-031-multi-harness-is-tui-only-cursor-ide-stays-cursor-only) and [ADR-043](ADR-C06-001-codex-adapter.md#adr-043-codex-is-a-third-tui-harness-cursor-ide-stays-cursor-only), and amends [ADR-086](ADR-C15-001-loopback-findings-handoff.md#adr-086-structured-agent-contracts-fail-closed-before-state-mutation).

| # | Decision | Status |
|---|---|---|
| ADR-099 | The Hero TUI is the only cycle runtime; IDE-chat orchestration is removed | Accepted |
| ADR-100 | Stage metrics are measured by the runtime, never self-reported by agents | Accepted |
| ADR-101 | Report contracts fail closed on missing fields and tolerate extra ones | Accepted |
| ADR-102 | A rejected report retries with feedback, then escalates; it never parks the cycle silently | Accepted |

## ADR-099: The Hero TUI is the only cycle runtime; IDE-chat orchestration is removed

Context: Hero originally defined its Runtime as "IDE chat, slash commands" — a cycle was driven by an orchestration agent living in an IDE chat session, with Cursor as the named host. The TUI was added later and ADR-026 already moved TUI orchestration to the harness rather than chat, but the two runtimes continued to coexist. The consequence was structural duplication: the agent prompt files in `assets/*/agents/*.md` are authored for IDE-chat, and `internal/tui/chat_format.go` carries roughly twenty preambles whose only job is to contradict them at runtime, each opening with "TUI execution context (Hero terminal UI — not Cursor IDE chat)". Every prompt change therefore had to be reasoned about twice, and behaviour that only the TUI could provide had to be re-derived in prose for the IDE path.

Decision: The Hero TUI is the single runtime for executing a development cycle. IDE-chat orchestration is removed, not deprecated: its prompt sections and the code that exists to neutralize them are deleted rather than guarded.

This is explicitly **not** a change to harness support. Cursor remains a fully supported harness driven by the TUI through its adapter, along with Claude, Codex, and OpenCode. `assets/cursor/`, harness detection, the `internal/modelprops` catalog and cost entries, workflow config, and the TUI harness picker all stay. The removed thing is the IDE-chat *runtime*, which was never Cursor-specific — Cursor is merely the harness the prompts happened to name.

Consequences: Prompt files describe one execution context, so a prompt change no longer needs a matching TUI override. Capabilities that depend on the runtime observing execution — real token usage, stage timing, automatic retry — can be assumed rather than approximated in prose. The TUI preambles that exist purely to undo IDE-chat assumptions become removable.

## ADR-100: Stage metrics are measured by the runtime, never self-reported by agents

Context: Under IDE-chat orchestration nothing observed the agents, so the only available source of token counts was the agents themselves: every agent prompt instructed the model to estimate `input_chars` and `output_chars`, and the orchestrator converted them at chars ÷ 4 and priced them from `models/*.yml`. The TUI does not have this limitation — `harness.ResolveUsage` already prefers billed counts reported by the adapter and falls back to character estimation only when the adapter reports nothing.

The self-report was therefore redundant, and it was actively harmful. The validation agents' prompts forbade a `metrics` object in the C15 report while displaying a complete `metrics` JSON block as the last artifact before the model generated. Weaker models copied the demonstration and dropped the negation; the decoder then rejected the whole report on `unknown_field`, discarding real validation work. This was observed in the field with a QA report and is the direct motivation for ADR-101.

Decision: No agent estimates, reports, or prices metrics. The TUI measures usage from the harness, prices it through `modelprops.EstimateCatalogCostUSD` against the same `models/*.yml` rates the Config screen shows, and accumulates tokens, duration, and cost onto the stage metrics row. The TUI also renders the per-stage metrics summary at stage close. `hero metrics` remains as a read-only query surface, and `--metrics-json` remains a CLI capability for non-TUI scripting; no runtime prompt instructs an agent to use it.

A model without USD rates in the catalog — notably ChatGPT-subsidized Codex rows — records no cost and says so, rather than reporting a charge the user never incurs.

Consequences: Cost stops depending on a model's arithmetic about its own context. Agent prompts lose every metrics section, including the counter-examples, because an illustrated shape is what weaker models copy. `EstimateCatalogCostUSD`, previously a stub that always returned zero, becomes the single cost authority for the runtime.

## ADR-101: Report contracts fail closed on missing fields and tolerate extra ones

Context: ADR-086 established that structured agent contracts fail closed before any state mutation, and the implementation applied that uniformly, including to unknown JSON fields. Uniformity was the error. A field the contract does not recognize cannot influence what Hero persists, so rejecting the report over one discards findings, stage transitions, and loop-back that were otherwise valid — while a *missing* field leaves Hero genuinely unable to tell what the agent meant. The allow-lists were already being patched per incident (`files_changed` carries the comment "informational; ignored by the scheduler"), which does not scale across models.

Decision: Fail-closed protects state, not shape. Missing and malformed fields continue to reject the entire report and persist nothing: `missing_field`, `invalid_json`, `invalid_enum`, `invalid_owner`, `unknown_reopen_id`, `duplicate_id`, `overlapping_arrays`, `assignment_union_mismatch`, `false_acceptance_gate`, `no_actionable_finding`, and `repro_test_failed` are unchanged. Extra fields are dropped, recorded as an `unknown_field` warning, and the report is decoded and persisted normally. This applies at every level: top-level, `failures[i]`, `repro`, and `acceptance_gates`.

Before a key is dropped it is matched against the contract case- and separator-insensitively, and adopted under its canonical name when it is an unambiguous near-miss (`reopenId` → `reopen_id`), recorded as a `field_renamed` warning. This is not leniency for its own sake: silently discarding a misspelled `reopen_id` would make Hero allocate a fresh finding ID instead of reopening the existing one, which quietly bypasses the three-round loop ceiling. Tolerating a shape must not lose a meaning.

Warnings are surfaced in the TUI at stage close so a drifting agent prompt stays visible instead of accumulating unnoticed.

Consequences: A cycle no longer stalls because a model attached a decorative field. Contract drift becomes observable rather than fatal. The `unknown_field` diagnostic code changes role from rejection to warning, and `field_renamed` is added.

## ADR-102: A rejected report retries with feedback, then escalates; it never parks the cycle silently

Context: Even with ADR-101, a genuinely undecodable report remains possible. The previous behaviour left the stage Running, persisted nothing, and instructed the orchestrator to tell the user to fix the report and run `/hero-start`. That is a hard stop that depends on a human noticing a stage that looks active.

Decision: When a validation report is rejected, Hero re-dispatches the same stage agent with the decoder diagnostic attached, bounded to two attempts. The retry prompt states that the stage work should not be redone, only the report reissued, and that extra fields are tolerated so the agent does not over-correct. When the attempts are spent, the stage is escalated with reason `report_contract`, putting the user in the explicit control state (`/hero-continue`, `/hero-add-todo`, `/hero-cancel`, `/hero-finish`) rather than a silent Running stage.

The retry is scoped to validation stages. Implementation already owns a wave loop and a completion gate; a second retry mechanism there would double-handle the same failure.

Consequences: Automatic recovery from transient contract drift, and an explicit, visible state when recovery fails. The fail-closed guarantee is untouched — a retry persists nothing, exactly as a rejection did.
