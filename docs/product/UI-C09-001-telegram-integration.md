# UI-C09-001 — Telegram Plugin Terminal UX

> Cycle C09 terminal UX. Extends the Settings/Config conventions in [UI-C07-001](UI-C07-001-tui-cycle-config.md) and Chat transcript conventions in [UI-C03-001](UI-C03-001-tui-harness-autonomy.md).

## 1. Settings section

Settings is a two-section screen: **CHAT VERBOSITY** (radio list) then **TELEGRAM PLUGIN** (status card). Section headers, rules, and status badges are not in the focus order. The Telegram card shows no secret values.

```text
Settings
────────────────────────────────────────
CHAT VERBOSITY                  select one
> Debug     Everything Hero currently shows…
  Compact   Responses, errors, …

────────────────────────────────────────
TELEGRAM PLUGIN            optional plugin
  Status:  [ Configured ]
  Plugin:  Installed · vX.Y.Z (protocol vN)
  Daemon:  Connected
  Project ID: ai_workflow  (live: ai_workflow_2)
  Auto report: Every 15 min
  Always send: Disabled
  | Replace |  | Clear |  | Test |
```

- Before installation: badge `Not installed`, command `$ hero plugin install telegram`, and `| Copy command |` (Enter copies the CLI string). Install remains a CLI operation.
- Before pairing: badge `Not configured`; Pair is the action button; Test is not shown until configured.
- While pairing: navigation remains available only through the modal's explicit Cancel action.
- On daemon outage: `⚠ Telegram daemon disconnected; retrying…` appears in Chat and Settings shows retry state. Successful recovery emits `✓ Telegram daemon reconnected.`
- Project ID is editable in Settings. The live per-instance suffix is display-only, e.g. `ai_workflow_2`.
- Auto report is editable per project: `0` means `Disabled`; `1` through `300` mean `Every N min`.
- Always send is a boolean per-project setting. `Disabled` preserves the default Telegram-only reply routing; `Enabled` also forwards completed local TUI harness turn responses to Telegram.

## 2. Pairing modal

```text
Pair Telegram                                      [Esc: cancel]

1. Open the configured Telegram bot.
2. Send: /start 428391
3. Return here; pairing will complete automatically.

Code expires in 09:42
Waiting for confirmation…

[Cancel]
```

- The code is visible only during the active pairing operation and is never written to a transcript/log.
- Success closes the modal and shows `✓ Telegram paired.`
- Timeout shows `⚠ Pairing code expired. Start pairing again.`
- Cancel closes the modal and invalidates the code.
- No screen renders the bot token or authorized chat id.

## 3. Chat transcript

Telegram content is visibly distinct from local input and harness output while retaining existing speaker headers.

```text
← [Telegram · ai_workflow_2]
Me confirme em qual projeto você está trabalhando.

[FREE - gpt-5.6-terra · codex]
Estou trabalhando em AI Workflow Hero…

→ [Telegram · ai_workflow_2]
Estou trabalhando em AI Workflow Hero…
```

- A Telegram slash command uses the existing command result rendering, preceded by the remote-origin line.
- Telegram `/model` keeps the selector in Telegram rather than opening the local palette. It asks `Escolha o Harness:`, `Escolha o modelo:`, and each available friendly property name (for example `Reasoning effort:`), with one-based numbered options.
- After `/hero-new`, the reply points the user to `/hero-config`. The configuration wizard asks for the title, objective, language, scope, stages, and human approval for each enabled stage, followed by whether to review cycle-agent models. It reviews only agents required by the enabled stages and selected scopes, hiding stale blocks outside that set. Each active named agent is reviewed as parent harness → model → property, followed by a subagent choice: keep the current setting, reuse the parent model, or choose a dedicated model. A dedicated subagent keeps the parent harness and uses the same numbered model → property flow; selecting a new parent model defaults the subagent back to the parent model. The fallback is reviewed without a subagent. The final summary offers `Salvar configuração`, `Revisar modelos`, or `Cancelar`.
- `/hero-config-show` displays a compact, non-secret summary of the canonical cycle configuration; while a wizard is active it displays the in-memory draft. `/hero-config cancel` discards that draft. No wizard answer is persisted before the explicit save choice.
- A cancellation command reports a concise daemon result, such as `✓ Telegram: 3 pending message(s) cancelled for ai_workflow_2.`
- `/list` returns a numbered list of connected instances. `/select n` confirms and persists the selected instance; subsequent text and slash commands need no prefix. Unprefixed text may contain `:` (for example error paste or `Nota: …`) and still routes to the selected instance; only a leading valid instance address (`aiwkhero:`, `aiwkhero_2:`, `free_1:`) is treated as explicit targeting. If that instance disconnects, the daemon asks the user to run `/list` and select again. A live forwarded message receives `OK, Received.`.
- Telegram `/help` replies with a compact catalog of routing, control, cycle-config, permission, queue, and Hero slash commands. It works before `/select` and does not create a harness turn.
- Telegram `/status` replies `idle` with the selected free-chat model and `Session`, `AI wk`, `AI rp`, and `Context: used/max`, or active-cycle details / `Waiting for harness`, as applicable. `Context` is the occupancy of the session currently shown in Chat (last model-call prompt including cache, plus that turn's output), not a billed sum across turns. Ordinary freechat during an active cycle reports the freechat session, not the stage agent. Cycle replies include the cycle title and current stage without the objective/summary. Active replies also include an `Agents` block with `agent: model` rows for operating agents. The Free Chat parent is named `harness`; Auto report sends the compact non-idle reply at its configured interval and remains silent while idle.
- Telegram `/interrupt` cancels every in-flight Chat Execute, including concurrent executions and `/hero-start` preflight, using the same TUI cancellation path as `Ctrl+C`; it does not create a harness turn.
- Telegram `/kill` is a last-resort command: it force-kills the selected TUI process (`SIGKILL`) without confirmation. The IPC client acknowledges delivery and best-effort replies `Killing TUI.` before dying; it does not wait on the Bubble Tea Update loop and does not stop the Telegram daemon.
- A harness-native permission is shown as a separate blocking prompt. Telegram receives its request ID and answers with `/hero-permission <id> allow` or `/hero-permission <id> deny`; the local TUI `y`/`n` path remains available.
- Pending/expired delivery notices are muted informational lines; they do not look like harness responses.
- Transcript text must never contain a token or chat id.

## 4. Notifications and errors

Important notifications are compact and always prefixed by the instance address:

```text
ai_workflow: Cycle #42 started.
ai_workflow: Planning completed. Awaiting approval.
ai_workflow_2: QA failed. 3 issues found.
ai_workflow_2: disconnected.
```

- Do not send thinking, tool, activity, stream deltas, or local diagnostics to Telegram.
- Authorization failures give the remote sender no project-specific response. Local logs may record a redacted rejection count.
- Cycle approvals emitted by a CLI child are delivered through the owning TUI's private lifecycle relay, so Telegram receives `Approval required: <stage>` even when the `hero stage close` command ran inside OpenCode. The relay does not forward intermediate harness activity.
- Missing plugin, vault failure, incompatible daemon, failed Bot API connection, and restart exhaustion use existing warning/error colors with an actionable local remediation message.

## 5. Keyboard and accessibility

- All Telegram Settings actions are reachable through the normal Settings focus order and Enter activation.
- The modal has a visible focus target, countdown text, and Escape/Cancel behavior.
- Color never conveys pairing, error, or connection state alone; each status has text and an existing Hero icon.
