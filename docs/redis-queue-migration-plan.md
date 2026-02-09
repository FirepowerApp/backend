# Redis Queue Migration Plan

**Tracking Issue:** #10
**Status:** Planning

## Overview

Migrate from Google Cloud Tasks to a Redis-backed task queue. The application will shift from an HTTP-triggered model (Cloud Tasks pushes work via HTTP POST) to a **worker/poller model** (the application pulls work from a Redis queue, processes it asynchronously, then sleeps until the next scheduled task).

### Goals

1. **Self-hosted task broker** - Redis can run anywhere: locally via Docker, on a VPS, or through managed services (AWS ElastiCache, Redis Cloud, etc.)
2. **Scheduled task support** - Tasks are enqueued with a future execution time; the worker picks them up only when that time arrives
3. **Async processing** - When multiple tasks are ready simultaneously, each spawns its own goroutine for concurrent processing
4. **Observable queue** - A web GUI shows all enqueued tasks, their payloads, scheduled execution times, status, and history
5. **Remove GCP dependency** - Eliminate all `cloud.google.com/go/cloudtasks` imports and the Cloud Tasks emulator from the stack

---

## Current Architecture

```
┌──────────────────┐       HTTP POST        ┌─────────────────────┐
│  Cloud Tasks     │ ───────────────────────>│  Backend (HTTP)     │
│  (or emulator)   │                         │  funcframework:8080 │
└──────────────────┘                         └─────────┬───────────┘
        ^                                              │
        │          CreateTask(scheduleTime)             │
        └──────────────────────────────────────────────┘
                    (self-rescheduling loop)
```

**Flow:**
1. An external trigger (or `localCloudTasksTest`) creates an initial Cloud Task with a game payload
2. Cloud Tasks delivers the payload as an HTTP POST to the backend at the scheduled time
3. The handler processes the game data (fetch play-by-play, fetch stats, send notifications)
4. If the game isn't over, `scheduleNextCheck()` creates a **new** Cloud Task scheduled `MESSAGE_INTERVAL_SECONDS` in the future
5. This self-rescheduling loop continues until `game-end` or `execution_end` is reached

**Files with Cloud Tasks coupling:**

| File | Coupling |
|------|----------|
| `internal/tasks/client.go` | `CloudTasksClient` interface wrapping GCP SDK |
| `internal/tasks/factory.go` | Creates Cloud Tasks client (emulator or production) |
| `internal/handlers/watchgameupdates_handler.go` | `scheduleNextCheck()` builds a `taskspb.Task` with HTTP request + schedule time |
| `config/config.go` | `ProjectID`, `QueueID`, `LocationID`, `CloudTasksAddress`, `HandlerAddress` |
| `cmd/watchgameupdates/main.go` | HTTP server via `funcframework` — only exists because Cloud Tasks pushes over HTTP |
| `docker-compose.yml` | `cloudtasks-emulator` service on port 8123 |
| `localCloudTasksTest/main.go` | Test helper that creates initial task via gRPC |
| `Makefile` | References to emulator, pull logic, test sequence |
| `.env.example` | Cloud Tasks env vars |

---

## Target Architecture

```
┌────────────┐         ┌─────────────────────────────────┐
│   Redis    │<────────│  Backend (Worker)                │
│            │         │                                  │
│  Sorted    │────────>│  Poll loop:                      │
│  Sets /    │         │   1. Dequeue ready tasks         │
│  Asynq     │         │   2. Process each (goroutine)    │
│  Queues    │         │   3. Re-enqueue if not done      │
│            │         │   4. Sleep until next task        │
└────────────┘         └──────────────────────────────────┘
       │
       │
┌──────▼─────┐
│  Asynqmon  │  (Web UI on port 8980)
│  Dashboard │
└────────────┘
```

**Flow:**
1. An initial task is enqueued to Redis (via CLI tool, API call, or the scheduler service)
2. The worker (asynq) picks up the task when its scheduled time arrives
3. The handler processes game data (same logic as today — fetch, notify)
4. If the game isn't over, it enqueues a **new** task with `asynq.ProcessIn(interval)` for delayed execution
5. Loop continues until `game-end` or `execution_end`

---

## Technology Choice: Asynq

**Library:** [`hibiken/asynq`](https://github.com/hibiken/asynq)

Asynq is a Go library purpose-built for Redis-backed distributed task queues. It is the right fit because:

| Requirement | Asynq Feature |
|-------------|---------------|
| Scheduled/delayed tasks | `asynq.ProcessIn(duration)` or `asynq.ProcessAt(time)` — uses Redis sorted sets under the hood |
| Async processing | Configurable concurrency — multiple goroutines process tasks in parallel |
| Observable queue + GUI | **Asynqmon** — standalone web UI showing queues, task payloads, scheduled times, retries, history |
| Host anywhere | Only needs a Redis connection string — works with local Docker Redis, AWS ElastiCache, Redis Cloud, etc. |
| Simple Go API | Define task types as constants, implement `asynq.Handler` interface |
| Retry / dead-letter | Built-in retry with backoff, dead-letter queue for failed tasks |
| Unique tasks | Deduplication to prevent double-scheduling the same game check |

**Alternatives considered:**
- **Raw Redis sorted sets** — Lower level, would require reimplementing scheduling, retries, monitoring. Asynq wraps this cleanly.
- **Bull/BullMQ** — Node.js ecosystem, not idiomatic for a Go project.
- **Celery** — Python ecosystem.
- **Machinery** — Go library but less actively maintained than Asynq; weaker monitoring story.

---

## Implementation Plan

### Phase 1: Add Redis + Asynq infrastructure

**1.1 Add dependencies**

```bash
cd watchgameupdates
go get github.com/hibiken/asynq
```

**1.2 Add Redis to `docker-compose.yml`**

Replace the `cloudtasks-emulator` service with Redis and Asynqmon:

```yaml
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 30
    networks:
      - net
    profiles:
      - home
      - test
      - default

  asynqmon:
    image: hibiken/asynqmon:latest
    ports:
      - "8980:8080"
    environment:
      - REDIS_ADDR=redis:6379
    depends_on:
      redis:
        condition: service_healthy
    networks:
      - net
    profiles:
      - home
      - test
      - default

volumes:
  redis-data:
```

**1.3 Update `config/config.go`**

Replace Cloud Tasks config fields with Redis config:

```go
type Config struct {
    Env                    string
    RedisAddress           string // e.g. "redis:6379" or "localhost:6379"
    RedisPassword          string // optional, for authenticated Redis
    RedisDB               int    // default 0
    MessageIntervalSeconds int
    // Retain API and notification config as-is
}
```

Remove: `ProjectID`, `QueueID`, `LocationID`, `UseEmulator`, `CloudTasksAddress`, `HandlerAddress`

Add env vars: `REDIS_ADDRESS`, `REDIS_PASSWORD`, `REDIS_DB`

**1.4 Update `.env.example`**

```
APP_ENV=
REDIS_ADDRESS=redis:6379
REDIS_PASSWORD=
REDIS_DB=0
MESSAGE_INTERVAL_SECONDS=60

PLAYBYPLAY_API_BASE_URL=
STATS_API_BASE_URL=
DISCORD_BOT_TOKEN=
```

---

### Phase 2: Create task types and handlers

**2.1 New file: `internal/tasks/types.go`**

Define task type constants and payload serialization:

```go
package tasks

import (
    "encoding/json"
    "watchgameupdates/internal/models"
    "github.com/hibiken/asynq"
)

const (
    TypeWatchGameUpdates = "game:watch_updates"
)

// Create a new asynq task from a game payload
func NewWatchGameUpdatesTask(payload models.Payload) (*asynq.Task, error) {
    data, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }
    return asynq.NewTask(TypeWatchGameUpdates, data), nil
}

// Deserialize payload from an asynq task
func ParseWatchGameUpdatesPayload(t *asynq.Task) (models.Payload, error) {
    var payload models.Payload
    err := json.Unmarshal(t.Payload(), &payload)
    return payload, err
}
```

**2.2 New file: `internal/tasks/handler.go`**

Implement `asynq.Handler` that contains the core game-check logic (extracted from the current HTTP handler):

```go
package tasks

import (
    "context"
    "log"
    "time"

    "watchgameupdates/config"
    "watchgameupdates/internal/notification"
    "watchgameupdates/internal/services"

    "github.com/hibiken/asynq"
)

type WatchGameUpdatesHandler struct {
    cfg    *config.Config
    client *asynq.Client
}

func NewWatchGameUpdatesHandler(cfg *config.Config, client *asynq.Client) *WatchGameUpdatesHandler {
    return &WatchGameUpdatesHandler{cfg: cfg, client: client}
}

func (h *WatchGameUpdatesHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
    payload, err := ParseWatchGameUpdatesPayload(t)
    if err != nil {
        return err
    }

    // Check execution window
    if payload.ExecutionEnd != nil {
        executionEnd, err := time.Parse(time.RFC3339, *payload.ExecutionEnd)
        if err != nil {
            return err
        }
        if time.Now().After(executionEnd) {
            log.Printf("Execution end reached for game %s, stopping.", payload.Game.ID)
            return nil // Don't reschedule
        }
    }

    // Core game processing logic (same as current handler)
    fetcher := &services.HTTPGameDataFetcher{}
    var notificationService *notification.Service
    if payload.ShouldNotify != nil {
        notificationService = notification.NewServiceWithNotificationFlag(*payload.ShouldNotify)
    } else {
        notificationService = notification.NewService()
    }
    defer notificationService.Close()

    lastPlay := services.FetchPlayByPlay(payload.Game.ID)

    recomputeTypes := map[string]struct{}{
        "blocked-shot": {}, "missed-shot": {}, "shot-on-goal": {},
        "goal": {}, "game-end": {},
    }

    if _, ok := recomputeTypes[lastPlay.TypeDescKey]; ok {
        requiredKeys := notificationService.GetAllRequiredDataKeys()
        gameData, err := fetcher.FetchAndParseGameData(payload.Game.ID, requiredKeys)
        if err != nil {
            log.Printf("ERROR fetching game data for %s: %v", payload.Game.ID, err)
        }
        // Handle shootout score adjustment
        if lastPlay.TypeDescKey == "game-end" {
            if homeGoals, ok1 := gameData["homeTeamGoals"]; ok1 {
                if awayGoals, ok2 := gameData["awayTeamGoals"]; ok2 {
                    if homeGoals == awayGoals {
                        // adjustScoreForShootout(gameData) — extract to shared util
                    }
                }
            }
        }
        notificationService.SendGameEventNotifications(payload.Game, gameData)
    }

    // Reschedule if game is not over
    if services.ShouldReschedule(payload, lastPlay) {
        return h.scheduleNextCheck(payload)
    }

    return nil
}

func (h *WatchGameUpdatesHandler) scheduleNextCheck(payload models.Payload) error {
    task, err := NewWatchGameUpdatesTask(payload)
    if err != nil {
        return err
    }

    interval := time.Duration(h.cfg.MessageIntervalSeconds) * time.Second
    info, err := h.client.Enqueue(task, asynq.ProcessIn(interval))
    if err != nil {
        return err
    }

    log.Printf("Scheduled next check for game %s, task ID: %s, scheduled at: %v",
        payload.Game.ID, info.ID, time.Now().Add(interval).Format(time.RFC3339))
    return nil
}
```

---

### Phase 3: Rewrite the entrypoint

**3.1 Rewrite `cmd/watchgameupdates/main.go`**

Replace the HTTP server with an Asynq worker:

```go
package main

import (
    "log"

    "watchgameupdates/config"
    "watchgameupdates/internal/tasks"

    "github.com/hibiken/asynq"
)

func main() {
    log.SetFlags(0)

    cfg := config.LoadConfig()

    // Asynq Redis connection
    redisOpt := asynq.RedisClientOpt{
        Addr:     cfg.RedisAddress,
        Password: cfg.RedisPassword,
        DB:       cfg.RedisDB,
    }

    // Client for enqueuing follow-up tasks
    client := asynq.NewClient(redisOpt)
    defer client.Close()

    // Worker server
    srv := asynq.NewServer(redisOpt, asynq.Config{
        Concurrency: 10,
        Queues: map[string]int{
            "default": 1,
        },
    })

    // Register handlers
    mux := asynq.NewServeMux()
    handler := tasks.NewWatchGameUpdatesHandler(cfg, client)
    mux.HandleFunc(tasks.TypeWatchGameUpdates, handler.ProcessTask)

    log.Printf("Starting asynq worker, connected to Redis at %s", cfg.RedisAddress)

    if err := srv.Run(mux); err != nil {
        log.Fatalf("Failed to start worker: %v", err)
    }
}
```

This fundamentally changes the application from an HTTP server to a **long-running worker process** that:
- Connects to Redis
- Polls for tasks whose scheduled time has arrived
- Processes each in a goroutine (up to `Concurrency` limit)
- Sleeps when no tasks are ready

---

### Phase 4: Create a task enqueue CLI/tool

**4.1 Replace `localCloudTasksTest/` with `cmd/enqueue/main.go`**

A CLI tool to enqueue initial game-watching tasks into Redis:

```go
package main

import (
    "encoding/json"
    "flag"
    "log"
    "time"

    "watchgameupdates/config"
    "watchgameupdates/internal/models"
    "watchgameupdates/internal/tasks"

    "github.com/hibiken/asynq"
)

func main() {
    gameID := flag.String("game", "", "NHL game ID to watch")
    duration := flag.Duration("duration", 12*time.Minute, "Max execution duration")
    delay := flag.Duration("delay", 0, "Delay before first execution")
    notify := flag.Bool("notify", true, "Send Discord notifications")
    flag.Parse()

    if *gameID == "" {
        log.Fatal("--game flag is required")
    }

    cfg := config.LoadConfig()

    client := asynq.NewClient(asynq.RedisClientOpt{
        Addr:     cfg.RedisAddress,
        Password: cfg.RedisPassword,
        DB:       cfg.RedisDB,
    })
    defer client.Close()

    executionEnd := time.Now().Add(*duration).Format(time.RFC3339)
    payload := models.Payload{
        Game:         models.Game{ID: *gameID},
        ExecutionEnd: &executionEnd,
        ShouldNotify: notify,
    }

    task, err := tasks.NewWatchGameUpdatesTask(payload)
    if err != nil {
        log.Fatalf("Failed to create task: %v", err)
    }

    opts := []asynq.Option{}
    if *delay > 0 {
        opts = append(opts, asynq.ProcessIn(*delay))
    }

    info, err := client.Enqueue(task, opts...)
    if err != nil {
        log.Fatalf("Failed to enqueue task: %v", err)
    }

    payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
    log.Printf("Task enqueued successfully:")
    log.Printf("  ID:        %s", info.ID)
    log.Printf("  Queue:     %s", info.Queue)
    log.Printf("  Game:      %s", *gameID)
    log.Printf("  Payload:   %s", payloadJSON)
}
```

---

### Phase 5: Clean up Cloud Tasks code

**5.1 Delete files:**
- `internal/tasks/client.go` (CloudTasksClient interface)
- `internal/tasks/factory.go` (Cloud Tasks client factory)
- `internal/handlers/watchgameupdates_handler.go` — extract reusable logic first, then delete
- `localCloudTasksTest/` directory

**5.2 Remove dependencies from `go.mod`:**
```bash
go get -u  # update deps
go mod tidy  # remove unused Cloud Tasks, gRPC, protobuf, funcframework deps
```

Dependencies to be removed:
- `cloud.google.com/go/cloudtasks`
- `github.com/GoogleCloudPlatform/functions-framework-go`
- `google.golang.org/api`
- `google.golang.org/grpc` (unless still needed)
- `google.golang.org/protobuf`
- Various transitive `cloud.google.com/go/*` packages

**5.3 Update `docker-compose.yml`:**
- Remove `cloudtasks-emulator` service
- Remove all `depends_on` references to it
- Add `redis` and `asynqmon` services (from Phase 1)
- Backend `depends_on` changes to `redis`

**5.4 Update `docker-compose.home.yml` and `docker-compose.test.yml`:**
- Remove `cloudtasks-emulator` references
- Update `depends_on` to reference `redis`

**5.5 Update `Makefile`:**
- Remove `pull-with-retry` target for cloud tasks emulator image
- Remove `shell-cloudtasks` target
- Remove `logs-cloudtasks` target
- Update `test` target to use `cmd/enqueue` instead of `localCloudTasksTest`
- Update help text and service URLs (add Asynqmon dashboard URL)

**5.6 Update `.env.example`:**
- Remove: `CLOUD_TASKS_EMULATOR_HOST`, `GCP_PROJECT_ID`, `GCP_LOCATION`, `CLOUD_TASKS_QUEUE`, `HANDLER_HOST`
- Add: `REDIS_ADDRESS`, `REDIS_PASSWORD`, `REDIS_DB`

---

### Phase 6: Update Dockerfile

The Dockerfile remains mostly the same but no longer needs to expose an HTTP port since the app is a worker, not a server:

```dockerfile
FROM golang:1.23.3 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/watchgameupdates

FROM gcr.io/distroless/static
WORKDIR /
COPY --from=builder /app/main .
CMD ["/main"]
```

Remove `ENV PORT=8080` and `EXPOSE 8080` — the worker connects **out** to Redis, it doesn't listen for incoming connections.

---

## Monitoring with Asynqmon

Asynqmon provides a web dashboard (included in docker-compose) that shows:

- **Queues** — active, pending, scheduled, retry, archived task counts
- **Task details** — click any task to see its full JSON payload
- **Scheduled tasks** — shows when each task will be picked up, with countdown timers
- **Task history** — completed and failed tasks with timestamps
- **Real-time updates** — auto-refreshes

Access at `http://localhost:8980` when running locally.

For production or remote deployments, Asynqmon can connect to any Redis instance by setting `REDIS_ADDR`. It can also be deployed as a standalone container pointing at your production Redis.

Additionally, the `asynq` CLI tool can be installed for terminal-based queue inspection:
```bash
go install github.com/hibiken/asynq/tools/asynq@latest
asynq dash --uri redis://localhost:6379
```

---

## Migration Summary

| What | Before (Cloud Tasks) | After (Redis + Asynq) |
|------|---------------------|----------------------|
| **Task broker** | Google Cloud Tasks (or emulator) | Redis |
| **Trigger model** | Push (HTTP POST to backend) | Pull (worker polls Redis) |
| **Scheduling** | `taskspb.Task.ScheduleTime` | `asynq.ProcessIn()` / `asynq.ProcessAt()` |
| **Rescheduling** | `CreateTask()` with new HTTP request | `client.Enqueue()` with `ProcessIn(interval)` |
| **Entrypoint** | HTTP server (funcframework:8080) | Asynq worker (connects to Redis) |
| **Monitoring** | None (emulator has no UI) | Asynqmon web dashboard |
| **Local dev** | Cloud Tasks emulator Docker image | Redis Docker image (standard) |
| **Production hosting** | GCP Cloud Tasks | Any Redis (self-hosted, ElastiCache, Redis Cloud, etc.) |
| **GCP dependency** | Required (SDK, credentials, project/queue/location) | None |
| **Config vars** | 6 Cloud Tasks-specific vars | 3 Redis vars (`REDIS_ADDRESS`, `REDIS_PASSWORD`, `REDIS_DB`) |
| **External deps** | ~15 GCP/gRPC/protobuf packages | 1 package (`hibiken/asynq`) |

---

## File Change Summary

| Action | File | Description |
|--------|------|-------------|
| **Create** | `internal/tasks/types.go` | Task type constants and payload serialization |
| **Create** | `internal/tasks/handler.go` | Asynq handler with game processing logic |
| **Create** | `cmd/enqueue/main.go` | CLI tool to enqueue initial tasks |
| **Rewrite** | `cmd/watchgameupdates/main.go` | HTTP server → Asynq worker |
| **Rewrite** | `config/config.go` | Replace Cloud Tasks config with Redis config |
| **Rewrite** | `docker-compose.yml` | Replace cloudtasks-emulator with redis + asynqmon |
| **Update** | `docker-compose.home.yml` | Update depends_on |
| **Update** | `docker-compose.test.yml` | Update depends_on |
| **Update** | `Dockerfile` | Remove PORT/EXPOSE (worker, not server) |
| **Update** | `Makefile` | Update targets, remove emulator references |
| **Update** | `.env.example` | Replace Cloud Tasks vars with Redis vars |
| **Update** | `go.mod` | Add asynq, remove GCP packages |
| **Delete** | `internal/tasks/client.go` | CloudTasksClient interface (no longer needed) |
| **Delete** | `internal/tasks/factory.go` | Cloud Tasks client factory (no longer needed) |
| **Delete** | `internal/handlers/watchgameupdates_handler.go` | Logic moves to `internal/tasks/handler.go` |
| **Delete** | `localCloudTasksTest/` | Replaced by `cmd/enqueue/` |

---

## Risk Considerations

1. **Data in-flight** — If there are scheduled Cloud Tasks in production when we switch, they'll be lost. Mitigation: deploy during a window when no games are actively being tracked, or run both systems briefly in parallel.

2. **Redis persistence** — By default Redis is in-memory. For durability, enable RDB snapshots or AOF in production Redis config. Asynq handles Redis restarts gracefully since scheduled tasks are stored in sorted sets.

3. **Single point of failure** — Redis becomes the single critical dependency. For production, use Redis Sentinel or a managed Redis service with replication.

4. **Retry behavior change** — Cloud Tasks has its own retry logic. Asynq also has configurable retries (`MaxRetry`, `RetryDelayFunc`). The default retry behavior should be reviewed to match the desired semantics (current code doesn't rely on Cloud Tasks retries — it self-reschedules).
