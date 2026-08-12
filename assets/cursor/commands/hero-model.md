# /hero-model — Select Default Model (TUI)

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

Explain that default model selection for the Hero TUI is done in the terminal UI, not by dispatching this command into Cursor chat.

## Instructions

1. Tell the user to open the **Hero TUI** (`hero`) and run `/hero-model` from the command palette.
2. **New projects** ship without a pre-selected default model; the user must pick one at least once before Chat, `/hero-new`, `/hero-sync`, `/hero-back`, `/hero-start`, or imported harness commands will run.
3. The TUI lists models discovered from the harness CLI (`agent models` / Cursor Agent CLI) at boot.
4. Selecting a model persists it to `.workflow-hero/config/hero.json` → `harnesses.<tool>.model` and applies it to:
   - Chat screen `Execute` calls (free chat and stage-bound conversation)
   - TUI runtime dispatches without a named Hero agent (`/hero-new`, `/hero-sync`, `/hero-back`, `/hero-start`, imported commands)
5. Do **not** invent a CLI verb for model selection; this is a TUI-only control surface.

## Notes

- Chat **Build** / **Plan** modes are toggled with **Tab** on the Chat screen (Plan maps to Cursor Agent CLI `--mode plan`).
- Runtime Task agents still resolve models from `workflow-config.yml` (ADR-005), independent of the TUI default model.
