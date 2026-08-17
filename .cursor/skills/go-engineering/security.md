# Security

## Purpose

This document defines secure coding practices for Go applications.

The objective is to reduce common security risks by encouraging secure defaults during software development.

Security should be considered throughout the implementation, not added afterward.

---

# General Principles

Always prefer secure defaults.

Never trade security for convenience without a clear justification.

Assume that:

- user input is untrusted;
- external systems may fail;
- network communication is unreliable;
- attackers may intentionally provide malformed data.

---

# Input Validation

Validate all external input.

Examples include:

- HTTP requests
- Message queues
- Configuration files
- Environment variables
- File contents
- User input

Reject invalid input as early as possible.

---

# Fail Securely

When an unexpected situation occurs, fail safely.

Prefer denying the operation instead of allowing uncertain behavior.

Never continue execution with partially validated data.

---

# SQL Injection

Always use parameterized queries.

Prefer

```go
db.QueryContext(ctx,
    "SELECT * FROM users WHERE id = ?",
    id,
)
```

Never concatenate user input into SQL statements.

Avoid

```go
query := "SELECT * FROM users WHERE id = " + id
```

---

# Context and Timeouts

Every external operation should use a context with cancellation or timeout.

Examples:

- database
- HTTP
- gRPC
- message brokers
- file operations

Avoid operations that can block indefinitely.

---

# HTTP Clients

Reuse HTTP clients.

Always configure:

- timeout
- transport
- TLS verification

Do not disable certificate validation.

Never use:

```go
InsecureSkipVerify: true
```

unless explicitly required for local development.

---

# HTTP Servers

Validate:

- request size
- request method
- content type

Reject malformed requests early.

Always configure server timeouts.

---

# JSON Processing

Prefer strongly typed structures.

Avoid using:

```go
map[string]any
```

unless dynamic data is required.

Use:

```go
json.Decoder
```

instead of reading the entire request into memory.

When appropriate:

```go
decoder.DisallowUnknownFields()
```

to reject unexpected input.

---

# File Handling

Never trust file names provided by users.

Validate:

- file size
- file type
- destination path

Avoid path traversal vulnerabilities.

---

# Temporary Files

Use secure temporary file creation.

Prefer the standard library.

Always remove temporary files when no longer needed.

---

# Secrets

Never hardcode:

- passwords
- API keys
- tokens
- certificates
- private keys

Secrets should come from secure configuration sources.

---

# Logging

Never log:

- passwords
- authentication tokens
- API keys
- cookies
- private keys
- session identifiers

Mask sensitive values whenever logging is necessary.

---

# Error Messages

Do not expose internal implementation details.

Avoid returning:

- SQL queries
- stack traces
- filesystem paths
- infrastructure details

External error messages should be safe for clients.

Detailed information belongs in logs.

---

# Cryptography

Always use the Go standard library for cryptographic operations.

Do not implement custom cryptographic algorithms.

Use secure random number generation.

Prefer:

```go
crypto/rand
```

instead of:

```go
math/rand
```

for security-sensitive operations.

---

# Authentication

Authentication should always verify identity before granting access.

Never trust client-provided identity information without verification.

---

# Authorization

Always verify permissions.

Authentication identifies the user.

Authorization determines what the user is allowed to do.

Never assume authenticated users are authorized.

---

# Resource Limits

Protect the application against resource exhaustion.

Consider limits for:

- request size
- file size
- concurrent requests
- memory usage
- execution time

---

# Goroutines

Avoid creating unbounded numbers of goroutines from user input.

Concurrency should always be controlled.

---

# Dependencies

Prefer the Go standard library whenever possible.

Minimize third-party dependencies.

Keep dependencies updated.

Remove unused dependencies.

---

# Panic Recovery

Recover only at application boundaries.

Unexpected panics should be logged and converted into controlled failures.

Business logic should never rely on panic recovery.

---

# Secure Defaults

Prefer:

- HTTPS
- TLS verification
- least privilege
- explicit configuration
- parameterized queries
- timeouts
- context propagation

Avoid insecure defaults.

---

# Security Checklist

Before completing a task, verify:

- Is all external input validated?
- Are SQL queries parameterized?
- Are timeouts configured?
- Is context propagated?
- Are secrets protected?
- Are sensitive values excluded from logs?
- Are error messages safe for clients?
- Is TLS verification enabled?
- Is secure randomness used where required?
- Are resource limits considered?
- Are dependencies minimized?