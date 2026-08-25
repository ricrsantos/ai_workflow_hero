# Logging

## Purpose

This document defines best practices for structured logging in Go applications.

Logs should provide meaningful operational insights while remaining concise, searchable and consistent.

Prefer structured logging using the Go standard library (`log/slog`).

---

# General Principles

Logs should help answer:

- What happened?
- When did it happen?
- Where did it happen?
- Why did it fail?
- What should be investigated?

Every log entry should provide value.

---

# Structured Logging

Always use structured logging.

Prefer

```go
logger.Info(
    "user created",
    "user_id", user.ID,
    "email", user.Email,
)
```

Avoid

```go
logger.Info(fmt.Sprintf(
    "User %d created",
    user.ID,
))
```

Structured logs are easier to search, filter and analyze.

---

# Log Levels

Use log levels consistently.

## DEBUG

Development information.

Examples

- request details
- intermediate values
- execution flow

Should normally be disabled in production.

---

## INFO

Normal application events.

Examples

- application started
- user created
- message processed
- job completed

---

## WARN

Unexpected situations that do not prevent execution.

Examples

- retry attempts
- temporary failures
- degraded functionality

Warnings should indicate situations worth monitoring.

---

## ERROR

Failures that prevent the requested operation.

Examples

- database unavailable
- validation failed
- external API failure

Errors should include sufficient context for investigation.

---

# Log Once

Errors should generally be logged only once.

Prefer logging at the application boundary.

Avoid duplicate logging across multiple layers.

Bad

Repository logs

↓

Service logs

↓

Handler logs

↓

Main logs

Good

Repository returns

↓

Service wraps

↓

Handler returns

↓

Application logs

---

# Add Context

Logs should include relevant contextual information.

Examples

```go
logger.Error(
    "order processing failed",
    "order_id", order.ID,
    "customer_id", order.CustomerID,
    "error", err,
)
```

Context should help identify the affected operation.

---

# Log Keys

Use consistent field names.

Examples

```text
request_id
trace_id
customer_id
user_id
order_id
duration_ms
status
error
```

Avoid multiple names for the same concept.

---

# Error Logging

Log the error object.

Prefer

```go
logger.Error(
    "saving order failed",
    "error", err,
)
```

Avoid logging only the error message string.

---

# Sensitive Information

Never log:

- passwords
- secrets
- API keys
- authentication tokens
- private keys
- session identifiers
- encryption keys

Avoid logging personal information unless explicitly required.

When necessary, mask or redact sensitive values.

---

# Large Objects

Avoid logging entire objects.

Instead, log only the relevant fields.

Bad

```go
logger.Info("request", "request", req)
```

Prefer

```go
logger.Info(
    "request received",
    "user_id", req.UserID,
    "items", len(req.Items),
)
```

---

# Performance

Avoid constructing expensive log messages unnecessarily.

Prefer structured fields over formatted strings.

---

# Consistency

Use the same message style throughout the application.

Prefer short action-oriented messages.

Examples

```text
user created

order processed

configuration loaded

database connection established
```

Avoid inconsistent wording.

---

# Startup Logging

Log important startup information.

Examples

- application version
- environment
- enabled features
- listening port

Do not log secrets or credentials.

---

# Shutdown Logging

Log graceful shutdown events.

Examples

- shutdown requested
- workers stopped
- connections closed

Unexpected termination should be logged as an error.

---

# Request Logging

Log request lifecycle at application boundaries.

Typical fields include:

- request ID
- method
- path
- duration
- status code

Avoid logging request bodies unless necessary.

---

# Background Jobs

Include identifiers that uniquely identify the job.

Examples

- job ID
- worker ID
- execution duration
- retry count

---

# Logging Checklist

Before completing a task, verify:

- Is structured logging used?
- Are log levels appropriate?
- Is the error logged only once?
- Is enough context provided?
- Are field names consistent?
- Is sensitive information excluded?
- Are large objects avoided?
- Are messages concise and meaningful?
- Would the logs help diagnose production issues?