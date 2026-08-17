# Review Checklist

## Purpose

This document defines the final review process that every Go implementation should follow before it is considered complete.

The objective is to identify correctness, readability, maintainability, performance, testing and security issues before delivering the code.

This checklist should be applied after implementation and before presenting the final result.

---

# General Review

Verify that:

- The implementation satisfies all requirements.
- No requested behavior is missing.
- No unnecessary functionality was introduced.
- The solution remains as simple as possible.

---

# Code Quality

Verify that:

- The code is idiomatic Go.
- Naming is clear and consistent.
- Functions have a single responsibility.
- Functions remain reasonably small.
- Packages have a clear responsibility.
- Public APIs are minimal.
- Unnecessary abstractions were avoided.
- The standard library is preferred whenever possible.

---

# Readability

Verify that:

- The code is easy to understand.
- Control flow is straightforward.
- Nesting is minimized.
- Early returns are used where appropriate.
- Magic values have been eliminated.
- Comments explain intent rather than implementation.

A future maintainer should understand the code quickly.

---

# Error Handling

Verify that:

- Every returned error is handled.
- Errors are wrapped with contextual information.
- `%w` is used when wrapping errors.
- `errors.Is()` is used for sentinel errors.
- `errors.As()` is used for typed errors.
- Panic is not used for expected failures.
- Error messages describe the failed operation.

---

# Context

Verify that:

- `context.Context` is propagated.
- Long-running operations respect cancellation.
- Timeouts exist for external operations.
- No unnecessary `context.Background()` calls exist.

---

# Concurrency

If concurrency is used, verify that:

- Every goroutine terminates.
- Context cancellation is respected.
- Resources are released.
- Channels have clear ownership.
- Shared state is synchronized.
- Goroutines cannot leak.
- Deadlocks are unlikely.
- `errgroup` would not simplify the implementation.

If concurrency is unnecessary, do not introduce it.

---

# Performance

Verify that:

- No obvious unnecessary allocations exist.
- Reflection is avoided unless justified.
- Slices and maps are preallocated when appropriate.
- Expensive work inside loops is minimized.
- HTTP clients are reused.
- Performance optimizations remain readable.
- No optimization was added without a reasonable justification.

---

# Logging

Verify that:

- Structured logging is used.
- Log levels are appropriate.
- Errors are logged only once.
- Sufficient context is included.
- Sensitive information is never logged.
- Log messages are concise and consistent.

---

# Security

Verify that:

- External input is validated.
- SQL queries are parameterized.
- Secrets are never hardcoded.
- Sensitive data is excluded from logs.
- TLS verification remains enabled.
- External operations have timeouts.
- Error messages do not expose internal implementation details.

---

# Testing

Verify that:

- The implementation is testable.
- Existing tests remain valid.
- New behavior is covered by tests when appropriate.
- Tests verify behavior rather than implementation.
- Tests are deterministic.
- Mocking is used only when necessary.

---

# Dependencies

Verify that:

- No unnecessary dependency was introduced.
- The standard library was preferred.
- Existing abstractions were reused appropriately.
- Coupling between packages remains low.

---

# API Design

Verify that:

- Public functions are easy to understand.
- Function signatures remain concise.
- Parameters are minimal.
- Return values are clear.
- Interfaces remain small and behavior-oriented.

---

# Maintainability

Verify that:

- Future changes will be easy.
- Responsibilities are clearly separated.
- The implementation follows existing project conventions.
- Similar problems are solved consistently across the codebase.

---

# Final Validation

Before considering the implementation complete, ask:

- Is this the simplest correct solution?
- Would an experienced Go developer consider this idiomatic?
- Is the implementation easy to maintain?
- Would another developer understand it quickly?
- Is every important error handled?
- Is the implementation safe?
- Is it adequately tested?
- Is there any unnecessary complexity that can still be removed?

If any answer is **No**, revise the implementation before presenting it.

Only consider the task complete when the answer to every applicable item is **Yes**.