# CrashTheCrease Backend

A Go-based backend service for tracking and managing NHL game updates. Supports two task queue backends: **Google Cloud Tasks** (HTTP mode) and **Redis via Asynq** (worker mode), selectable at runtime with a `--mode` flag.

## Prerequisites

- **Go 1.23.3+** - [Install Go](https://go.dev/doc/install)
- **Docker** - [Install Docker](https://docs.docker.com/get-docker/)
- **Make** - Usually pre-installed on Linux/macOS

## Quick Start

```bash
# One-time setup: install dependencies and pull images
make setup

# Start home environment (connects to live NHL/MoneyPuck APIs)
make home

# View logs
make logs

# Stop services
make stop
```

Your services will be available at:
- Backend: http://localhost:8080
- Cloud Tasks Emulator: http://localhost:8123

### Quick Start (Redis Worker Mode)

```bash
# Start Redis + Asynqmon + backend worker
make redis-up

# Build the enqueue CLI
make build-enqueue

# Enqueue a game-watching task
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
│   │   ├── notification/        # Discord notification service
│   │   └── models/              # Data models
│   ├── config/                  # Configuration management
│   ├── Dockerfile               # Container definition (HTTP mode)
│   └── Dockerfile.worker        # Container definition (worker mode)
├── localCloudTasksTest/         # Test client for Cloud Tasks
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
# Setup & Prerequisites
make check-deps          # Check Go and Docker installation
make setup               # Initial setup (one-time)
make pull                # Pull latest Docker images

# Development
make home                 # Start home environment (live APIs)
make test-containers     # Start test environment (mock APIs)
make test                # Run full automated test suite

# Container Management
make status              # Show container status
make logs                # View all logs
make logs-backend        # View backend logs only
make stop                # Stop containers
make clean               # Remove containers and cleanup

# Redis Queue (Worker Mode)
make redis-up            # Start Redis + Asynqmon + worker backend
make redis-test          # Start Redis test environment (with mock APIs)
make redis-stop          # Stop Redis containers
make redis-clean         # Remove Redis containers and volumes
make redis-logs          # View Redis worker logs
make redis-status        # Show Redis container status

# Building
make build               # Build all Go binaries
make build-backend       # Build backend binary only
make build-enqueue       # Build the Redis enqueue CLI tool

# Troubleshooting
make doctor              # Run diagnostic checks
make port-check          # Check port availability
```

### Environment Modes

The system supports two task queue backends and multiple environment modes:

#### Home Mode (Live APIs)
Uses real NHL and MoneyPuck APIs for development and testing with live data.

```bash
make home
```

Configuration: `watchgameupdates/.env.home`

#### Test Mode (Mock APIs)
Uses mock APIs for isolated testing without external dependencies.

```bash
make test-containers
```

Configuration: `watchgameupdates/.env.local`

**Using a Locally Built Mock API:**

If you're developing changes to the mock data server, you can use a locally built image instead of pulling from the registry:

```bash
# First, build your local mock API image
cd /path/to/your/mockdataserver
docker build -t mockdataapi:latest .

# Then run test containers with the local image
make test-containers LOCAL_MOCK=true

# Or run full automated tests with the local image
make test LOCAL_MOCK=true
```

The `LOCAL_MOCK=true` flag tells the system to use your locally built `mockdataapi:latest` image instead of pulling `blnelson/firepowermockdataserver:latest` from the registry. This is useful for testing changes to the mock API without needing to push to a registry.

#### Redis Worker Mode

Uses Redis as the task queue instead of Cloud Tasks. The backend runs as a long-lived worker process that polls Redis for scheduled tasks. Includes the Asynqmon web dashboard for queue visualization.

```bash
# Start Redis worker environment
make redis-up

# Build the enqueue CLI and add a task
make build-enqueue
./bin/enqueue --game=2024030411 --duration=12m --notify=true

# Open the Asynqmon dashboard to observe the task
open http://localhost:8980

# Start with mock APIs for testing
make redis-test
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
```

**`.env.home`** - Development environment with live APIs
```env
APP_ENV=development
CLOUD_TASKS_EMULATOR_HOST=cloudtasks-emulator:8123
PLAYBYPLAY_API_BASE_URL=https://api-web.nhle.com
STATS_API_BASE_URL=https://moneypuck.com
DISCORD_BOT_TOKEN=your_bot_token_here
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

**`.env.example`** - Template for custom configurations (includes both Cloud Tasks and Redis vars)

Update the `DISCORD_BOT_TOKEN` in your environment files as needed.

## Testing

### Automated Integration Tests

Run the complete test suite:

```bash
make test
```

This will:
1. Start all required containers (backend, cloud tasks emulator, mock APIs)
2. Run the test sequence
3. Monitor logs for completion
4. Report results and stop test containers

### Manual Testing

Start the test environment and run tests manually:

```bash
# Start containers
make test-containers

# Or use a locally built mock API image
make test-containers LOCAL_MOCK=true

# In another terminal, run test client
cd localCloudTasksTest
./localCloudTasksTest

# View logs
make logs-backend

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
```

## Advanced Usage

### Direct Docker Compose Usage

If you prefer using Docker Compose directly:

```bash
# Start home environment
docker compose --profile home up -d

# Start test environment
docker compose --profile test up -d

# View logs
docker compose logs -f

# Stop all services
docker compose down
```

### Building Binaries

Build Go binaries without Docker:

```bash
# Build all binaries
make build

# Build specific target
go run build.go -target watchgameupdates
go run build.go -target enqueue
go run build.go -target localCloudTasksTest

# Run in specific mode
./bin/watchgameupdates -mode=http    # Cloud Tasks HTTP server
./bin/watchgameupdates -mode=worker  # Redis/Asynq worker

# Binaries are output to ./bin/
```

### Manual Container Setup

For manual Docker setup without Docker Compose:

```bash
# Create network
docker network create net

# Start Cloud Tasks emulator
docker pull ghcr.io/aertje/cloud-tasks-emulator:latest
docker run -d --name cloudtasks-emulator --network net -p 8123:8123 \
  ghcr.io/aertje/cloud-tasks-emulator:latest --host=0.0.0.0

# Build and run backend
cd watchgameupdates
docker build -t watchgameupdates .
docker run -d -p 8080:8080 --name watchgameupdates --network net \
  --env-file .env.home watchgameupdates
cd ..

# Create Cloud Tasks queue
cd localCloudTasksTest
./localCloudTasksTest
cd ..
```

## Troubleshooting

### Common Issues

**Port already in use**
```bash
# Check what's using the port
make port-check

# Stop conflicting containers
make clean
```

**Services won't start**
```bash
# Run diagnostics
make doctor

# Check Docker is running
docker info

# View service logs
make logs
```

**Image pull failures**

The Makefile includes automatic retry logic with exponential backoff (2s, 4s, 8s, 16s delays). If pulls continue to fail:

```bash
# Manually pull images
docker pull ghcr.io/aertje/cloud-tasks-emulator:latest

# Or retry with make
make pull
```

**Container health check failures**

Services have health checks with 30 retries (up to 150 seconds). If services still fail:

```bash
# Check service status
make status

# View detailed logs
make logs

# Test endpoints manually
curl http://localhost:8080
curl http://localhost:8123
```

### Debugging

```bash
# View all container logs
make logs

# View specific service logs
make logs-backend
make logs-cloudtasks

# Get a shell in the backend container
make shell-backend

# Check container status
docker compose ps

# Inspect container details
docker inspect watchgameupdates
```

### Clean Restart

If you encounter persistent issues:

```bash
# Stop and remove everything
make clean-all

# Restart from scratch
make setup
make home
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

### Cloud Tasks (HTTP Mode)

```bash
cd watchgameupdates
docker build -t crashthecrease-backend .
docker run -p 8080:8080 crashthecrease-backend
```

Deploys on Google Cloud Run / Cloud Functions with Google Cloud Tasks.

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
   make home
   ```

2. Make code changes in your editor

3. Rebuild and restart:
   ```bash
   # Rebuild and restart all services
   docker compose --profile home up -d --build

   # Or use make
   make stop
   make home
   ```

4. View logs to verify changes:
   ```bash
   make logs-backend
   ```

5. Test your changes:
   ```bash
   make test
   ```

### Adding New Features

1. Create new handlers in `internal/handlers/`
2. Add business logic in `internal/services/`
3. Define models in `internal/models/`
4. Update configuration in `config/`
5. Write tests alongside your code
6. Update environment variables if needed

## Legacy Scripts

The following shell scripts are still available for reference but Docker Compose + Makefile is the recommended approach:

- `setup-local.sh` - Legacy setup script (use `make setup && make home` instead)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Ensure code builds: `make build`
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
- Run `make doctor` for diagnostics
- Review container logs with `make logs`
- Open an issue on GitHub
