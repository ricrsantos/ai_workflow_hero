# Telegram image transport reference

Verified on 2026-09-30 against the official Bot API documentation. This is
reference evidence for task-13, not an implemented delivery contract or a
completed acceptance gate.

The standard hosted API's sendPhoto accepts multipart image uploads up to
10 MB; combined width plus height may not exceed 10000, and aspect ratio
may not exceed 20. Captions allow up to 1024 characters after entity parsing.
Validate these constraints explicitly and report failed screenshot IDs instead
of silently omitting or changing files. This does not authorize a local Bot API
server, broadcasts, unpaired chats, or fallback document delivery.

Source: https://core.telegram.org/bots/api#sendphoto

sendMediaGroup requires 2–10 items per album. A single image therefore uses
sendPhoto; larger ready-set snapshots require ordered bounded batches with
explicit per-ID retry results, not truncation.

Source: https://core.telegram.org/bots/api#sendmediagroup

Delivery remains daemon-owned, addressed, and gated by project always_send.
Do not include credential values or raw provider responses in captions/errors.
