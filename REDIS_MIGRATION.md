# Redis Message Queue Migration Guide

This document explains the Redis-based message queue implementation that replaces Google Cloud Tasks.

## Architecture Overview

The new architecture uses **Redis + Asynq** for message queuing and delayed task execution:

```
┌─────────────────────────────────────────────────┐
│  Backend Container (Dual-mode)                  │
│                                                 │
│  ┌──────────────────┐   ┌──────────────────┐  │
│  │  HTTP Server     │   │  Asynq Worker    │  │
│  │  :8080          │   │  (Concurrent)    │  │
│  │                  │   │                  │  │
│  │  - Health checks │   │  - Processes     │  │
│  │  - External APIs │   │    messages      │  │
│  │  - Enqueues tasks│   │  - 10 goroutines │  │
│  └──────────────────┘   └──────────────────┘  │
│           │                      ▲             │
└───────────┼──────────────────────┼─────────────┘
            │                      │
            │ Enqueue              │ Consume
            ▼                      │
     ┌──────────────────────────────────┐
     │       Redis Container            │
     │       :6379                      │
     │  - Sorted sets (delayed queue)   │
     │  - Message persistence (AOF)     │
     └──────────────────────────────────┘
```

### Key Components

1. **Redis** - Message broker and task scheduler
2. **Backend** - Runs both HTTP server and Asynq worker concurrently
3. **Asynq** - Go library that provides delayed queue functionality

## How It Works

### Message Processing Flow

1. **Initial Task Creation**
   ```bash
   cd localRedisTest
   go run main.go --game 2024030411 --duration 12m
   ```

2. **Backend Receives Task**
   - Asynq worker polls Redis (using blocking operations - efficient!)
   - Worker fetches task when scheduled time arrives
   - Runs `handleGameCheckTask()` in a goroutine (concurrent processing)

3. **Game Processing**
   - Fetches play-by-play data from NHL API
   - Checks for scoring events
   - Fetches xG data from MoneyPuck if needed
   - Sends Discord notification

4. **Rescheduling**
   - If game is not over, calls `ScheduleNextCheck()`
   - Creates new task in Redis with 60-second delay
   - Loop continues until game ends or execution window expires

### Concurrency

The Asynq worker is configured for **10 concurrent tasks**:

```go
asynq.Config{
    Concurrency: 10,  // Process up to 10 games simultaneously
}
```

This means you can monitor 10 games at once, each in its own goroutine!

### Retry Logic

Failed tasks automatically retry with exponential backoff:

- **1st retry**: 30 seconds
- **2nd retry**: 1 minute
- **3rd retry**: 2 minutes
- **Max retries**: 3

### Priority Queues

Three queues with different priorities:

| Queue | Priority | Use Case |
|-------|----------|----------|
| `critical` | 6 | Immediate tasks, high-priority games |
| `default` | 3 | Regular game monitoring |
| `low` | 1 | Background tasks |

## Setup Instructions

### 1. Install Dependencies

```bash
cd watchgameupdates
go mod tidy
```

### 2. Configure Environment

Update `.env.local` or `.env.home`:

```env
APP_ENV=local
REDIS_ADDRESS=redis:6379
REDIS_PASSWORD=              # Optional, for production
HANDLER_HOST=http://backend:8080

# API endpoints
PLAYBYPLAY_API_BASE_URL=http://mockdataapi:8125
STATS_API_BASE_URL=http://mockdataapi:8124

# Discord
DISCORD_BOT_TOKEN=your-token-here
```

### 3. Start Services

```bash
# Start Redis and backend
docker compose up -d

# Or use make commands
make home   # With live APIs
make test   # With mock APIs
```

### 4. Submit Initial Task

```bash
cd localRedisTest
go mod download
go run main.go --game 2024030411 --duration 12m

# Options:
#   --game string      Game ID (default "2024030411")
#   --redis string     Redis address (default "localhost:6379")
#   --duration duration Duration (default 12m)
#   --now              Schedule immediately instead of 10s delay
```

### 5. Monitor Logs

```bash
docker compose logs -f backend

# Or use make
make logs
```

## Differences from Cloud Tasks

| Feature | Cloud Tasks | Redis + Asynq |
|---------|-------------|---------------|
| **Message Broker** | GCP Cloud Tasks | Redis |
| **Delayed Execution** | Native | Asynq (sorted sets) |
| **Concurrency** | Auto-scaling | Configurable (10 workers) |
| **Retries** | Configurable | Exponential backoff |
| **Local Dev** | Emulator | Same Redis image |
| **Production** | Managed service | Redis (Memorystore/ElastiCache) |
| **Monitoring** | GCP Console | Asynqmon UI |
| **Cost** | Pay per task | Redis hosting cost |

## Files Changed

### Created
- `/watchgameupdates/internal/handlers/scheduler.go` - Asynq task scheduling
- `/localRedisTest/main.go` - Test client for submitting tasks
- `/watchgameupdates/.env.local` - Local environment config
- `/watchgameupdates/.env.home` - Home environment config

### Modified
- `/watchgameupdates/cmd/watchgameupdates/main.go` - Dual-mode server (HTTP + Asynq worker)
- `/watchgameupdates/internal/handlers/watchgameupdates_handler.go` - Removed Cloud Tasks code
- `/watchgameupdates/config/config.go` - Redis configuration
- `/docker-compose.yml` - Redis instead of Cloud Tasks emulator
- `/watchgameupdates/go.mod` - Asynq dependencies

### Removed
- `/watchgameupdates/internal/tasks/` - Old Cloud Tasks client code

## Monitoring

### View Queue Stats

You can add an HTTP endpoint to view queue statistics:

```go
http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
    stats, _ := handlers.GetQueueStats(r.Context())
    json.NewEncoder(w).Encode(stats)
})
```

### Asynqmon Web UI (Optional)

Add to `docker-compose.yml`:

```yaml
asynqmon:
  image: hibiken/asynqmon:latest
  ports:
    - "8090:8090"
  environment:
    - REDIS_ADDR=redis:6379
  depends_on:
    - redis
```

Access at `http://localhost:8090` to see:
- Queue status
- Task history
- Active workers
- Retry statistics

### Redis CLI

```bash
# Connect to Redis
docker compose exec redis redis-cli

# View all keys
KEYS *

# Check queue length
LLEN asynq:queues:default

# View scheduled tasks
ZRANGE asynq:scheduled 0 -1 WITHSCORES

# Flush all data (careful!)
FLUSHALL
```

## Production Deployment

### Use Managed Redis

**Google Cloud (Memorystore):**
```env
REDIS_ADDRESS=10.x.x.x:6379
REDIS_PASSWORD=your-secure-password
```

**AWS (ElastiCache):**
```env
REDIS_ADDRESS=your-redis.cache.amazonaws.com:6379
REDIS_PASSWORD=your-secure-password
```

### Enable TLS (Production)

Update Asynq client configuration:

```go
redisOpt := asynq.RedisClientOpt{
    Addr:      cfg.RedisAddress,
    Password:  cfg.RedisPassword,
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
    },
}
```

### Scale Workers

You can run multiple backend instances, each with 10 workers:

```bash
# Kubernetes example
replicas: 3  # 30 total workers (3 pods × 10 workers)
```

All instances share the same Redis, so tasks are distributed automatically.

## Troubleshooting

### Tasks Not Processing

1. Check Redis connection:
   ```bash
   docker compose exec redis redis-cli ping
   ```

2. Check backend logs:
   ```bash
   docker compose logs backend
   ```

3. Verify task was enqueued:
   ```bash
   docker compose exec redis redis-cli
   > KEYS asynq:*
   ```

### High Memory Usage

Redis stores all pending/scheduled tasks in memory. Monitor with:

```bash
docker stats redis
```

If memory is high:
- Reduce `ExecutionEnd` duration
- Archive completed tasks more aggressively
- Use Redis eviction policies

### Worker Crashes

Check for:
- Panics in game processing logic
- API timeouts (NHL/MoneyPuck)
- Discord notification failures

All errors are logged and trigger retries automatically.

## Testing

### Unit Tests

```bash
cd watchgameupdates
go test ./...
```

### Integration Test

```bash
make test
```

This will:
1. Start Redis, backend, and mock APIs
2. Submit a test game task
3. Monitor logs for completion
4. Clean up containers

## Rollback Plan

If you need to rollback to Cloud Tasks:

1. Checkout previous commit:
   ```bash
   git checkout HEAD~1
   ```

2. Restart with Cloud Tasks emulator:
   ```bash
   docker compose down
   docker compose up -d
   ```

The old Cloud Tasks code is preserved in git history.

## FAQ

**Q: Why not use Cloud Pub/Sub instead?**
A: Pub/Sub doesn't support delayed message delivery, which is required for the 60-second polling interval.

**Q: Can I use this with Cloud Run?**
A: Yes! Deploy backend to Cloud Run and use Memorystore for Redis. The worker runs inside the same container.

**Q: How much does Redis cost vs Cloud Tasks?**
A:
- Cloud Tasks: ~$0.40 per million tasks
- Memorystore: ~$50/month for small instance (fixed cost)

For low volumes, Cloud Tasks is cheaper. For high volumes (thousands of games), Redis is more cost-effective.

**Q: What happens if Redis crashes?**
A:
- AOF persistence enabled: All tasks restored on restart
- In-flight tasks: May be re-executed (should be idempotent)
- New tasks: Will fail to enqueue until Redis is back

**Q: Can I use a different message queue?**
A: Yes! The code is structured so you could swap Asynq for:
- RabbitMQ (with delayed message plugin)
- AWS SQS (with delay seconds)
- Bull MQ (Node.js + Redis)

Just implement a new scheduler that matches the `ScheduleNextCheck()` interface.

## Support

For issues or questions:
- Check logs: `docker compose logs -f backend`
- Redis CLI: `docker compose exec redis redis-cli`
- GitHub Issues: [Your repo]/issues
