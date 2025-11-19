# CrashTheCrease Backend - Docker Compose Setup

This document explains the new Docker Compose + Makefile setup that replaces the functionality of `setup-local.sh` and `run_automated_test.sh`.

## 📋 Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Usage](#usage)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Migration from Shell Scripts](#migration-from-shell-scripts)

## 🎯 Overview

### What This Replaces

The new Docker Compose setup consolidates ~1000 lines of shell script into ~150 lines of declarative configuration:

- ✅ Replaces `setup-local.sh` (546 lines)
- ✅ Replaces `run_automated_test.sh` (518 lines)
- ✅ Adds retry logic for network errors when pulling images
- ✅ Provides standardized commands via Makefile
- ✅ Supports multiple environments (test/dev)
- ✅ Includes comprehensive health checks
- ✅ Maintains container persistence for faster restarts

### Benefits

| Feature | Shell Scripts | Docker Compose |
|---------|--------------|----------------|
| Lines of Code | ~1000 | ~150 |
| Health Checks | Manual polling | Built-in |
| Error Handling | Custom bash | Automatic |
| Port Management | Manual detection | Automatic |
| Dependency Order | Manual orchestration | Declarative |
| Log Aggregation | Separate commands | `docker compose logs` |
| Cleanup | Custom functions | `docker compose down` |

## 🚀 Quick Start

### Prerequisites

- **Go 1.23.3+** - [Install Go](https://go.dev/doc/install)
- **Docker** - [Install Docker](https://docs.docker.com/get-docker/)
- **Make** - Usually pre-installed on Linux/macOS

### Initial Setup

```bash
# Check prerequisites and pull images
make setup

# Start development environment (live APIs)
make dev

# OR start test environment (mock APIs)
make test-containers
```

### Run Automated Tests

```bash
# Run full integration test suite
make test
```

### View Logs

```bash
# View all logs
make logs

# View backend logs only
make logs-backend

# View cloud tasks emulator logs
make logs-cloudtasks
```

### Stop Everything

```bash
# Stop containers (preserves for quick restart)
make stop

# Stop and remove everything
make clean
```

## 🏗️ Architecture

### Services

The setup includes three main services:

#### 1. **Cloud Tasks Emulator**
- **Image**: `ghcr.io/aertje/cloud-tasks-emulator:latest`
- **Container**: `cloudtasks-emulator`
- **Port**: `8123`
- **Purpose**: Emulates Google Cloud Tasks for local development
- **Persistence**: Always kept running across test runs (like the original scripts)

#### 2. **Backend (WatchGameUpdates)**
- **Image**: Built from `./watchgameupdates/Dockerfile`
- **Container**: `watchgameupdates`
- **Port**: `8080`
- **Purpose**: Main application service
- **Environment**: Configured via `.env.local` or `.env.home`

#### 3. **Mock Data API (Optional)**
- **Image**: `mockdataapi:latest` (must be built separately)
- **Container**: `mockdataapi-testserver-1`
- **Ports**: `8124` (MoneyPuck), `8125` (NHL API)
- **Purpose**: Provides mock NHL and MoneyPuck API responses
- **Note**: Only used in test mode; removed in commit 606aa80

### Docker Compose Files

```
docker-compose.yml        # Base configuration (all services)
docker-compose.test.yml   # Override for test mode (with mocks)
docker-compose.dev.yml    # Override for dev mode (live APIs)
```

### Network Architecture

```
┌─────────────────────────────────────────┐
│          Docker Network: net            │
│                                         │
│  ┌─────────────────────────────────┐   │
│  │   cloudtasks-emulator:8123      │   │
│  └──────────────┬──────────────────┘   │
│                 │                       │
│  ┌──────────────▼──────────────────┐   │
│  │   watchgameupdates:8080         │   │
│  │   (Backend Service)             │   │
│  └──────────────┬──────────────────┘   │
│                 │                       │
│  ┌──────────────▼──────────────────┐   │
│  │ mockdataapi-testserver-1        │   │
│  │ (8124: MoneyPuck, 8125: NHL)    │   │
│  │ [Test mode only]                │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
         │
         ▼
    Host Ports: 8080, 8123, 8124, 8125
```

## 📖 Usage

### Makefile Commands

Run `make help` to see all available commands:

```bash
make help               # Show all available commands
```

#### Setup & Prerequisites

```bash
make check-go          # Check Go installation
make check-docker      # Check Docker installation
make check-deps        # Check all prerequisites
make setup             # Full initial setup
```

#### Image Management

```bash
make pull              # Pull required images with retry logic
make pull-with-retry   # Pull images with exponential backoff
```

#### Development & Testing

```bash
make dev               # Start dev environment (live APIs)
make test-containers   # Start test containers and keep running
make test              # Run full automated test suite
```

#### Container Management

```bash
make status            # Show container status
make logs              # View all logs
make logs-backend      # View backend logs only
make logs-cloudtasks   # View cloud tasks logs only
make stop              # Stop all containers
make restart           # Restart all containers
make clean             # Stop and remove containers
make clean-all         # Remove containers + images
```

#### Building

```bash
make build             # Build all Go binaries
make build-backend     # Build backend binary only
make build-testserver  # Build testserver binary (if exists)
```

#### Utilities

```bash
make port-check        # Check port availability
make shell-backend     # Open shell in backend container
make shell-cloudtasks  # Open shell in cloud tasks container
make doctor            # Run all diagnostics
```

### Direct Docker Compose Usage

You can also use Docker Compose directly:

```bash
# Start with default profile
docker compose up

# Start test environment
docker compose --profile test up

# Start dev environment
docker compose --profile dev up

# Stop everything
docker compose down

# View logs
docker compose logs -f

# Check status
docker compose ps
```

## ⚙️ Configuration

### Environment Files

Three environment files control the configuration:

#### 1. `.env.local` - Test Mode with Mock APIs

```bash
APP_ENV=local
CLOUD_TASKS_EMULATOR_HOST=cloudtasks-emulator:8123
PLAYBYPLAY_API_BASE_URL=http://mockdataapi-testserver-1:8125
STATS_API_BASE_URL=http://mockdataapi-testserver-1:8124
# ... other settings
```

**Used by**: `make test`, `make test-containers`

#### 2. `.env.home` - Development Mode with Live APIs

```bash
APP_ENV=development
CLOUD_TASKS_EMULATOR_HOST=cloudtasks-emulator:8123
PLAYBYPLAY_API_BASE_URL=https://api-web.nhle.com
STATS_API_BASE_URL=https://moneypuck.com
# ... other settings
```

**Used by**: `make dev`

#### 3. `.env.example` - Template

Copy this to create your own environment files:

```bash
cp watchgameupdates/.env.example watchgameupdates/.env.custom
# Edit .env.custom with your settings
```

### Docker Profiles

Services are organized into profiles:

- **`default`**: Cloud Tasks Emulator + Backend
- **`dev`**: Development mode (live APIs)
- **`test`**: Test mode (includes mock APIs)

## 🔧 Troubleshooting

### Common Issues

#### 1. Port Already in Use

**Error**: `port is already allocated`

**Solution**:
```bash
# Check what's using the ports
make port-check

# Kill the process or stop conflicting containers
make clean
```

#### 2. Mock API Container Not Found

**Error**: `mockdataapi:latest not found`

**Explanation**: The testserver was removed in commit 606aa80

**Solution**:
- Use dev mode instead: `make dev`
- Or rebuild the testserver from git history

#### 3. Image Pull Failures

**Error**: Network timeout when pulling images

**Solution**: The Makefile includes automatic retry logic with exponential backoff

```bash
# Retry logic automatically applies with make pull
make pull

# Manual retry if needed
docker pull ghcr.io/aertje/cloud-tasks-emulator:latest
```

#### 4. Services Not Ready

**Error**: Health check failures

**Solution**: Health checks retry for up to 30 attempts

```bash
# Check service status
make status

# View logs to diagnose
make logs

# Check if ports are responding
curl http://localhost:8080
curl http://localhost:8123
```

#### 5. Cloud Tasks Emulator Won't Start

**Error**: `cloudtasks-emulator` fails to start

**Solution**:
```bash
# Check if port 8123 is available
lsof -i :8123

# Remove old container and restart
docker stop cloudtasks-emulator
docker rm cloudtasks-emulator
make dev
```

### Diagnostic Commands

```bash
# Run all diagnostics
make doctor

# Check prerequisites
make check-deps

# Check port availability
make port-check

# View container status
docker compose ps

# View detailed container info
docker inspect watchgameupdates
docker inspect cloudtasks-emulator
```

### Container Persistence

Like the original scripts, the cloud tasks emulator is preserved across runs:

```bash
# Stops containers but doesn't remove them
make stop

# Completely removes everything
make clean
```

### Logs and Debugging

```bash
# Follow all logs
make logs

# Backend logs only
make logs-backend

# Cloud tasks logs only
make logs-cloudtasks

# Get a shell in backend container
make shell-backend

# Get a shell in cloud tasks container
make shell-cloudtasks
```

## 🔄 Migration from Shell Scripts

### Command Equivalents

| Old Shell Script | New Makefile Command |
|------------------|---------------------|
| `./setup-local.sh` | `make setup && make dev` |
| `./scripts/run_automated_test.sh` | `make test` |
| `./scripts/run_automated_test.sh --containers-only` | `make test-containers` |
| `./scripts/run_automated_test.sh --env-home` | `make dev` |
| `docker logs watchgameupdates` | `make logs-backend` |
| `docker stop ...` | `make stop` |
| `docker rm ...` | `make clean` |

### What's Preserved

✅ **All functionality from the original scripts**:
- Automatic network creation
- Health checks with retries
- Container persistence (cloud tasks emulator)
- Port conflict detection
- Colored output
- Log monitoring
- Graceful cleanup

✅ **Plus new features**:
- Retry logic for image pulls with exponential backoff
- Declarative service dependencies
- Standard Docker Compose commands
- Better error messages
- Integrated help system (`make help`)
- Multiple environment support
- Diagnostic tools (`make doctor`)

### What Changed

❌ **No longer needed**:
- Manual health check polling (built into Docker Compose)
- Custom cleanup functions (use `docker compose down`)
- Port conflict detection scripts (Docker handles this)
- Custom logging functions (use `docker compose logs`)

## 📝 Advanced Usage

### Custom Overrides

Create `docker-compose.override.yml` for local customizations:

```yaml
version: '3.8'

services:
  backend:
    volumes:
      - ./custom-data:/data
    environment:
      - CUSTOM_VAR=value
```

### Running Specific Services

```bash
# Start only cloud tasks emulator
docker compose up cloudtasks-emulator

# Start backend + cloud tasks
docker compose up backend

# Start with rebuild
docker compose up --build
```

### Multiple Compose Files

```bash
# Combine base + test + custom
docker compose -f docker-compose.yml \
               -f docker-compose.test.yml \
               -f docker-compose.custom.yml \
               up
```

### CI/CD Integration

The setup is CI/CD ready:

```yaml
# Example GitHub Actions
- name: Run integration tests
  run: |
    make setup
    make test
```

## 🎓 Learning Resources

- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Docker Compose File Reference](https://docs.docker.com/compose/compose-file/)
- [Make Documentation](https://www.gnu.org/software/make/manual/)
- [Docker Networking](https://docs.docker.com/network/)

## 📞 Support

If you encounter issues:

1. Run diagnostics: `make doctor`
2. Check logs: `make logs`
3. Verify prerequisites: `make check-deps`
4. Review this documentation
5. Check the original scripts for comparison

## 🔗 Related Files

- `docker-compose.yml` - Base service definitions
- `docker-compose.test.yml` - Test mode overrides
- `docker-compose.dev.yml` - Dev mode overrides
- `Makefile` - Command shortcuts and retry logic
- `watchgameupdates/.env.local` - Test environment config
- `watchgameupdates/.env.home` - Dev environment config
- `watchgameupdates/.env.example` - Template
