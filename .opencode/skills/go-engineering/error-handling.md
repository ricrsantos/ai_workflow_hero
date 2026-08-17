# Error Handling

## Purpose

This document defines best practices for handling errors in Go.

Errors are part of the normal control flow and should be handled explicitly.

The objective is to produce predictable, debuggable and maintainable code.

---

# General Principles

Always treat errors as expected outcomes.

Never ignore returned errors.

Every error should either:

- be handled;
- be wrapped with additional context; or
- be returned to the caller.

---

# Never Ignore Errors

Avoid

```go
result, _ := service.Process(ctx)
```

Prefer

```go
result, err := service.Process(ctx)
if err != nil {
    return err
}
```

---

# Return Errors

Business failures should always be returned.

Do not use panic for expected failures.

---

# Wrap Errors

Whenever an error crosses a layer boundary, wrap it with contextual information.

Prefer

```go
if err != nil {
    return fmt.Errorf("creating user: %w", err)
}
```

Avoid

```go
return err
```

unless additional context would add no value.

---

# Preserve Original Errors

Always wrap using `%w`.

Never discard the original error.

Good

```go
return fmt.Errorf("loading configuration: %w", err)
```

Avoid

```go
return errors.New("configuration error")
```

when an underlying error already exists.

---

# Use errors.Is

Use `errors.Is()` to compare sentinel errors.

Example

```go
if errors.Is(err, ErrNotFound) {
    ...
}
```

Do not compare wrapped errors using `==`.

---

# Use errors.As

Use `errors.As()` when checking for specific error types.

Example

```go
var validationErr *ValidationError

if errors.As(err, &validationErr) {
    ...
}
```

---

# Add Context

Error messages should explain what operation failed.

Good

```go
return fmt.Errorf("saving order: %w", err)
```

Avoid

```go
return err
```

when the caller cannot determine where the failure occurred.

---

# Error Messages

Error messages should:

- start with lowercase;
- not end with punctuation;
- describe the failed operation.

Good

```text
saving order: database unavailable
```

Avoid

```text
Database Error.
```

---

# Sentinel Errors

Use sentinel errors only for well-known conditions.

Example

```go
var ErrNotFound = errors.New("not found")
```

Do not create unnecessary sentinel errors.

---

# Custom Error Types

Define custom error types only when additional structured information is required.

Example

```go
type ValidationError struct {
    Field string
    Reason string
}
```

Do not create custom error types solely to change the error message.

---

# Logging

Errors should generally be logged once.

Prefer logging at the application boundary.

Avoid logging the same error multiple times as it propagates.

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

Application logs once

---

# Context Cancellation

Always propagate context errors.

Example

```go
if err := ctx.Err(); err != nil {
    return err
}
```

Never replace `context.Canceled` or `context.DeadlineExceeded` with generic errors.

---

# Recover

Use `recover()` only to protect application boundaries.

Typical examples

- HTTP middleware
- Worker execution loops
- Background task supervisors

Do not use `recover()` inside business logic.

---

# Avoid Panic

Do not panic because:

- a file was not found;
- a database query failed;
- an API returned an error;
- user input is invalid.

Panic is reserved for unrecoverable programmer errors or impossible states.

---

# Layer Responsibilities

Repository

- Return storage errors.
- Wrap infrastructure failures.

Service

- Add business context.
- Return domain errors.

Handler

- Convert errors into HTTP, gRPC or messaging responses.

Each layer should add context without hiding the original error.

---

# Error Handling Checklist

Before completing a task, verify:

- Were all errors checked?
- Were important errors wrapped?
- Was `%w` used correctly?
- Are `errors.Is()` and `errors.As()` used where appropriate?
- Are context cancellation errors preserved?
- Is panic avoided for normal failures?
- Is each error logged at most once?
- Does every error message describe the failed operation?
- Can the caller understand where the failure originated?