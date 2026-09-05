# UI-C09-001 — Telegram Plugin Terminal UX

> Cycle C09 terminal UX. Extends the Settings/Config conventions in [UI-C07-001](UI-C07-001-tui-cycle-config.md) and Chat transcript conventions in [UI-C03-001](UI-C03-001-tui-harness-autonomy.md).

## 1. Settings section

Settings includes a Telegram section only when the optional plugin is installed. It uses the normal focused-label styling and shows no secret values.

```text
Telegram
Plugin:     Installed · vX.Y.Z
Daemon:     Connected
Status:     Configured
Project ID: ai_workflow

[Pair] [Send test] [Replace] [Clear]
```

- Before installation: `Telegram plugin is not installed. Install with: hero plugin install telegram`.
- Before pairing: `Status: Not configured`; Pair is enabled, test is disabled.
- While pairing: navigation remains available only through the modal's explicit Cancel action.
- On daemon outage: `⚠ Telegram daemon disconnected; retrying…` appears in Chat and Settings shows retry state. Successful recovery emits `✓ Telegram daemon reconnected.`
- Project ID is editable in Settings. The live per-instance suffix is display-only, e.g. `ai_workflow_2`.

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
- A cancellation command reports a concise daemon result, such as `✓ Telegram: 3 pending message(s) cancelled for ai_workflow_2.`
- Pending/expired delivery notices are muted informational lines; they do not look like harness responses.
- Transcript text must never contain a token or chat id.

## 4. Notifications and errors

Important notifications are compact and always prefixed by the instance address:

```text
ai_workflow: Cycle #42 started.
ai_workflow: Planning completed. Awaiting approval.
ai_workflow_2: QA failed. 3 issues found.
```

- Do not send thinking, tool, activity, stream deltas, or local diagnostics to Telegram.
- Authorization failures give the remote sender no project-specific response. Local logs may record a redacted rejection count.
- Missing plugin, vault failure, incompatible daemon, failed Bot API connection, and restart exhaustion use existing warning/error colors with an actionable local remediation message.

## 5. Keyboard and accessibility

- All Telegram Settings actions are reachable through the normal Settings focus order and Enter activation.
- The modal has a visible focus target, countdown text, and Escape/Cancel behavior.
- Color never conveys pairing, error, or connection state alone; each status has text and an existing Hero icon.
