# C9 QA loop-back — implementation gaps

Source: QA (`qa_agent`, `gpt-5.6-terra`). `tests_passed: false`. `logging: fail`.
Build/vet/diff/architecture passed. No secrets in changed logging.

## Required work (this Implementation pass)

1. Logging in `internal/engine/engine.go` `LoopBackToImplementation`:
   - Info-level success log already exists.
   - Add application `error` logs on meaningful failure return paths (errors currently return without error-level logging).
   - Add `debug`-level operational logging on the changed loop-back path (required leveled coverage; debug must not emit at the default `info` level).
   - Do not log secrets, credentials, tokens, or PII.

2. Remaining Telegram SDD gaps from Judge (Implementation iter 2 returned empty):
   Read `.workflow-hero/cycles/current/judge-gaps.md` and implement those tasks only against `openspec/changes/telegram-integration/`.
   Backend packages exist; wire TUI and operational paths.

Scope: native → `generic_agent`.
