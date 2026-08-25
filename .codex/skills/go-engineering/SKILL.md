---
name: go-engineering
description: Improves the quality of Go code generation by enforcing idiomatic Go, modern engineering practices, maintainability, performance, testing, and project organization. This skill is intended for implementation, code review, and refactoring tasks.
version: 1.0.0
author: ricrsantos
---

# Go Engineering

## Purpose

This skill improves the quality of Go code produced by AI agents.

It teaches agents to write idiomatic, maintainable, testable and production-ready Go code while following modern engineering practices.

This skill is intended for:

- Backend implementation
- Refactoring
- Code review
- Bug fixing
- Performance improvements
- Test generation

It is **not** intended to define application architecture or business rules.

---

# Engineering Principles

When writing Go code, always prioritize:

1. Correctness over cleverness.
2. Readability over brevity.
3. Simplicity over abstraction.
4. Small focused functions.
5. Explicit behavior.
6. Standard library whenever possible.
7. Low coupling.
8. High cohesion.
9. Composition over unnecessary abstraction.
10. Idiomatic Go.

---

# Always Follow

The agent should always:

- Produce idiomatic Go.
- Follow Effective Go principles.
- Keep APIs small and easy to understand.
- Prefer explicit code over magic.
- Write self-explanatory code.
- Use meaningful names.
- Return wrapped errors.
- Respect context cancellation.
- Write code that is easy to test.
- Keep packages focused on a single responsibility.
- Avoid unnecessary dependencies.
- Prefer standard library solutions first.

---

# Never

Avoid generating code that:

- Uses panic for normal error handling.
- Ignores returned errors.
- Creates unnecessary abstractions.
- Introduces generic interfaces without a clear need.
- Creates large utility packages.
- Uses reflection when unnecessary.
- Performs premature optimization.
- Hides important logic.
- Creates deeply nested code.
- Uses global mutable state.

---

# Package Organization

Prefer organizing code by feature or domain instead of technical layers.

Packages should have a clear responsibility.

Avoid packages that become generic dumping grounds.

---

# Documentation

Exported types and functions should include concise GoDoc comments.

Comments should explain **why**, not **what**, unless documentation is required for exported APIs.

---

# Decision Priority

When multiple implementations are possible, prefer the solution that is:

1. More idiomatic.
2. Easier to read.
3. Easier to maintain.
4. Easier to test.
5. Simpler.
6. Better aligned with the Go standard library.

---

# Additional Guidance

This skill is composed of specialized documents.

When applicable, also follow the guidance from:

- effective-go.md
- package-design.md
- error-handling.md
- concurrency.md
- testing.md
- performance.md
- logging.md
- security.md
- review-checklist.md

Only load the documents that are relevant to the current task.
