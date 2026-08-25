# Package Design

## Purpose

This document defines best practices for organizing Go packages.

The goal is to maximize readability, maintainability, and discoverability while keeping packages cohesive and loosely coupled.

This document intentionally avoids prescribing a specific software architecture.

---

# Guiding Principles

A package should have:

- One clear responsibility.
- High cohesion.
- Low coupling.
- A well-defined public API.

A developer should quickly understand the package's purpose from its name.

---

# Organize by Feature

Prefer organizing packages around business features or domains instead of technical layers.

Good

```text
internal/
    user/
    order/
    billing/
    notification/
```

Avoid

```text
controllers/
services/
repositories/
models/
helpers/
```

---

# Vertical Organization

Keep everything related to a feature close together.

Example

```text
internal/
    user/
        service.go
        repository.go
        handler.go
        model.go
        validator.go
        errors.go
```

The objective is to reduce navigation between unrelated directories.

---

# Package Responsibilities

Each package should solve one problem.

Good examples

```text
cache
config
auth
billing
orders
users
```

Avoid packages that become generic storage locations.

Examples

```text
common
shared
utils
helpers
misc
base
core
```

---

# Public API

Export only what other packages truly need.

Everything else should remain unexported.

Smaller public APIs are easier to understand and maintain.

---

# Internal Types

Implementation details should remain private whenever possible.

Expose behavior instead of implementation.

---

# Avoid Cyclic Dependencies

Package dependencies should form a clear direction.

Never create circular imports.

If two packages depend on each other, reconsider the package boundaries.

---

# Keep Packages Small

A package should remain understandable without reading the entire project.

Large packages often indicate multiple responsibilities.

When appropriate, split packages by responsibility.

---

# File Organization

Group files by responsibility.

Example

```text
service.go
repository.go
handler.go
validator.go
errors.go
dto.go
mapper.go
```

Avoid arbitrary names such as

```text
part1.go
extra.go
misc.go
temp.go
```

---

# Constructors

Provide constructors when initialization is required.

Example

```go
func NewService(repo Repository, logger *slog.Logger) *Service
```

Avoid constructors that perform unnecessary work.

---

# Dependency Injection

Dependencies should be received explicitly.

Prefer constructor injection.

Good

```go
service := NewService(repo, logger)
```

Avoid:

- Global dependencies.
- Service locators.
- Hidden initialization.

---

# Keep Dependencies Minimal

A package should depend only on what it needs.

Avoid importing packages only for convenience.

Favor the standard library whenever possible.

---

# Configuration

Configuration should be passed into packages.

Avoid packages reading environment variables directly.

Good

```go
cfg := Config{
    Timeout: time.Second * 5,
}
```

instead of

```go
os.Getenv(...)
```

inside business logic.

---

# Utilities

Avoid creating large utility packages.

If a helper is only used by one package, keep it inside that package.

Extract shared code only when it has multiple legitimate consumers.

---

# Data Types

Keep related types together.

Example

```text
user.go
user_service.go
user_repository.go
```

instead of placing all structs into one global models package.

---

# Interfaces

Define interfaces where they are consumed.

Keep interfaces focused on behavior.

Prefer

```go
type UserStore interface {
    FindByID(...)
}
```

over large generic interfaces.

---

# Package Checklist

Before creating or modifying a package, verify:

- Does the package have one responsibility?
- Is the package organized by feature?
- Is the public API minimal?
- Are implementation details private?
- Are dependencies explicit?
- Is constructor injection used?
- Are circular dependencies avoided?
- Are helper functions kept close to where they are used?
- Would a new developer immediately understand the package's purpose?