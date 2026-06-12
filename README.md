# Firepower Backend

A Go-based backend service for tracking and managing NHL game updates. Supports two task queue backends: **Google Cloud Tasks** (HTTP mode) and **Redis via Asynq** (worker mode), selectable at runtime with a `--mode` flag.

## Prerequisites

- **Go 1.23.3+** - [Install Go](https://go.dev/doc/install)
- **Podman 5.0+** - [Install Podman](https://podman.io/docs/installation) (macOS: `brew install podman podman-compose`)
- **Make** - Usually pre-installed on Linux/macOS

## Quick Start

```bash
# Start with live NHL/MoneyPuck APIs (.env.home)
make live

# Or start with the mock game data emulator (.env.local)
make emulator

# View logs
make logs

# Stop services
make stop
```

Your services will be available at:
- Backend: http://localhost:8080
- Cloud Tasks Emulator: http://localhost:8123
- Mock NHL API (emulator mode only): http://localhost:8125
- Mock MoneyPuck API (emulator mode only): http://localhost:8124

### Quick Start (Redis Worker Mode)

```bash
# Start Redis + Asynqmon + backend worker
make redis-up

# Option A: Enqueue today's games automatically via the scheduler
make redis-schedule

# Option B: Enqueue a single game manually
make build-enqueue
./bin/enqueue --game=2024030411

# View task queue dashboard
open http://localhost:8980

# View logs
make redis-logs

# Stop
make redis-stop
```

Your services will be available at:
- Asynqmon Dashboard: http://localhost:8980
- Redis: localhost:6379

## Project Structure

```
backend/
├── watchgameupdates/            # Main service
│   ├── cmd/
│   │   ├── watchgameupdates/    # App entry point (-mode=http or -mode=worker)
│   │   └── enqueue/             # CLI tool to enqueue tasks into Redis
│   ├── internal/
│   │   ├── handlers/            # HTTP handlers (Cloud Tasks mode)
│   │   ├── services/            # Shared business logic & game processor
│   │   ├── tasks/               # Cloud Tasks client + Asynq task types/handler
│   │   ├── notification/        # Notifier interface, Discord, and service dispatcher
│   │   │   ├── liveactivity/    # iOS Live Activity APNs broadcast push
│   │   │   └── notifiers/       # Factory: reads NOTIFIERS env and wires notifiers
│   │   └── models/              # Data models
│   ├── config/                  # Configuration management
│   ├── Dockerfile               # Container definition (HTTP mode)
│   └── Dockerfile.worker        # Container definition (worker mode)
├── localCloudTasksTest/         # Test client for Cloud Tasks
├── k8s/                         # Kubernetes manifests (kustomize)
│   ├── base/
│   │   ├── app/                 # Workloads + ConfigMap (owned by deploy pipeline)
│   │   └── infra/               # Services (owned by infrastructure pipeline)
│   └── overlays/
│       ├── production/          # firepower namespace, prod config
│       └── staging/             # firepower-staging namespace, silent notifiers
├── docs/                        # Documentation
│   └── queue-visualization.md   # Asynqmon dashboard guide
├── docker-compose.yml           # Cloud Tasks orchestration
├── docker-compose.redis.yml     # Redis queue orchestration
├── Makefile                     # Development commands
└── README.md                    # This file
```

## Development

### Available Commands

Run `make help` to see all available commands:

```bash
# Cloud Tasks mode
make live                    # Start backend + tasks emulator with live APIs (.env.home)
make emulator                # Start backend + tasks emulator + mock game data APIs (.env.local)
make stop                    # Stop all running containers
make logs                    # Follow logs from running containers
make schedule                # Start full system and run scheduler with live NHL data
make schedule-test           # Start full system and run scheduler with mock data
make schedule-team TEAM=TOR [DATE=YYYY-MM-DD]  # Run scheduler for one team
make watch TEAM=COL          # Live e2e test: schedule today's real game and tail logs

# Redis mode
make redis-up                # Start Redis + Asynqmon + worker backend
make redis-test              # Start Redis test environment (with mock APIs)
make redis-schedule          # Start Redis environment and run scheduler with live data
make redis-schedule-test     # Start Redis environment and run scheduler with mock data
make redis-schedule-team TEAM=TOR [DATE=YYYY-MM-DD]  # Run Redis scheduler for one team
make redis-stop              # Stop Redis containers
make redis-logs              # View Redis worker logs
make build-enqueue           # Build the Redis enqueue CLI tool
```

### Environment Modes

The system supports two task queue backends and multiple environment modes:

#### Live Mode
Uses real NHL and MoneyPuck APIs for development and testing with live data.

```bash
make live
```

Configuration: `watchgameupdates/.env.home`

#### Emulator Mode (Mock APIs)
Pulls the `blnelson/firepowermockdataserver:latest` image and runs the stack against mock NHL and MoneyPuck endpoints. Useful for offline development and reproducible tests.

```bash
make emulator
```

Configuration: `watchgameupdates/.env.local`

**Using a Locally Built Mock API**

If you're developing changes to the gameDataEmulator alongside backend changes, build the emulator image locally from your branch and start the stack directly — bypassing the `podman pull` that `make emulator` and `make schedule-test` run first.

```bash
# 1. Build the emulator image from your local branch
cd ../gameDataEmulator
git checkout your-branch
podman build -t docker.io/blnelson/firepowermockdataserver:latest .

# 2. Start the backend + mock API stack (no pull step)
cd ../backend
podman-compose -f docker-compose.yml -f docker-compose.emulator.yml up --build -d
```

To also run the full end-to-end sequence with the scheduler (which seeds the queue and triggers notifications):

```bash
podman-compose -f docker-compose.yml -f docker-compose.emulator.yml --profile scheduler up --build -d
```

This is equivalent to `make schedule-test` but uses your locally built emulator image instead of pulling the latest from the registry. Services will be available at:
- Backend: http://localhost:8080
- Cloud Tasks emulator: http://localhost:8123
- Mock NHL API: http://localhost:8125
- Mock MoneyPuck API: http://localhost:8124

The reason only the emulator needs a separate build step is that the backend is always built from local source by Compose (via the `build:` directive in `docker-compose.yml`), so it automatically reflects any local changes. The emulator, by contrast, is referenced as a pre-built image (`image: blnelson/firepowermockdataserver:latest`), so Compose just uses whatever is cached locally under that tag. The difference between this approach and `make schedule-test` is that the Makefile target runs `podman pull docker.io/blnelson/firepowermockdataserver:latest` before starting the stack, which would overwrite your locally built image with the latest version from the registry. By invoking `podman-compose` directly and skipping that pull, Podman uses the local image you built from your branch instead.

#### Redis Worker Mode

Uses Redis as the task queue instead of Cloud Tasks. The backend runs as a long-lived worker process that polls Redis for scheduled tasks. Includes the Asynqmon web dashboard for queue visualization.

```bash
# Start Redis worker environment
make redis-up

# Option A: Enqueue today's games via the scheduler (live data)
make redis-schedule

# Option B: Enqueue a single game manually
make build-enqueue
./bin/enqueue --game=2024030411 --duration=12m --notify=true

# Open the Asynqmon dashboard to observe the task
open http://localhost:8980

# Start with mock APIs for testing (includes scheduler)
make redis-schedule-test
```

Configuration: `watchgameupdates/.env.redis` (create from `.env.redis.example`)

The `--mode` flag on the binary controls which backend is used:
- `./bin/watchgameupdates -mode=http` — Cloud Tasks HTTP handler (default)
- `./bin/watchgameupdates -mode=worker` — Redis/Asynq worker

See [docs/queue-visualization.md](docs/queue-visualization.md) for details on using the Asynqmon dashboard.

### Configuration

Environment files are located in `watchgameupdates/`:

**`.env.local`** - Test environment with mock APIs
```env
APP_ENV=local
CLOUD_TASKS_EMULATOR_HOST=cloudtasks-emulator:8123
PLAYBYPLAY_API_BASE_URL=http://mockdataapi-testserver-1:8125
STATS_API_BASE_URL=http://mockdataapi-testserver-1:8124
DISCORD_BOT_TOKEN=your_bot_token_here
DISCORD_CHANNEL_ID=your_channel_id_here
```

**`.env.home`** - Development environment with live APIs
```env
APP_ENV=development
CLOUD_TASKS_EMULATOR_HOST=cloudtasks-emulator:8123
PLAYBYPLAY_API_BASE_URL=https://api-web.nhle.com
STATS_API_BASE_URL=https://moneypuck.com
DISCORD_BOT_TOKEN=your_bot_token_here
DISCORD_CHANNEL_ID=your_channel_id_here
MESSAGE_INTERVAL_SECONDS=60          # How often to poll during active play (default: 60)
PERIOD_END_INTERVAL_SECONDS=1200     # How long to wait after a period ends (default: 1200 = 20min)
```

**`.env.redis`** - Redis worker mode environment
```env
APP_ENV=local
REDIS_ADDRESS=redis:6379
REDIS_PASSWORD=
REDIS_DB=0
MESSAGE_INTERVAL_SECONDS=60
PLAYBYPLAY_API_BASE_URL=
STATS_API_BASE_URL=
DISCORD_BOT_TOKEN=your_bot_token_here
```

**`.env.example`** - Template for custom configurations (includes both Cloud Tasks and Redis vars, and APNs vars for Live Activity)

Update `DISCORD_BOT_TOKEN` and `DISCORD_CHANNEL_ID` in your environment files. To enable iOS Live Activity push, include `liveactivity` in `NOTIFIERS` and populate the `APNS_*` vars — see `.env.example` for the full list.

### Notifiers

Active notifiers are controlled by the `NOTIFIERS` environment variable — set per workload in the Kubernetes manifests under `k8s/base/app/` (with per-environment overrides in `k8s/overlays/<env>/app/`), or in your local `.env.*` file. It accepts a comma-separated list of notifier names:

| Name | Description | Required secrets |
|---|---|---|
| `liveactivity` | iOS Live Activity APNs broadcast push | `APNS_TEAM_ID`, `APNS_KEY_ID`, `APNS_AUTH_KEY`, `APNS_TOPIC` |
| `discord` | Discord channel messages | `DISCORD_BOT_TOKEN`, `DISCORD_CHANNEL_ID` |

```env
# LiveActivity only (cluster default)
NOTIFIERS=liveactivity

# Both notifiers active
NOTIFIERS=liveactivity,discord

# Notifications disabled
NOTIFIERS=
```

Secrets for both notifiers live in the `app-secrets` Kubernetes Secret and are always mounted in the pod, regardless of which notifiers are currently enabled. This means you can add or remove a notifier by only changing the configmap value — no secret changes and no image rebuild.

#### Activating or deactivating a notifier without a rebuild

This applies when the notifier is already implemented in the image running in the cluster. `NOTIFIERS` is an env var on the handler/scheduler workloads, so edit it on the deployment and roll the pod to pick up the change:

```bash
# Option A — edit the deployment in-cluster directly
kubectl -n firepower edit deployment handler
# change NOTIFIERS: "liveactivity"
# to     NOTIFIERS: "liveactivity,discord"

# Option B — change it in the repo and re-deploy
# prod default lives in k8s/base/app/deployment-handler.yaml;
# per-environment overrides live in k8s/overlays/<env>/app/kustomization.yaml
kubectl apply -f <(kubectl kustomize k8s/overlays/production/app | IMAGE_TAG=<tag> envsubst '${IMAGE_TAG}')

# Then restart the pod either way
kubectl -n firepower rollout restart deployment/handler
kubectl -n firepower rollout status deployment/handler

# Verify the right notifiers registered at startup
kubectl -n firepower logs -l app=backend --tail=30
```

> Adding a brand-new notifier type (one not yet implemented in the codebase) requires a code change, a new image build, and a full deployment before it can be enabled via the configmap.

**Tuning reschedule intervals:** `MESSAGE_INTERVAL_SECONDS` controls how frequently the
handler re-checks a live game (default 60s). When a period ends, the service uses
`PERIOD_END_INTERVAL_SECONDS` instead (default 1200s / 20 minutes) to avoid unnecessary
polling during intermissions.

## Testing

### Integration Testing with the Scheduler

The scheduler commands have equivalents for both queue backends:

| Cloud Tasks | Redis | Description |
|---|---|---|
| `make schedule` | `make redis-schedule` | Run scheduler with live NHL data |
| `make schedule-test` | `make redis-schedule-test` | Run scheduler with mock data |
| `make schedule-team TEAM=TOR` | `make redis-schedule-team TEAM=TOR` | Run scheduler for one team |

**Cloud Tasks (default):**

```bash
make schedule-test
```

This pulls the mock data emulator, starts the backend + Cloud Tasks emulator + mock APIs, and runs the scheduler to seed tasks. To run against live NHL data, use `make schedule`. To target a single team against an already-running emulator:

```bash
make schedule-team TEAM=TOR
```

**Redis:**

```bash
make redis-schedule-test
```

This starts the Redis worker environment (Redis + Asynqmon + worker backend + mock APIs) and runs the scheduler to seed tasks. To run against live NHL data, use `make redis-schedule`. To target a single team:

```bash
make redis-schedule-team TEAM=TOR
```

Tasks enqueued by the Redis scheduler are visible in the Asynqmon dashboard at http://localhost:8980.

### Manual Testing

Start the emulator environment and drive it manually:

```bash
make emulator

# In another terminal, follow logs
make logs

# Stop when done
make stop
```

### Unit Tests

```bash
# Run all unit tests
go test ./...

# Run with coverage
go test -cover ./...

# Test specific package
go test ./watchgameupdates/internal/services/...
```

### Test Commands

**Specific test function:**
```bash
cd watchgameupdates && go test -v -run TestDiscordNotifier_FormatMessage ./internal/notification
```

**Specific sub-test (individual test case):**
```bash
cd watchgameupdates && go test -v -run TestDiscordNotifier_FormatMessage/TiedGoals_HomeWinsShootout ./internal/notification
```

**All tests in a package:**
```bash
cd watchgameupdates && go test -v ./internal/notification
```

**All tests in the entire project:**
```bash
cd watchgameupdates && go test -v ./...
```

**Tests with coverage:**
```bash
cd watchgameupdates && go test -v -cover ./internal/notification
```

**Generate coverage report:**
```bash
cd watchgameupdates && go test -v -coverprofile=coverage.out ./internal/notification
cd watchgameupdates && go tool cover -html=coverage.out
```

## Architecture

### Run Modes

The backend binary supports two modes via `--mode`:

| | HTTP Mode (`-mode=http`) | Worker Mode (`-mode=worker`) |
|---|---|---|
| **Task broker** | Google Cloud Tasks (or emulator) | Redis (via Asynq) |
| **Trigger model** | Push — Cloud Tasks POSTs to backend | Pull — worker polls Redis |
| **Compose file** | `docker-compose.yml` | `docker-compose.redis.yml` |
| **Monitoring** | Cloud Tasks emulator (port 8123) | Asynqmon dashboard (port 8980) |
| **Enqueue tool** | `localCloudTasksTest/` | `cmd/enqueue/` |

Both modes share the same core game processing logic in `internal/services/gameprocessor.go`.

### Services (HTTP Mode)

Uses `docker-compose.yml`:

- **Cloud Tasks Emulator** — Port 8123
- **Backend** (HTTP server) — Port 8080
- **Mock Data API** (test only) — Ports 8124, 8125

### Services (Worker Mode)

Uses `docker-compose.redis.yml`:

- **Redis** — Port 6379
- **Asynqmon** (web dashboard) — Port 8980
- **Backend** (Asynq worker) — no exposed port
- **Mock Data API** (test profile) — Ports 8124, 8125

### Key Components

- **GameProcessor** - Shared game-check logic (fetch play-by-play, fetch stats, send notifications)
- **WatchGameUpdatesHandler** (HTTP) - HTTP handler for Cloud Tasks mode
- **WatchGameUpdatesHandler** (Asynq) - Task handler for Redis worker mode
- **HTTPGameDataFetcher** - Fetches game data from NHL/MoneyPuck APIs
- **Rescheduler** - Determines if a game check should be rescheduled

### Data Flow

```
HTTP Mode:  Cloud Tasks → HTTP Handler → GameProcessor → Reschedule via Cloud Tasks
Worker Mode:     Redis  → Asynq Handler → GameProcessor → Reschedule via Redis enqueue
                                  ↓
                          Discord Notifications
                          LiveActivity APNs Push (iOS)
```

## Advanced Usage

### Direct Compose Usage

The Makefile targets are thin wrappers over `podman-compose`. To run them directly:

```bash
# Live APIs (equivalent to `make live`)
docker compose -f docker-compose.yml -f docker-compose.live.yml up --build -d

# Mock APIs (equivalent to `make emulator`)
podman-compose -f docker-compose.yml -f docker-compose.emulator.yml up --build -d

# With scheduler (equivalent to `make schedule` / `make schedule-test`)
docker compose -f docker-compose.yml -f docker-compose.live.yml --profile scheduler up --build -d

# View logs / stop
podman-compose -f docker-compose.yml logs --follow backend
podman-compose -f docker-compose.yml -f docker-compose.live.yml -f docker-compose.emulator.yml down
```

### Building Binaries

Build Go binaries without Podman:

```bash
go run build.go -target watchgameupdates
go run build.go -target enqueue
go run build.go -target localCloudTasksTest

# Run in specific mode
./bin/watchgameupdates -mode=http    # Cloud Tasks HTTP server
./bin/watchgameupdates -mode=worker  # Redis/Asynq worker

# Binaries are output to ./bin/
```

## Troubleshooting

### Common Issues

**Port already in use**

Stop the running stack and check what's holding the port:

```bash
make stop
lsof -i :8080
lsof -i :8123
```

**Services won't start**

```bash
# Check Podman is running (macOS: ensure podman machine is started)
podman info

# macOS: if podman is not reachable, initialize and start the VM
podman machine init && podman machine start

# View service logs
make logs

# Inspect container state
podman-compose ps
```

**Image pull failures**

```bash
podman pull ghcr.io/aertje/cloud-tasks-emulator:latest
podman pull docker.io/blnelson/firepowermockdataserver:latest
```

**Test endpoints manually**

```bash
curl http://localhost:8080
curl http://localhost:8123
```

### Clean Restart

If you encounter persistent issues:

```bash
make stop
docker compose -f docker-compose.yml -f docker-compose.live.yml -f docker-compose.emulator.yml down --remove-orphans
make live   # or `make emulator`
```

## API Reference

### Backend Endpoints

**POST /** - Process game update request
```bash
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"game_id": "2024030411", "execution_end": "2024-06-17T18:00:00Z"}'
```

### External APIs

**NHL Schedule API**
```
GET https://api-web.nhle.com/v1/score/{date}
```

**NHL Game Center API**
```
GET https://api-web.nhle.com/v1/gamecenter/{game_id}/play-by-play
```

**MoneyPuck Expected Goals API**
```
GET https://moneypuck.com/moneypuck/gameData/{season}/{game_id}.csv
```

### Task Payload Format

```json
{
  "game_id": "2024030411",
  "execution_end": "2024-06-17T18:00:00Z"
}
```

## Deployment

### Kubernetes (self-hosted cluster)

The cluster uses two namespaces: `firepower` (production) and `firepower-staging` (staging). Manifests live in `k8s/` as kustomize overlays. Two GitHub Actions workflows manage deployment:

| Workflow | Trigger | What it does |
|---|---|---|
| **Infrastructure** | Manual (`workflow_dispatch`) | Creates namespace, services, and `app-secrets` for the chosen environment |
| **Deploy to Kubernetes** | Auto on `main` push → staging; manual for production | Applies ConfigMap + workloads with the chosen image tag |

**Standing up a new environment:**
```bash
# 1. Run Infrastructure workflow (Actions tab → Infrastructure → Run workflow → staging)
# 2. Run Deploy workflow (Actions tab → Deploy to Kubernetes → Run workflow → staging, image_tag=latest)
```

**Promoting a staging-verified image to production:**
```bash
# Run Deploy workflow manually: environment=production, image_tag=main-<sha>
# where <sha> is the 7-char commit SHA you verified in staging
```

The staging overlay silences all notifiers (`NOTIFIERS=""` on both handler and scheduler) so staging runs never post to real Discord channels or fire APNs pushes.

### Cloud Tasks (HTTP Mode — local/manual)

```bash
cd watchgameupdates
podman build -t crashthecrease-backend .
podman run -p 8080:8080 crashthecrease-backend
```

### Redis Worker Mode

```bash
cd watchgameupdates
docker build -f Dockerfile.worker -t crashthecrease-worker .
docker run --env-file .env.redis crashthecrease-worker
```

Requires a Redis instance. Works with any Redis provider:
- **Local** — Docker Redis
- **AWS** — ElastiCache
- **Managed** — Redis Cloud, Upstash, etc.

Set `REDIS_ADDRESS`, `REDIS_PASSWORD`, and `REDIS_DB` in your environment.

## Development Workflow

### Making Changes

1. Start development environment:
   ```bash
   make live   # or `make emulator` for offline mock data
   ```

2. Make code changes in your editor

3. Rebuild and restart:
   ```bash
   make stop
   make live
   ```

   The compose `build:` directive rebuilds the backend image from local source on each `up --build`.

4. View logs to verify changes:
   ```bash
   make logs
   ```

5. Test your changes:
   ```bash
   make schedule-test
   ```

### Adding New Features

1. Create new handlers in `internal/handlers/`
2. Add business logic in `internal/services/`
3. Define models in `internal/models/`
4. Update configuration in `config/`
5. Write tests alongside your code
6. Update environment variables if needed

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run unit tests: `cd watchgameupdates && go test ./...`
5. Verify the stack still boots: `make emulator`
6. Submit a pull request

### Code Style

- Follow Go standard formatting (`gofmt`)
- Write tests for new functionality
- Update documentation for user-facing changes
- Keep commits focused and atomic

## License

[Your License Here]

## Support

For issues and questions:
- Check the Troubleshooting section above
- Review container logs with `make logs`
- Open an issue on GitHub
