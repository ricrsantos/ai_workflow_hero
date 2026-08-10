# /hero-model — Select Chat Model (TUI)

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

Explain that chat model selection for the Hero TUI is done in the terminal UI, not by dispatching this command into Cursor chat.

## Instructions

1. Tell the user to open the **Hero TUI** (`hero`) and run `/hero-model` from the command palette.
2. The TUI lists models discovered from the harness CLI (`agent models` / Cursor Agent CLI) at boot.
3. Selecting a model persists it to `.workflow-hero/config/hero.json` → `harnesses.<tool>.model` and applies it to subsequent Chat screen `Execute` calls.
4. Do **not** invent a CLI verb for model selection; this is a TUI-only control surface.

## Notes

- Chat **Build** / **Plan** modes are toggled with **Tab** on the Chat screen (Plan maps to Cursor Agent CLI `--mode plan`).
- Runtime Task agents still resolve models from `workflow-config.yml` (ADR-005), independent of the TUI chat model.
