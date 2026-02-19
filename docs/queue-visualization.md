# Queue Visualization with Asynqmon

When running the backend in **worker mode** (`-mode=worker`), all tasks flow through a Redis queue managed by [Asynq](https://github.com/hibiken/asynq). The [Asynqmon](https://github.com/hibiken/asynqmon) web dashboard provides real-time visibility into the queue.

## Accessing the Dashboard

When using `docker-compose.redis.yml`, Asynqmon is available at:

```
http://localhost:8980
```

## What You Can See

### Queues Overview
- **Active** — tasks currently being processed by workers
- **Pending** — tasks ready to be picked up immediately
- **Scheduled** — tasks with a future execution time (e.g., the next game check in 60 seconds)
- **Retry** — tasks that failed and are waiting for automatic retry
- **Archived** — permanently failed tasks (exceeded max retries)

### Task Details
Click any task to inspect:
- **Task ID** — unique identifier
- **Type** — task type (e.g., `game:watch_updates`)
- **Payload** — full JSON payload including game ID, execution end, notification settings
- **State** — current state (scheduled, pending, active, etc.)
- **Queue** — which queue the task belongs to
- **Created At / Processed At** — timing information
- **Retry count** — how many times the task has been retried

### Scheduled Tasks
The Scheduled tab shows all future tasks with:
- When each task will be picked up (countdown timer)
- The full payload contents
- Options to delete or run immediately

## Running Asynqmon Standalone

To connect Asynqmon to any Redis instance (local, remote, or cloud):

```bash
docker run --rm -p 8980:8080 \
  -e REDIS_ADDR=<your-redis-host>:6379 \
  -e REDIS_PASSWORD=<optional-password> \
  -e REDIS_DB=0 \
  hibiken/asynqmon:latest
```

## CLI Alternative

For terminal-based inspection, the `asynq` CLI tool can be installed:

```bash
go install github.com/hibiken/asynq/tools/asynq@latest
asynq dash --uri redis://localhost:6379
```

This opens an interactive terminal dashboard with similar capabilities to the web UI.
