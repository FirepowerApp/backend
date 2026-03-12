# Firepower Backend

A Go-based backend service for tracking and managing NHL game updates using Google Cloud Tasks.

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

**`.env.example`** - Template for custom configurations

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

### Services

The project uses Docker Compose to orchestrate three main services:

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
go run build.go -target localCloudTasksTest

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

### Containerized Deployment

```bash
cd watchgameupdates
docker build -t crashthecrease-backend .
docker run -p 8080:8080 crashthecrease-backend
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
