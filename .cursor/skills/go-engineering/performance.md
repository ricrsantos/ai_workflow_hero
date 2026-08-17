# Performance

## Purpose

This document defines best practices for writing efficient Go code.

The objective is to produce code that is naturally efficient while maintaining readability and simplicity.

Performance improvements should always be guided by measurement rather than assumptions.

---

# General Principles

Prioritize:

1. Correctness
2. Readability
3. Maintainability
4. Measured performance

Never sacrifice maintainability for hypothetical performance gains.

---

# Measure First

Do not optimize without evidence.

Use benchmarks and profiling to identify bottlenecks.

Typical tools include:

- go test -bench
- pprof
- trace

Optimization should solve an observed problem.

---

# Standard Library First

Prefer standard library implementations.

Many standard library packages are already highly optimized.

Avoid replacing them without measurable benefits.

---

# Memory Allocations

Reduce unnecessary allocations.

Prefer reusing existing objects when appropriate.

Avoid creating temporary objects inside tight loops.

---

# Slice Allocation

When the required size is known, preallocate slices.

Prefer

```go
items := make([]Item, 0, expectedSize)
```

instead of repeatedly growing the slice.

Avoid excessive overallocation.

---

# Maps

Preallocate maps when the approximate number of elements is known.

Example

```go
users := make(map[string]User, expectedSize)
```

---

# Strings

Avoid repeated string concatenation inside loops.

Prefer

```go
strings.Builder
```

for building strings.

Use

```go
bytes.Buffer
```

when working with bytes.

---

# Copies

Avoid unnecessary copying of large structures.

Pass large structs by pointer when appropriate.

Small immutable values may be passed by value.

---

# Receivers

Choose receiver types intentionally.

Use value receivers for:

- small immutable types;
- value semantics.

Use pointer receivers for:

- large structs;
- mutable state;
- consistency across methods.

---

# Interfaces

Do not introduce interfaces solely for performance.

Keep interfaces small and focused.

Prefer concrete types until abstraction is required.

---

# Reflection

Reflection is slower and less type-safe.

Avoid reflection unless it provides significant value.

Prefer compile-time type safety.

---

# Goroutines

Concurrency is not a performance optimization by itself.

Use goroutines only when:

- work is independent;
- parallelism provides measurable benefit;
- synchronization cost is justified.

---

# Channels

Channels have overhead.

Use channels for communication.

Do not replace simple function calls with channels unnecessarily.

---

# Mutexes

Keep critical sections short.

Avoid holding locks while:

- performing I/O;
- waiting on external services;
- executing expensive computations.

---

# Database Operations

Prefer fewer efficient queries over many small queries.

Avoid repeated queries inside loops.

Batch operations whenever practical.

---

# HTTP

Reuse HTTP clients.

Avoid creating a new client for every request.

Always configure timeouts.

---

# JSON

Marshal and unmarshal only when necessary.

Avoid repeatedly encoding and decoding the same data.

Prefer typed structures over generic maps.

---

# Loops

Avoid unnecessary work inside loops.

Move invariant computations outside the loop whenever possible.

Prefer clear code over micro-optimizations.

---

# Logging

Avoid expensive log message construction when the log level is disabled.

Use structured logging.

Do not serialize large objects unless necessary.

---

# Object Reuse

Reuse expensive objects when it improves performance and does not reduce clarity.

Examples may include:

- buffers;
- encoders;
- decoders.

Avoid premature pooling.

---

# sync.Pool

Use `sync.Pool` only after profiling demonstrates allocation pressure.

Do not use it as a general-purpose cache.

---

# Caching

Introduce caching only when:

- repeated computation is expensive;
- cache invalidation is well understood;
- memory usage is acceptable.

Avoid speculative caching.

---

# Benchmarking

Use benchmarks to compare implementations.

Example

```bash
go test -bench=.
```

Benchmark realistic workloads.

Avoid drawing conclusions from synthetic microbenchmarks alone.

---

# Profiling

Use profiling to locate bottlenecks before optimizing.

Measure:

- CPU usage
- Memory allocations
- Blocking
- Goroutine activity

Do not guess where performance problems exist.

---

# Readability

A slightly slower implementation that is significantly easier to understand is often the better choice.

Optimize only when measurable gains justify the added complexity.

---

# Performance Checklist

Before optimizing code, verify:

- Has the problem been measured?
- Is there benchmark data?
- Is readability preserved?
- Are unnecessary allocations avoided?
- Are slices and maps preallocated when appropriate?
- Is reflection truly necessary?
- Are HTTP clients reused?
- Are database operations efficient?
- Is concurrency justified?
- Would another developer easily understand the optimized code?