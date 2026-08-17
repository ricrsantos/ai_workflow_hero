# Concurrency

## Purpose

This document defines best practices for writing concurrent Go code.

Concurrency should improve responsiveness and throughput while remaining safe, predictable and easy to reason about.

Concurrency is not a goal by itself. Use it only when it provides a clear benefit.

---

# General Principles

Prefer simple concurrent designs.

Every goroutine must have:

- a clear owner;
- a defined lifetime;
- a termination condition.

Never start goroutines that cannot be stopped.

---

# Context First

Any operation that may block should receive a `context.Context`.

Prefer

```go
func Process(ctx context.Context) error
```

instead of

```go
func Process() error
```

Always propagate the received context.

Never create a new background context inside business logic.

Avoid

```go
ctx := context.Background()
```

---

# Context Cancellation

Always respect context cancellation.

Long-running operations should periodically check:

```go
if err := ctx.Err(); err != nil {
    return err
}
```

Never ignore:

- context.Canceled
- context.DeadlineExceeded

---

# Timeouts

External operations should always have timeouts.

Examples:

- HTTP requests
- Database queries
- RPC calls
- Message brokers
- File operations

Prefer configuring timeouts through context.

---

# Goroutines

Start goroutines only when concurrency provides measurable value.

Every goroutine should eventually terminate.

Avoid fire-and-forget goroutines unless explicitly required.

Bad

```go
go process(data)
```

Good

```go
go func() {
    defer wg.Done()
    process(ctx, data)
}()
```

---

# Goroutine Ownership

The function that creates a goroutine is responsible for managing its lifecycle.

That includes:

- cancellation;
- synchronization;
- cleanup.

---

# errgroup

When multiple goroutines contribute to the same operation, prefer `errgroup`.

Example situations:

- parallel API calls;
- concurrent data loading;
- fan-out/fan-in processing.

`errgroup` automatically propagates cancellation when one goroutine fails.

---

# WaitGroup

Use `sync.WaitGroup` only when:

- no error propagation is needed;
- cancellation is not required.

Otherwise prefer `errgroup`.

---

# Channels

Channels communicate ownership of data.

Use channels for communication.

Do not use channels as shared storage.

Keep channel directions explicit whenever possible.

Example

```go
func Producer(out chan<- Item)
func Consumer(in <-chan Item)
```

---

# Buffered Channels

Use buffered channels only with a clear reason.

Document why buffering is required.

Avoid arbitrary buffer sizes.

---

# Closing Channels

The sender closes the channel.

Receivers must never close channels they did not create.

Never close a channel multiple times.

---

# Select

Use `select` whenever waiting on:

- multiple channels;
- cancellation;
- timeouts.

Always consider context cancellation inside long-running selects.

---

# Shared State

Prefer passing data instead of sharing mutable state.

When shared state is necessary:

- minimize its scope;
- document ownership;
- synchronize access.

---

# Mutexes

Use mutexes to protect shared state.

Do not use mutexes as communication mechanisms.

Keep critical sections as short as possible.

Never expose protected state without synchronization.

---

# RWMutex

Use `sync.RWMutex` only when:

- reads greatly outnumber writes;
- contention has been measured.

Do not assume RWMutex is automatically faster.

---

# Atomics

Use atomic operations only for simple independent values.

Avoid replacing mutexes with atomics unless performance measurements justify it.

---

# Deadlocks

Avoid situations where goroutines wait indefinitely.

Common causes:

- forgotten channel receive;
- forgotten channel send;
- missing Done();
- unclosed producer;
- lock ordering issues.

---

# Worker Pools

Use worker pools when processing many independent tasks.

The number of workers should be configurable.

Avoid creating one goroutine per item when processing very large workloads.

---

# Pipelines

Pipeline stages should:

- receive input;
- process;
- produce output;
- terminate on cancellation.

Each stage should respect context cancellation.

---

# Resource Cleanup

Always release resources.

Examples

```go
defer cancel()
defer file.Close()
defer rows.Close()
```

Cleanup should happen as close as possible to resource acquisition.

---

# Race Conditions

Never assume concurrent access is safe.

Run the race detector during development.

```bash
go test -race ./...
```

Code should be written to prevent races rather than relying on testing to find them.

---

# Concurrency Checklist

Before completing concurrent code, verify:

- Does every goroutine terminate?
- Is context propagated?
- Is cancellation respected?
- Are timeouts configured?
- Is shared state synchronized?
- Are channels owned correctly?
- Are resources released?
- Would `errgroup` simplify coordination?
- Is concurrency actually necessary?
- Is the design easy to understand?