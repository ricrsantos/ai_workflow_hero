# Effective Go

## Purpose

This document defines the fundamental principles for writing idiomatic Go code.

These rules should be considered the default unless another document provides more specific guidance.

---

# General Principles

Write Go code that is:

- Simple
- Readable
- Explicit
- Predictable
- Maintainable

Avoid clever solutions when a straightforward implementation is available.

Prefer code that another Go developer can understand immediately.

---

# Simplicity First

Always choose the simplest solution that correctly solves the problem.

Avoid introducing abstractions before they are needed.

Do not optimize prematurely.

---

# Naming

Use short, meaningful names.

Examples:

Good

```go
user
repo
ctx
err
cfg
req
res
```

Avoid

```go
userInformationObject
temporaryVariable
repositoryInstance
```

Package names should be:

- lowercase
- singular
- concise
- descriptive

Good

```text
user
order
billing
cache
config
```

Avoid

```text
helpers
common
utilities
misc
shared
```

---

# Functions

Functions should perform one clear responsibility.

Prefer small functions.

Large functions are difficult to understand and test.

If a function requires scrolling several screens, consider splitting it.

---

# Function Parameters

Keep parameter lists small.

Prefer:

```go
func Create(ctx context.Context, req CreateUserRequest)
```

instead of

```go
func Create(
    ctx context.Context,
    name string,
    email string,
    phone string,
    address string,
    city string,
)
```

When many values naturally belong together, define a struct.

---

# Return Values

Return only what the caller needs.

Prefer

```go
(user, error)
```

instead of

```go
(id int, user User, ok bool, err error)
```

---

# Zero Values

Design types so that their zero value is useful whenever possible.

Avoid requiring initialization unless necessary.

---

# Composition

Prefer composition over inheritance-like abstractions.

Embed behavior only when it improves clarity.

Do not build deep embedding hierarchies.

---

# Interfaces

Interfaces describe behavior.

Keep interfaces small.

One or two methods are often enough.

Prefer

```go
type Reader interface {
    Read([]byte) (int, error)
}
```

Avoid large "god interfaces".

---

# Structs

Keep structs cohesive.

Every field should contribute to the same responsibility.

Large structs often indicate multiple responsibilities.

---

# Constants

Use typed constants whenever appropriate.

Prefer

```go
const StatusActive Status = "active"
```

instead of magic strings.

---

# Control Flow

Return early.

Avoid unnecessary nesting.

Prefer

```go
if err != nil {
    return err
}
```

instead of

```go
if err == nil {
    ...
}
```

---

# Switch

Prefer switch statements when they improve readability.

Avoid long chains of if/else when evaluating multiple cases.

---

# Defer

Use defer for cleanup.

Typical examples:

- closing files
- unlocking mutexes
- closing HTTP bodies
- canceling contexts

Do not use defer inside hot loops when performance is critical.

---

# Panic

Do not use panic for expected failures.

Panic should only be used for:

- impossible states
- programmer errors
- application startup failures when execution cannot continue

Business errors must always be returned.

---

# Standard Library First

Always check whether the Go standard library already provides a solution.

Avoid external dependencies that duplicate standard functionality.

---

# Explicitness

Prefer explicit code over hidden behavior.

Avoid APIs that surprise the reader.

A reader should understand what the code does without reading multiple files.

---

# Readability

Optimize for the next developer reading the code.

Readable code is more valuable than clever code.

When in doubt, choose the implementation that is easier to understand.

---

# Idiomatic Go Checklist

Before considering code complete, verify:

- Is the code simple?
- Is the naming clear?
- Are functions small?
- Are interfaces minimal?
- Is control flow straightforward?
- Are errors handled explicitly?
- Is the standard library preferred?
- Is the code easy to test?
- Is unnecessary abstraction avoided?
- Would an experienced Go developer consider this idiomatic?