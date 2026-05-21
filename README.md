# Firepower Backend

A Go-based backend service for tracking and managing NHL game updates using Google Cloud Tasks.

## Prerequisites

- **Go 1.23.3+** - [Install Go](https://go.dev/doc/install)
- **Podman 5.0+** - [Install Podman](https://podman.io/docs/installation) (macOS: `brew install podman podman-compose`)
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

## Project Structure

```
backend/
├── watchgameupdates/        # Main service
│   ├── cmd/                 # Application entry point
│   ├── internal/            # Internal packages
│   │   ├── handlers/        # HTTP handlers
│   │   ├── services/        # Business logic
│   │   ├── tasks/           # Cloud Tasks integration
│   │   └── models/          # Data models
│   ├── config/              # Configuration management
│   └── Dockerfile           # Container definition
├── localCloudTasksTest/     # Test client for Cloud Tasks
├── scripts/                 # Utility scripts
├── docker-compose.yml       # Service orchestration
├── Makefile                 # Development commands
└── README.md                # This file
```

## Development

### Available Commands

Run `make help` to see all available commands:

```bash
# Setup & Prerequisites
make check-deps          # Check Go and Podman installation
make setup               # Initial setup (one-time)
make pull                # Pull latest container images

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

# Building
make build               # Build all Go binaries
make build-backend       # Build backend binary only

# Troubleshooting
make doctor              # Run diagnostic checks
make port-check          # Check port availability
```

### Environment Modes

The system supports two environment modes:

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

The reason only the emulator needs a separate build step is that the backend is always built from local source by Compose (via the `build:` directive in `docker-compose.yml`), so it automatically reflects any local changes. The emulator, by contrast, is referenced as a pre-built image, so Compose just uses whatever is cached locally under that tag. The difference between this approach and `make schedule-test` is that the Makefile target runs `podman pull` before starting the stack, which would overwrite your locally built image with the latest version from the registry. By invoking `podman-compose` directly and skipping that pull, Podman uses the local image you built from your branch instead.

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
MESSAGE_INTERVAL_SECONDS=60          # How often to poll during active play (default: 60)
PERIOD_END_INTERVAL_SECONDS=1200     # How long to wait after a period ends (default: 1200 = 20min)
```

**`.env.example`** - Template for custom configurations

Update the `DISCORD_BOT_TOKEN` in your environment files as needed.

**Tuning reschedule intervals:** `MESSAGE_INTERVAL_SECONDS` controls how frequently the
handler re-checks a live game (default 60s). When a period ends, the service uses
`PERIOD_END_INTERVAL_SECONDS` instead (default 1200s / 20 minutes) to avoid unnecessary
polling during intermissions.

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

### Services

The project uses Compose to orchestrate three main services:

**Cloud Tasks Emulator**
- Emulates Google Cloud Tasks for local development
- Persists across test runs
- Port: 8123

**Backend (watchgameupdates)**
- Main application service
- Processes game updates and manages task queues
- Port: 8080

**Mock Data API** (optional, test mode only)
- Provides mock NHL and MoneyPuck API responses
- Ports: 8124 (MoneyPuck), 8125 (NHL)

### Key Components

- **WatchGameUpdatesHandler** - Main HTTP handler for game update requests
- **HTTPGameDataFetcher** - Fetches game data from NHL APIs
- **PlayByPlay Service** - Processes play-by-play events
- **Task Factory** - Creates and manages Cloud Tasks
- **Rescheduler** - Handles task rescheduling based on game state
- **Fetcher Service** - Retrieves expected goals data from MoneyPuck

### Data Flow

```
Cloud Tasks → Backend Handler → Fetch Game Data → Process Events → Reschedule/Complete
                    ↓
              Discord Notifications
```

## Advanced Usage

### Direct Compose Usage

If you prefer using Podman Compose directly:

```bash
# Start home environment (live APIs)
podman-compose -f docker-compose.yml -f docker-compose.live.yml up --build -d

# Start test environment (mock APIs)
podman-compose -f docker-compose.yml -f docker-compose.emulator.yml up --build -d

# View logs
podman-compose -f docker-compose.yml logs --follow backend

# Stop all services
podman-compose down
```

### Building Binaries

Build Go binaries without Podman:

```bash
# Build all binaries
make build

# Build specific target
go run build.go -target watchgameupdates
go run build.go -target localCloudTasksTest

# Binaries are output to ./bin/
```

### Manual Container Setup

For manual setup without Compose:

```bash
# Create network
podman network create net

# Start Cloud Tasks emulator
podman pull ghcr.io/aertje/cloud-tasks-emulator:latest
podman run -d --name cloudtasks-emulator --network net -p 8123:8123 \
  ghcr.io/aertje/cloud-tasks-emulator:latest --host=0.0.0.0

# Build and run backend
cd watchgameupdates
podman build -t watchgameupdates .
podman run -d -p 8080:8080 --name watchgameupdates --network net \
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

# Check Podman is running (macOS: ensure podman machine is started)
podman info

# View service logs
make logs
```

**Image pull failures**

The Makefile includes automatic retry logic with exponential backoff (2s, 4s, 8s, 16s delays). If pulls continue to fail:

```bash
# Manually pull images
podman pull ghcr.io/aertje/cloud-tasks-emulator:latest

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

> **macOS note:** Podman runs containers inside a Linux VM (`podman machine`). On first use, initialize and start it:
> ```bash
> podman machine init
> podman machine start
> ```

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
podman-compose ps

# Inspect container details
podman inspect watchgameupdates
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

### Containerized Deployment

```bash
cd watchgameupdates
podman build -t crashthecrease-backend .
podman run -p 8080:8080 crashthecrease-backend
```

### Google Cloud Platform

The service is designed to deploy on:
- **Google Cloud Run** - Containerized serverless deployment
- **Google Cloud Functions** - Function-as-a-Service deployment
- **Google Cloud Tasks** - Managed task queue service

Update environment variables for production:
- Remove `CLOUD_TASKS_EMULATOR_HOST` (use real Cloud Tasks)
- Set proper `GCP_PROJECT_ID` and `GCP_LOCATION`
- Configure production API endpoints
- Set production Discord webhook URLs

## Development Workflow

### Making Changes

1. Start development environment:
   ```bash
   make live
   ```

2. Make code changes in your editor

3. Rebuild and restart:
   ```bash
   # Rebuild and restart all services
   podman-compose -f docker-compose.yml -f docker-compose.live.yml up --build -d

   # Or use make
   make stop
   make live
   ```

4. View logs to verify changes:
   ```bash
   make logs
   ```

5. Test your changes:
   ```bash
   cd watchgameupdates && go test ./...
   ```

### Adding New Features

1. Create new handlers in `internal/handlers/`
2. Add business logic in `internal/services/`
3. Define models in `internal/models/`
4. Update configuration in `config/`
5. Write tests alongside your code
6. Update environment variables if needed

## Legacy Scripts

The following shell scripts are still available for reference but Podman Compose + Makefile is the recommended approach:

- `setup-local.sh` - Legacy setup script (use `make live` or `make emulator` instead)

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
