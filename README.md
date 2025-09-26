# CrashTheCrease Backend

A Go-based backend service for tracking and managing NHL game updates using Google Cloud Tasks.

## Quick Start

To set up and run the project locally in one command:

```bash
# Make the script executable and run complete setup
chmod +x setup-local.sh && ./setup-local.sh
```

This script will automatically:
1. Check Go installation and version (requires Go 1.23.3+)
2. Install dependencies for all Go modules
3. Build all services and binaries
4. Set up data directory
5. Set up Docker environment (Cloud Tasks emulator + main service)
6. Create Cloud Tasks queue
7. Test the setup
8. **Keep running and monitor containers**

### Development Workflow

The script acts like a development server:
- **Keeps running**: Monitors containers and shows their status
- **Graceful shutdown**: Press `Ctrl+C` to stop containers (but preserve them)
- **Fast restarts**: Stopped containers are reused on next script run for faster startup
- **Clean rebuilds**: Containers are only removed and rebuilt when script starts, ensuring fresh builds

### Setup Script Options

```bash
# Skip Docker setup (build binaries only)
./setup-local.sh --skip-docker

# Don't cleanup containers on failure
./setup-local.sh --no-cleanup

# Show help
./setup-local.sh --help
```

## Project Structure

- **`watchgameupdates/`** - Main service for processing game updates and managing task queues
  - Uses Google Cloud Functions framework
  - Integrates with Google Cloud Tasks
  - Fetches NHL game data from official APIs
- **`localCloudTasksTest/`** - Test client for local development and testing
  - Creates Cloud Tasks queues and tasks
  - Tests the complete workflow
- **`scheduleGameTrackers/`** - Schedule tracking components
- **`data/`** - Directory for storing NHL game data and responses
- **`GetEventDataByDate.sh`** - Helper script to fetch NHL game data
- **`setup-local.sh`** - One-command setup script for local development

## Architecture

The project follows a clean architecture pattern:

- **`cmd/`** - Application entry points
- **`internal/handlers/`** - HTTP handlers and routing
- **`internal/services/`** - Business logic services
  - **Fetcher Service** - Retrieves game data from external APIs
  - **PlayByPlay Service** - Processes play-by-play data
  - **Rescheduler Service** - Manages task rescheduling
- **`internal/tasks/`** - Task queue management and Cloud Tasks integration
- **`internal/models/`** - Data models and structures
- **`config/`** - Configuration management

## Manual Setup (Alternative)

If you prefer to set up components manually:

### Prerequisites

- Go 1.23.3 or later
- Docker (for Cloud Tasks emulator and containerized services)
- curl (for testing)

### Steps

1. **Install Go dependencies:**
   ```bash
   cd watchgameupdates && go mod download && go mod tidy && cd ..
   cd localCloudTasksTest && go mod download && go mod tidy && cd ..
   ```

2. **Build services:**
   ```bash
   # Use the build system instead
   go run build.go -target watchgameupdates
   go run build.go -target localCloudTasksTest
   go run build.go -target schedulegametrackers

   # Or build all at once
   go run build.go -all
   ```

3. **Set up Docker environment:**
   ```bash
   # Create Docker network
   docker network create net

   # Start Cloud Tasks emulator
   docker run -d --name cloudtasks-emulator --network net -p 8123:8123 \
     ghcr.io/aertje/cloud-tasks-emulator:latest --host=0.0.0.0

   # Build and run main service
   cd watchgameupdates
   docker build -t sendgameupdates .
   docker run -d -p 8080:8080 --name sendgameupdates --network net \
     --env-file .env sendgameupdates
   cd ..
   ```

4. **Create Cloud Tasks queue:**
   ```bash
   cd localCloudTasksTest
   ./localCloudTasksTest
   cd ..
   ```

## Configuration

The main service configuration is managed through environment variables in `watchgameupdates/.env`:

```env
APP_ENV=local
CLOUD_TASKS_EMULATOR_HOST=host.docker.internal:8123
GCP_PROJECT_ID=localproject
GCP_LOCATION=us-south1
CLOUD_TASKS_QUEUE=gameschedule
HANDLER_HOST=http://host.docker.internal:8080
```

## Running the Services

After setup, you can run the services in different ways:

### Dockerized Setup (Recommended)
If you used the full setup script, services are already running in Docker containers:

- **Cloud Tasks Emulator**: http://localhost:8123
- **Main Service**: http://localhost:8080

### Standalone Binaries
```bash
# Main service (alternative to Docker)
cd watchgameupdates
./watchgameupdates

# Test client
cd localCloudTasksTest
./localCloudTasksTest
```

### Helper Scripts
```bash
# Fetch NHL game data for testing
./GetEventDataByDate.sh

# Reference Docker setup (see existing commands)
./watchgameupdates/scripts/local_cloud_task_test.sh
```

## Testing

### Local Test Mode

The project now includes a comprehensive test mode that simulates NHL and MoneyPuck APIs locally, allowing for complete end-to-end testing without external dependencies.

#### Automated Docker Test (Recommended)

For a complete containerized test that mirrors your original workflow:

```bash
./scripts/run_automated_test.sh
```

**What this script does:**
1. **Cleanup**: Removes any existing containers that might conflict
2. **Build & Run**: Builds and starts all required containers:
   - Backend watchgameupdates service (port 8080)
   - Testserver with docker-compose (provides mock game data)
   - Cloud tasks emulator (ghcr.io/aertje/cloud-tasks-emulator:latest)
3. **Test Initiation**: Runs the local cloud tasks test program to trigger the test sequence
4. **Monitoring**: Watches backend logs for the completion signal: `"Last play type: game-end, Should reschedule: false"`
5. **Cleanup**: Stops all containers once testing is complete (containers are preserved for log inspection)

**Features:**
- Runs silently without displaying container logs during execution
- Provides colored status updates throughout the process
- Includes timeout protection (5-minute maximum)
- Preserves containers for post-test log inspection
- Handles errors gracefully with proper cleanup

**After automated testing:**
- Containers are stopped but not deleted for inspection
- Check backend logs: `docker logs watchgameupdates`
- Check testserver logs: `docker-compose -f testserver/docker-compose.yml logs`
- Clean up: `docker rm watchgameupdates cloudtasks-emulator && docker-compose -f testserver/docker-compose.yml rm -f`

#### Quick Test Mode Setup (Alternative)

The following script is an alternative single command test execution:

```bash
./scripts/run_full_test.sh
```

Actual performance is not fully validated.

This script will:
1. Enable test mode in environment variables
2. Build and start the backend with test servers
3. Run the end-to-end test suite
4. Cycle through 10 predefined game events
5. Verify statistics fetching and game completion

#### Test Mode Features

**Simulated APIs:**
- **NHL Play-by-Play API** (localhost:8125) - Returns predefined game events
- **MoneyPuck Statistics API** (localhost:8124) - Returns fictitious statistics

**Predefined Game Events:**
1. `faceoff` - Game start event
2. `shot-on-goal` - Triggers statistics fetch
3. `blocked-shot` - Triggers statistics fetch
4. `missed-shot` - Triggers statistics fetch
5. `goal` - Triggers statistics fetch
6. `hit` - Standard game event
7. `takeaway` - Standard game event
8. `giveaway` - Standard game event
9. `penalty` - Standard game event
10. `game-end` - Completes the test cycle

**Test Statistics:**
- Home Team Expected Goals: Varies by game ID (default: 2.50)
- Away Team Expected Goals: Varies by game ID (default: 2.50)
- Statistics are fetched only for events that trigger recomputation

#### Manual Test Mode

To run the original manual workflow (now automated by the script above):

```bash
# Build and run backend using Docker
docker build -t watchgameupdates watchgameupdates/
docker run -p 8080:8080 --name watchgameupdates --network net --env-file watchgameupdates/.env watchgameupdates

# Build and run test server
cd testserver/ && docker-compose up --build -d

# Build and run cloud task emulator
docker run -it --name cloudtasks-emulator -p 8123:8123 ghcr.io/aertje/cloud-tasks-emulator:latest --host=0.0.0.0

# Initiate monitoring a fake game
./localCloudTasksTest/localCloudTasksTest
```

### Testing with NHL Data

The project includes scripts to work with real NHL game data:

```bash
# Get yesterday's game ID and fetch data
./GetEventDataByDate.sh

# Test the handler with real game data
# (See GetEventDataByDate.sh for example curl commands)
```

### Unit Tests

```bash
# Run tests for all modules
go test ./watchgameupdates/...
go test ./localCloudTasksTest/...

# Run with coverage
go test -cover ./...
```

## Development

### Key Services

- **WatchGameUpdatesHandler** - Main HTTP handler that processes game update requests
- **HTTPGameDataFetcher** - Fetches game data from NHL APIs
- **PlayByPlay Service** - Processes play-by-play events
- **Task Factory** - Creates and manages Cloud Tasks
- **Rescheduler** - Handles task rescheduling based on game state

### Local Development Workflow

1. Use `./setup-local.sh` to set up the complete environment
2. Make changes to Go code
3. Rebuild specific services:
   ```bash
   cd watchgameupdates && go build -o watchgameupdates ./cmd/watchgameupdates/
   ```
4. Restart Docker containers if needed:
   ```bash
   docker restart sendgameupdates
   ```
5. Test changes using the test client or helper scripts

### Environment Variables

The system supports different environments through the `APP_ENV` variable:
- `local` - Local development with emulators
- `dev` - Development environment
- `prod` - Production environment

## Deployment

The project includes a Dockerfile for containerized deployment:

```bash
cd watchgameupdates
docker build -t crashthecrease-backend .
docker run -p 8080:8080 crashthecrease-backend
```

For Google Cloud deployment, the service is designed to work with:
- Google Cloud Functions
- Google Cloud Tasks
- Google Cloud Run

## Stopping Services

To stop the local development environment:

```bash
# Stop containers
docker stop cloudtasks-emulator sendgameupdates

# Remove containers (optional)
docker rm cloudtasks-emulator sendgameupdates

# Remove network (optional)
docker network rm net
```

## Troubleshooting

### Common Issues

1. **Go version errors**: Ensure you have Go 1.23.3 or later installed
2. **Docker not running**: Start Docker Desktop or Docker daemon
3. **Port conflicts**: Ensure ports 8080, 8123, 8124, and 8125 are available
4. **Container startup failures**: Check Docker logs: `docker logs watchgameupdates`

### Automated Test Script Issues

1. **Script fails with permission denied**:
   ```bash
   chmod +x scripts/run_automated_test.sh
   ```

2. **Test times out after 5 minutes**:
   - Check if all containers started properly: `docker ps`
   - Inspect backend logs: `docker logs watchgameupdates`
   - Verify testserver is responding: `curl http://localhost:8125/v1/gamecenter/test/play-by-play`

3. **Port conflicts during automated testing**:
   - Ensure no other services are running on ports 8080, 8123, 8124, 8125
   - The script attempts to clean up existing containers automatically

4. **Network issues**:
   - The script creates a Docker network named 'net' if it doesn't exist
   - If you encounter network conflicts, manually clean up: `docker network rm net`

5. **Container inspection after testing**:
   ```bash
   # View all containers (running and stopped)
   docker ps -a

   # Check backend logs
   docker logs watchgameupdates

   # Check testserver logs
   docker-compose -f testserver/docker-compose.yml logs

   # Access container shell for debugging
   docker exec -it watchgameupdates /bin/sh
   ```

### Debugging

1. **Check service logs:**
   ```bash
   docker logs cloudtasks-emulator
   docker logs sendgameupdates
   ```

2. **Verify network connectivity:**
   ```bash
   docker network inspect net
   ```

3. **Test individual components:**
   ```bash
   # Test just the Go binary
   cd watchgameupdates && ./watchgameupdates
   ```

### Getting Help

- Check the logs for detailed error messages
- Verify that all environment variables are properly set
- Ensure Docker is running and accessible
- Make sure the Cloud Tasks emulator is responding
- Verify Go dependencies are correctly installed

## API Reference

### Main Service Endpoints

- **POST /** - Process game update requests
  ```bash
  curl -X POST http://localhost:8080 \
    -H "Content-Type: application/json" \
    -d '{"game_id": "2024030411", "execution_end": "2024-06-17T18:00:00Z"}'
  ```

### NHL API Integration

The service integrates with the NHL API:
- **Schedule API**: `https://api-web.nhle.com/v1/score/{date}`
- **Game Center API**: `https://api-web.nhle.com/v1/gamecenter/{game_id}/play-by-play`

### Task Payload Format

```json
{
  "game_id": "2024030411",
  "execution_end": "2024-06-17T18:00:00Z"
}
```

## Contributing

1. Ensure Go 1.23.3+ is installed
2. Run `./setup-local.sh` to set up the development environment
3. Make your changes
4. Run tests: `go test ./...`
5. Test with the complete setup
6. Submit a pull request
