# go-wide-event-logger

A Go library for **wide event logging** — one rich structured log line per unit of work instead of dozens of scattered `log.Info/Error/Debug` calls.

## What are Wide Events?

Traditional logging produces disconnected log lines:

```
INFO  Starting user update
DEBUG Fetching user from cache
DEBUG Cache miss, querying database
INFO  User updated successfully
DEBUG Broadcasting websocket event
```

Wide events replace this with **one comprehensive event** containing all context:

```json
{
  "level": "info",
  "message": "wide_event",
  "name": "update_user",
  "outcome": "success",
  "request": { "method": "PUT", "path": "/api/v1/users/abc-123" },
  "response": { "status": 200, "latency_ms": 12.4 },
  "user": { "id": "def-456", "name": "john", "role": "admin" },
  "db": { "operation": "update", "entity": "user" },
  "cache": { "hit": false }
}
```

One event per request. Every field queryable. No context lost.

## Installation

```bash
go get github.com/mxcd/go-wide-event-logger
```

## Quick Start

```go
import wideevent "github.com/mxcd/go-wide-event-logger"

func main() {
    r := gin.Default()
    r.Use(wideevent.Middleware())

    r.GET("/api/users", func(c *gin.Context) {
        we := wideevent.FromGin(c)
        we.Str("endpoint", "list_users").Success()
        c.JSON(200, users)
    })

    r.Run(":8080")
}
```

## Convention: the `we` Variable

Wide events use the variable name **`we`** — it reads as natural English:

```go
we.Success()           // "we succeeded"
we.Failure(err)        // "we failed with err"
we.UserID(id)          // "we have user ID"
we.UserName("john")    // "we have user name john"
```

## API Reference

### Event Creation

| Function | Use Case |
|----------|----------|
| `wideevent.FromGin(c)` | Get event from Gin handler (created by middleware) |
| `wideevent.FromContext(ctx)` | Get event from standard context (repository layers) |
| `wideevent.Begin(emitter, name)` | Create standalone event (background tasks, cron jobs) |

### Typed Setters

All setters return `*Event` for chaining:

```go
we.Str("key", "value")           // string
we.Int("key", 42)                // int
we.Int64("key", int64(99))       // int64
we.Float64("key", 3.14)          // float64
we.Bool("key", true)             // bool
we.Dur("key", 5*time.Second)     // time.Duration (emitted as ms)
we.Time("key", time.Now())       // time.Time (emitted as RFC3339Nano)
we.Err("key", err)               // error (nil is a no-op)
we.Interface("key", anything)    // any
we.UUID("key", uuidValue)        // fmt.Stringer
we.Set("key", anyValue)          // generic setter
```

### Convenience Builders

```go
we.Success()              // sets outcome = "success"
we.Failure(err)           // sets outcome = "failure", error = err.Error()
we.UserID(id)             // sets user.id
we.UserName("john")       // sets user.name
we.UserRole("admin")      // sets user.role
```

### Event Inspection

```go
evt.Fields()        // []wideevent.Field — snapshot of all key-value pairs
evt.FieldsMap()     // map[string]any — nested map with dot keys expanded
evt.HasError()      // bool — true if outcome="failure" or status >= 500
evt.StatusCode()    // int — response status code, or 0 if not set
```

### Nested Keys via Dots

Keys containing dots produce nested JSON:

```go
we.Set("db.operation", "update")
we.Set("db.entity", "user")
// Output: {"db": {"operation": "update", "entity": "user"}}
```

## Dev Mode

For local development, use `DevStdoutEmitter()` for human-readable, colorized output with unicode tree formatting:

```go
r.Use(wideevent.Middleware(
    wideevent.WithEmitter(wideevent.DevStdoutEmitter()),
))
```

Output:

```
┌ GET /api/v1/users/abc-123                    200  12.4ms
│
│  request   method=GET  path=/api/v1/users/abc-123
│            host=localhost:8080  client_ip=127.0.0.1
│  response  status=200  body_size=1842  latency_ms=12.4
│  user      id=def-456  name=john  role=admin
│  db        operation=update  entity=user
│  cache     hit=false
│
└ outcome=success  duration=12.5ms
```

Colors are auto-detected based on terminal capability. Use `WithColor(true/false)` to override:

```go
wideevent.DevStdoutEmitter(wideevent.WithColor(false)) // force no color
```

Use `DevWriterEmitter` for custom output targets:

```go
wideevent.DevWriterEmitter(os.Stderr, wideevent.WithColor(true))
```

Combine with JSON for file logging using `MultiEmitter`:

```go
wideevent.WithEmitter(wideevent.MultiEmitter(
    wideevent.DevStdoutEmitter(),          // pretty terminal output
    wideevent.JSONWriterEmitter(logFile),   // structured JSON to file
))
```

### Path Categories & Muting

Group request paths into named categories and mute noisy ones:

```go
wideevent.DevStdoutEmitter(
    wideevent.WithCategory("assets", "/src/assets", "/static"),
    wideevent.WithCategory("health", "/health", "/ready"),
    wideevent.WithCategory("api", "/api"),
    wideevent.WithMuteCategories("assets", "health"),
)
```

Muted categories produce no output. Non-muted categories show a label in the header:

```
┌ GET /api/v1/users/abc-123  [api]  200  12.4ms
│
│  ...
└ outcome=success  duration=12.5ms
```

Categories use prefix matching — `WithCategory("assets", "/src/assets")` matches `/src/assets/logo.png`, `/src/assets/styles.css`, etc.

## Setup Options

```go
r.Use(wideevent.Middleware(
    // Custom output target (default: JSON to stdout)
    wideevent.WithEmitter(wideevent.JSONWriterEmitter(os.Stderr)),

    // Tail sampling strategy (default: AlwaysSample)
    wideevent.WithSampler(wideevent.CompositeSampler(
        wideevent.AlwaysOnError(),
        wideevent.Rate(100),
    )),

    // Add fields before handler (e.g., trace ID)
    wideevent.WithPreEnricher(func(we *wideevent.Event, c *gin.Context) {
        we.Str("trace.id", c.GetHeader("X-Trace-ID"))
    }),

    // Add fields after handler (e.g., user context from auth)
    wideevent.WithPostEnricher(func(we *wideevent.Event, c *gin.Context) {
        if user := getUser(c); user != nil {
            we.UserID(user.ID).UserName(user.Name).UserRole(user.Role)
        }
    }),

    // Skip paths (e.g., health checks)
    wideevent.WithSkipPaths("/health", "/ready"),
))
```

## Examples

### Gin Request Logging

```go
package main

import (
    "github.com/gin-gonic/gin"
    wideevent "github.com/mxcd/go-wide-event-logger"
)

func main() {
    r := gin.New()
    r.Use(wideevent.Middleware(
        wideevent.WithSkipPaths("/health"),
        wideevent.WithPostEnricher(func(we *wideevent.Event, c *gin.Context) {
            if u := getAuthUser(c); u != nil {
                we.UserID(u.ID).UserName(u.Username).UserRole(string(u.Role))
            }
        }),
    ))

    r.PUT("/api/v1/users/:id", func(c *gin.Context) {
        we := wideevent.FromGin(c)
        we.Str("endpoint", "update_user")

        id := c.Param("id")
        user, err := updateUser(c.Request.Context(), id)
        if err != nil {
            we.Failure(err)
            c.JSON(500, gin.H{"error": "internal"})
            return
        }
        we.Success()
        c.JSON(200, user)
    })

    r.Run(":8080")
}
```

### Background Task Logging

```go
func (s *Service) SyncUsers(ctx context.Context) {
    we := wideevent.Begin(s.emitter, "sync_users")
    defer we.Emit()

    we.Set("source", "cron").Set("trigger", "scheduled")

    users, err := s.fetchExternalUsers(ctx)
    if err != nil {
        we.Failure(err)
        return
    }
    we.Int("batch_size", len(users)).Success()
}
```

### Repository Layer Enrichment

```go
func (r *Repository) UpdateUser(ctx context.Context, id uuid.UUID, p *UpdateParams) (*ent.User, error) {
    we := wideevent.FromContext(ctx)
    if we != nil {
        we.Set("db.operation", "update").Set("db.entity", "user")
        we.Set("cache.hit", r.userCache.Has(id.String()))
    }

    user, err := r.client.User.UpdateOneID(id).SetUsername(p.Username).Save(ctx)
    if err != nil && we != nil {
        we.Set("db.error", err.Error())
    }
    return user, err
}
```

### Custom Emitter

Implement the `Emitter` interface to send events anywhere — a database, an external service, etc. Use `evt.Fields()` for raw key-value pairs or `evt.FieldsMap()` for a pre-nested `map[string]any`.

```go
type DBLogEmitter struct {
    db *ent.Client
}

func (e *DBLogEmitter) Emit(evt *wideevent.Event) {
    fields := evt.FieldsMap() // nested map[string]any, dot keys already expanded
    data, _ := json.Marshal(fields)
    e.db.Log.Create().
        SetLevel(fields["level"].(string)).
        SetData(data).
        SaveX(context.Background())
}
```

### MultiEmitter

Use `MultiEmitter` to fan out events to multiple destinations simultaneously:

```go
r.Use(wideevent.Middleware(
    wideevent.WithEmitter(wideevent.MultiEmitter(
        wideevent.JSONStdoutEmitter(),   // structured logs to stdout
        &DBLogEmitter{db: client},        // persist to database
    )),
))
```

### Tail Sampling

```go
// Production: always keep errors, sample 1-in-100 successes
sampler := wideevent.CompositeSampler(
    wideevent.AlwaysOnError(),
    wideevent.AlwaysOnStatus(429, 503),
    wideevent.Rate(100),
)

r.Use(wideevent.Middleware(
    wideevent.WithSampler(sampler),
))
```

## JSON Output Format

Events are emitted as single JSON lines with:

- **Nested structure**: dot-separated keys become nested objects
- **Level inference**: `HasError()` or status >= 500 → `"error"`, status >= 400 → `"warn"`, else `"info"`
- **Durations**: emitted as float64 milliseconds
- **Times**: emitted as RFC3339Nano strings
- **Message**: always `"wide_event"` for easy filtering

### Automatic Fields (Middleware)

**Request fields** (set before handler):
- `request.method`, `request.path`, `request.query`, `request.host`
- `request.proto`, `request.content_length`, `request.client_ip`
- `request.user_agent`, `request.id`, `request.start_time`

**Response fields** (set after handler):
- `response.status`, `response.body_size`
- `response.latency`, `response.latency_ms`
- `response.gin_errors`, `response.gin_error.N`

## Available Samplers

| Sampler | Description |
|---------|-------------|
| `AlwaysSample()` | Emit every event (default) |
| `NeverSample()` | Emit nothing |
| `AlwaysOnError()` | Emit if event has error |
| `AlwaysOnStatus(codes...)` | Emit if status matches |
| `Rate(n)` | Emit every Nth event |
| `Probability(p)` | Emit with probability p (0.0-1.0) |
| `CompositeSampler(s...)` | Emit if ANY sampler says yes (OR logic) |

## License

MIT
