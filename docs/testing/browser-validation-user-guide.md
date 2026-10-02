# Browser Validation User Guide — C17 Target Behavior

This guide describes the confirmed C17 design. It is not a claim that the installed release already implements it. Execution is through Hero TUI.

## Prepare test access

After /hero-new, identify whether the planned application screens need authentication. Enable Config → Test users and add each required user/profile, or edit project-root .env.hero using root .env.hero.example. Keep login/password values local. Never send passwords, OTPs or tokens in chat/Telegram. Never commit .env.hero. Config passwords are masked, and stale drafts require Reload.

Planning declares the application URL, start/readiness commands, browser method, compatible tool, test users, fixtures and mandatory screens/journeys. Prefer the project's existing suite. Missing setup is resolved by the user/Implementation; QA does not provision a framework. Login/password forms are supported; interactive MFA/SSO/CAPTCHA are not supported in this cycle.

## Unblock testing

Read the blocked message: it identifies the reason, affected screens/journeys/profile and exact corrective action. Correct local configuration or the named prerequisite, then run /hero-continue. Editing alone does not resume. Accounts must access their protected target; configured does not mean verified. Mixed reports retain real defects while blocked. No stage passes with mandatory coverage pending.

Preparation defaults to two minutes and two attempts per prerequisite. It spends the stage's remaining active budget. timeout_minutes is accumulated active elapsed time across retries/waves, counted once for parallel agents. Human-only waits/offline time do not spend it. Restart requires explicit continuation with the preserved balance. Exhausted budget requires an explicit limit increase; it never resets automatically.

## View screenshots

Enable Screenshots in the Browser UI or browser E2E Config section; each defaults to Off. Each safely tested screen produces a cycle image card. Open the collection using its documented TUI shortcut even while agents run, then Enter/o opens the image in the system viewer. Images are stored in current/screenshots and retained with the archived cycle. Login secrets/tokens are excluded from captures; ordinary application data may be visible.

Use /hero-screenshot for the latest image, /hero-screenshot list for IDs, /hero-screenshot <id> for one, or /hero-screenshot todos for every ready capture in one request. Retrieval is asynchronous and does not ask an agent to take a new image. The same commands are available from the selected Telegram project. Actual image delivery follows the project's Always send reply setting; enable it to receive automatic captures and retrieval batches in the paired chat. Delivery failures do not remove local evidence; read the failed IDs and retry retrieval.

Archive removes root .env.hero but preserves safe screenshots. A cleanup error leaves archive pending; fix the named file-access problem and retry /hero-archive. Resume requires configuring credentials again. Finish, cancel, upgrade and uninstall retain credentials; uninstall reports that fact. Stage approval uses /hero-approve, /hero-reject, /hero-cancel or /hero-finish.
