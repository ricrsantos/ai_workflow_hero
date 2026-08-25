# Examples

## Purpose

This document provides practical examples of idiomatic Go code.

Each example demonstrates the evolution from a poor implementation to a better one, explaining why the final version is preferred.

These examples complement the engineering rules defined by the other documents.

---

# Error Handling

## Bad

Errors are ignored.

```go
user, _ := repo.FindByID(ctx, id)
```

## Better

The error is checked.

```go
user, err := repo.FindByID(ctx, id)
if err != nil {
    return err
}
```

## Best

The error is wrapped with context.

```go
user, err := repo.FindByID(ctx, id)
if err != nil {
    return fmt.Errorf("finding user %d: %w", id, err)
}
```

---

# Context

## Bad

```go
func Process() error {
    ...
}
```

## Better

```go
func Process(ctx context.Context) error {
    ...
}
```

## Best

```go
func Process(ctx context.Context) error {
    if err := ctx.Err(); err != nil {
        return err
    }

    ...
}
```

---

# Constructors

## Bad

```go
service := &Service{}
```

## Better

```go
service := NewService(repo)
```

## Best

```go
service := NewService(
    repo,
    logger,
    cache,
)
```

Dependencies are explicit.

---

# Dependency Injection

## Bad

```go
var db = connectDatabase()
```

## Better

```go
type Service struct {
    db *sql.DB
}
```

## Best

```go
type Service struct {
    repo Repository
}
```

Depend on behavior instead of implementation.

---

# Interfaces

## Bad

```go
type UserRepository interface {
    Save(...)
    Update(...)
    Delete(...)
    Find(...)
    Count(...)
    Exists(...)
    ...
}
```

## Better

```go
type UserStore interface {
    Save(...)
    Find(...)
}
```

## Best

```go
type UserFinder interface {
    FindByID(...)
}

type UserSaver interface {
    Save(...)
}
```

Small interfaces are easier to implement and test.

---

# Logging

## Bad

```go
logger.Info(fmt.Sprintf(
    "User %d created",
    id,
))
```

## Better

```go
logger.Info(
    "user created",
    "user_id", id,
)
```

## Best

```go
logger.Info(
    "user created",
    "user_id", user.ID,
    "email", user.Email,
    "tenant_id", tenantID,
)
```

---

# Error Logging

## Bad

Repository

```go
logger.Error(err.Error())
return err
```

Service

```go
logger.Error(err.Error())
return err
```

Handler

```go
logger.Error(err.Error())
```

## Best

Repository

```go
return fmt.Errorf("saving user: %w", err)
```

Service

```go
return fmt.Errorf("creating user: %w", err)
```

Application

```go
logger.Error(
    "request failed",
    "error", err,
)
```

Log once.

---

# Panic

## Bad

```go
if err != nil {
    panic(err)
}
```

## Best

```go
if err != nil {
    return fmt.Errorf("creating order: %w", err)
}
```

---

# Early Return

## Bad

```go
if err == nil {
    ...
} else {
    return err
}
```

## Best

```go
if err != nil {
    return err
}

...
```

Reduce nesting.

---

# Goroutines

## Bad

```go
go process(item)
```

Nobody owns the goroutine.

## Better

```go
wg.Add(1)

go func() {
    defer wg.Done()
    process(item)
}()
```

## Best

```go
g, ctx := errgroup.WithContext(ctx)

g.Go(func() error {
    return process(ctx, item)
})

if err := g.Wait(); err != nil {
    return err
}
```

---

# Channels

## Bad

Using channels for shared state.

```go
counter <- value
```

## Best

Use channels to communicate work.

```go
jobs <- Job{}
```

Use mutexes to protect shared state.

---

# String Building

## Bad

```go
result := ""

for _, s := range values {
    result += s
}
```

## Best

```go
var builder strings.Builder

for _, s := range values {
    builder.WriteString(s)
}

return builder.String()
```

---

# Slice Allocation

## Bad

```go
var users []User

for _, id := range ids {
    users = append(users, load(id))
}
```

## Best

```go
users := make([]User, 0, len(ids))

for _, id := range ids {
    users = append(users, load(id))
}
```

---

# HTTP Client

## Bad

```go
client := http.Client{}

client.Do(req)
```

Created every request.

## Best

```go
var client = &http.Client{
    Timeout: 5 * time.Second,
}
```

Reuse clients.

---

# SQL

## Bad

```go
query := "SELECT * FROM users WHERE id=" + id
```

## Best

```go
db.QueryContext(
    ctx,
    "SELECT * FROM users WHERE id = ?",
    id,
)
```

Always use parameterized queries.

---

# Testing

## Bad

```go
func TestUser(t *testing.T) {

}
```

## Better

```go
func TestCreateUser(t *testing.T) {

}
```

## Best

```go
func TestCreateUser_InvalidEmail(t *testing.T) {

}
```

Describe behavior.

---

# Table-Driven Tests

## Best

```go
tests := []struct {
    name string
    input string
    want string
}{
    {
        name: "valid email",
    },
    {
        name: "invalid email",
    },
}
```

Prefer table-driven tests for multiple scenarios.

---

# Package Organization

## Bad

```text
controllers/
services/
repositories/
models/
```

## Better

```text
user/
order/
billing/
```

## Best

```text
internal/

    user/
        service.go
        repository.go
        handler.go
        model.go

    order/
        service.go
        repository.go
        handler.go
```

Organize by feature.

---

# Utility Packages

## Bad

```text
utils/
helpers/
common/
misc/
```

## Best

Keep helper functions inside the package where they are used.

Extract shared functionality only after multiple legitimate consumers exist.

---

# Comments

## Bad

```go
// Increment i
i++
```

## Best

```go
// Retry once because the upstream API occasionally returns
// transient connection failures.
```

Explain why, not what.

---

# General Rule

Whenever multiple valid implementations exist, prefer the one that is:

1. Simpler.
2. More idiomatic.
3. Easier to understand.
4. Easier to test.
5. Easier to maintain.

Readable code is almost always the best code.