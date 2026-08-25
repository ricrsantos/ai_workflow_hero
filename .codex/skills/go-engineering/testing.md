# Testing

## Purpose

This document defines best practices for writing tests in Go.

Tests should verify behavior, provide confidence for future changes, and serve as executable documentation.

A good test is simple, deterministic, and easy to understand.

---

# General Principles

Tests should be:

- Independent
- Deterministic
- Fast
- Readable
- Maintainable

A test should verify one behavior.

---

# Test Behavior, Not Implementation

Tests should validate externally observable behavior.

Avoid testing private implementation details.

Prefer

```go
user, err := service.Create(ctx, req)
```

instead of checking internal fields or intermediate steps.

---

# Table-Driven Tests

Prefer table-driven tests whenever multiple scenarios share the same logic.

Example

```go
tests := []struct {
    name string
    input string
    want string
}{
    ...
}
```

Each test case should have a descriptive name.

---

# Test Names

Use descriptive names.

Good

```text
TestCreateUser_Success
TestCreateUser_InvalidEmail
TestCreateUser_DuplicateEmail
```

Avoid

```text
Test1
TestUser
TestCase
```

---

# Keep Tests Small

A test should be understandable without scrolling through many screens.

Split unrelated scenarios into separate tests.

---

# Arrange, Act, Assert

Structure tests consistently.

Arrange

Prepare the test data.

Act

Execute the behavior.

Assert

Verify the expected outcome.

Separate these phases with blank lines when it improves readability.

---

# Error Assertions

Always verify returned errors.

Prefer

```go
if err != nil {
    t.Fatalf(...)
}
```

or

```go
if !errors.Is(err, ErrNotFound) {
    ...
}
```

Do not ignore errors.

---

# Compare Expected Results

Compare only the values relevant to the behavior being tested.

Avoid asserting unrelated fields.

---

# Deterministic Tests

Tests should always produce the same result.

Avoid:

- random values;
- current time without control;
- external services;
- network dependencies.

Inject clocks, IDs or random generators when necessary.

---

# Mocking

Mock behavior, not implementations.

Only mock dependencies that cross process boundaries or expensive infrastructure.

Typical examples:

- repositories;
- external APIs;
- message brokers;
- storage.

Avoid mocking simple value objects or business logic.

---

# Interfaces for Testing

Do not introduce interfaces solely to make code testable.

Interfaces should exist because they model behavior, not because tests require them.

---

# Integration Tests

Use integration tests to verify collaboration with real infrastructure.

Examples:

- database;
- HTTP server;
- messaging;
- cache.

Prefer real components over mocks whenever practical.

---

# Test Data

Keep test data small.

Only include fields relevant to the scenario.

Avoid unnecessarily large fixtures.

---

# Test Helpers

Extract helper functions only when they improve readability.

Avoid hiding important behavior inside helper functions.

---

# Subtests

Use subtests to group related scenarios.

Example

```go
t.Run("invalid email", func(t *testing.T) {
    ...
})
```

Keep subtests independent.

---

# Parallel Tests

Use

```go
t.Parallel()
```

only when tests are truly independent.

Avoid parallel execution if shared resources exist.

---

# Benchmarks

Use benchmarks only to measure performance.

Never assume performance improvements without measurement.

Example

```bash
go test -bench=.
```

---

# Race Detection

Run race detection regularly.

```bash
go test -race ./...
```

Concurrent code should always be validated with the race detector.

---

# Coverage

Coverage is a useful indicator, not a goal.

Prefer meaningful tests over maximizing coverage percentage.

A small number of high-quality tests is better than many superficial tests.

---

# Golden Files

Use golden files only when comparing large or complex outputs.

Keep them under version control.

Regenerate them intentionally.

---

# Avoid

Avoid tests that:

- depend on execution order;
- require internet access;
- depend on current time;
- depend on local machine configuration;
- verify implementation instead of behavior;
- become brittle after harmless refactoring.

---

# Testing Checklist

Before completing a task, verify:

- Does each test verify one behavior?
- Are tests deterministic?
- Is table-driven testing appropriate?
- Are test names descriptive?
- Are mocks truly necessary?
- Are important error paths covered?
- Is the code easy to understand?
- Would another developer immediately understand what the test validates?
- Does the test provide confidence rather than merely increasing coverage?