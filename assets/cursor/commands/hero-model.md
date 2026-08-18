# /hero-model — Select Default Model (TUI)

## Role

You are the **orchestration agent** for AI Workflow Hero.

## Responsibilities

Explain that default model selection for the Hero TUI is done in the terminal UI, not by dispatching this command into Cursor chat.

## Instructions

1. Tell the user to open the **Hero TUI** (`hero`) and run `/hero-model` from the command palette.
2. **New projects** ship without a pre-selected default model; the user must pick one at least once before Chat, `/hero-new`, `/hero-sync`, `/hero-back`, `/hero-start`, or imported harness commands will run.
3. The TUI uses the immediate in-memory/project-cache/catalog model list, then refreshes every enabled harness in the background when `/hero-model` opens. OpenCode is not started at TUI boot solely for metadata.
4. After a model is selected, configure the dynamic C5 properties `fs` (fast), `th` (thinking), and `ef` (reasoning effort) when the harness/catalog exposes them. The picker shows `na` and a warning when metadata is unavailable, stale, or a saved value is no longer accepted.
5. Selecting a model and pressing `ENTER to save` persists the pair and properties atomically to `.workflow-hero/config/hero.json` → `harnesses.<tool>.model`, `freechat_default`, and `model_properties.<harness>.<native-model>`. `Esc` cancels the complete draft.
6. The selected pair/properties apply to:
    - ordinary Chat/freechat `Execute` calls
    - `/hero-new` `Execute`
7. Do **not** invent a CLI verb for model selection; this is a TUI-only control surface. Workflow commands continue to use `agents.*` / `fallback_model` values from `workflow-config.yml`; `/hero-model` never edits that YAML.

## Notes

- Chat **Build** / **Plan** modes are toggled with **Tab** on the Chat screen (Plan maps to Cursor Agent CLI `--mode plan`).
- Chat shows the effective labels `[fs-<value>] [th-<value>] [ef-<value>]` below the response pane. Validated freechat values are green; `false`, `na`, unavailable, and workflow values that have not been validated are gray. Missing-catalog and stale-cache notices use the yellow warning status.
- Runtime Task agents still resolve models from `workflow-config.yml` (ADR-005), independent of the TUI default model.
