#!/bin/bash

# CrashTheCrease Backend Local Setup Script
# This script sets up the entire project for local development and testing

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check Go version
check_go_version() {
    if ! command_exists go; then
        log_error "Go is not installed. Please install Go 1.23.3 or later."
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    REQUIRED_VERSION="1.23.3"

    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
        log_error "Go version $GO_VERSION is installed, but version $REQUIRED_VERSION or later is required."
        exit 1
    fi

    log_success "Go version $GO_VERSION is installed and compatible"
}

# Function to check Docker installation
check_docker() {
    if ! command_exists docker; then
        log_warning "Docker is not installed. Docker is required for Cloud Tasks emulator."
        log_info "Please install Docker from https://docs.docker.com/get-docker/"
        return 1
    fi

    if ! docker info >/dev/null 2>&1; then
        log_warning "Docker is installed but not running. Please start Docker."
        return 1
    fi

    log_success "Docker is installed and running"
    return 0
}

# Function to install dependencies for a Go module
install_go_dependencies() {
    local module_path=$1
    local module_name=$2

    log_info "Installing dependencies for $module_name..."

    if [ ! -f "$module_path/go.mod" ]; then
        log_error "go.mod not found in $module_path"
        exit 1
    fi

    cd "$module_path"
    go mod download
    go mod tidy
    cd - > /dev/null

    log_success "$module_name dependencies installed"
}

# Function to build a Go service
build_go_service() {
    local module_path=$1
    local module_name=$2
    local binary_name=$3

    log_info "Building $module_name..."

    cd "$module_path"
    if [ -d "cmd" ]; then
        go build -o "$binary_name" ./cmd/*/
    else
        go build -o "$binary_name" .
    fi
    cd - > /dev/null

    log_success "$module_name built successfully"
}

# Function to check and create data directory
setup_data_directory() {
    log_info "Setting up data directory..."

    if [ ! -d "data" ]; then
        mkdir -p data
        log_info "Created data directory"
    else
        log_success "Data directory already exists"
    fi
}

# Function to create Docker network
setup_docker_network() {
    log_info "Setting up Docker network..."

    if ! docker network ls | grep -q "net"; then
        docker network create net
        log_success "Docker network 'net' created"
    else
        log_success "Docker network 'net' already exists"
    fi
}

# Function to pull required Docker images
pull_docker_images() {
    log_info "Pulling required Docker images..."

    docker pull ghcr.io/aertje/cloud-tasks-emulator:latest
    log_success "Cloud Tasks emulator image pulled"
}

# Function to check for port conflicts
check_port_availability() {
    local port=$1
    local service_name=$2

    if lsof -i :$port >/dev/null 2>&1; then
        log_warning "Port $port is already in use by another process"
        log_info "Finding process using port $port..."
        lsof -i :$port

        # Check if it's our container
        if docker ps --format "table {{.Names}}\t{{.Ports}}" | grep -q ":$port->"; then
            log_info "Port $port is used by an existing Docker container, will clean up"
            return 1
        else
            log_error "Port $port is in use by a non-Docker process. Please stop the process or use a different port."
            log_info "You can find and stop the process with: kill \$(lsof -t -i:$port)"
            return 2
        fi
    fi
    return 0
}

# Function to clean up existing containers
cleanup_containers() {
    log_info "Cleaning up existing containers..."

    # Check if our containers exist
    existing_containers=$(docker ps -a --filter "name=cloudtasks-emulator" --filter "name=sendgameupdates" --format "{{.Names}}" 2>/dev/null || true)

    if [ -n "$existing_containers" ]; then
        log_info "Found existing containers: $existing_containers"
        log_info "Stopping and removing them to create fresh containers..."

        # Stop and remove our specific containers
        docker stop cloudtasks-emulator sendgameupdates 2>/dev/null || true
        docker rm -f cloudtasks-emulator sendgameupdates 2>/dev/null || true

        log_success "Existing containers cleaned up"
    else
        log_info "No existing containers found"
    fi

    # Also clean up any other containers that might be using our ports
    conflicting_containers=$(docker ps -a --format "table {{.Names}}\t{{.Ports}}" | grep -E ":8080->|:8123->" | awk '{print $1}' | grep -v NAMES | grep -v "cloudtasks-emulator\|sendgameupdates" || true)

    if [ -n "$conflicting_containers" ]; then
        log_warning "Found containers using our ports: $conflicting_containers"
        echo "$conflicting_containers" | xargs -r docker rm -f 2>/dev/null || true
        log_success "Conflicting containers cleaned up"
    fi
}

# Function to start Cloud Tasks emulator
start_cloud_tasks_emulator() {
    log_info "Starting Cloud Tasks emulator..."

    # Remove any existing container with the same name
    docker rm -f cloudtasks-emulator 2>/dev/null || true

    if docker run -d --name cloudtasks-emulator --network net -p 8123:8123 \
        ghcr.io/aertje/cloud-tasks-emulator:latest --host=0.0.0.0; then

        # Wait for emulator to be ready
        log_info "Waiting for Cloud Tasks emulator to be ready..."
        sleep 5

        # Check if container is running
        if docker ps | grep -q cloudtasks-emulator; then
            log_success "Cloud Tasks emulator started successfully"

            # Test if the emulator is responding
            for i in {1..10}; do
                if curl -f -s http://localhost:8123 >/dev/null 2>&1; then
                    log_success "Cloud Tasks emulator is responding"
                    break
                elif [ $i -eq 10 ]; then
                    log_warning "Cloud Tasks emulator may not be fully ready (this is sometimes normal)"
                else
                    sleep 1
                fi
            done
        else
            log_error "Cloud Tasks emulator container is not running"
            docker logs cloudtasks-emulator 2>/dev/null || true
            exit 1
        fi
    else
        log_error "Failed to start Cloud Tasks emulator"
        exit 1
    fi
}

# Function to build and run the main service container
build_and_run_service() {
    log_info "Building main service Docker image..."

    cd watchgameupdates
    if [ ! -f "Dockerfile" ]; then
        log_error "Dockerfile not found in watchgameupdates directory"
        exit 1
    fi

    if docker build -t sendgameupdates .; then
        log_success "Docker image built successfully"
    else
        log_error "Failed to build Docker image"
        exit 1
    fi

    log_info "Starting main service container..."

    # Remove any existing container with the same name
    docker rm -f sendgameupdates 2>/dev/null || true

    if docker run -d -p 8080:8080 --name sendgameupdates --network net \
        --env-file .env sendgameupdates; then

        # Wait for service to be ready
        log_info "Waiting for main service to be ready..."
        sleep 5

        if docker ps | grep -q sendgameupdates; then
            log_success "Main service container started successfully"

            # Test if the service is responding
            for i in {1..15}; do
                if curl -f -s http://localhost:8080 >/dev/null 2>&1; then
                    log_success "Main service is responding on port 8080"
                    break
                elif [ $i -eq 15 ]; then
                    log_warning "Main service may not be fully ready yet (this can be normal on first start)"
                    log_info "You can check the service status with: docker logs sendgameupdates"
                else
                    sleep 2
                fi
            done
        else
            log_error "Main service container is not running"
            log_info "Checking container logs..."
            docker logs sendgameupdates 2>/dev/null || true
            exit 1
        fi
    else
        log_error "Failed to start main service container"
        log_info "This might be due to a port conflict. Checking what's using port 8080..."
        lsof -i :8080 2>/dev/null || true
        exit 1
    fi

    cd - > /dev/null
}

# Function to run initial setup for cloud tasks
setup_cloud_tasks() {
    log_info "Setting up Cloud Tasks queue..."

    cd localCloudTasksTest
    ./localCloudTasksTest
    log_success "Cloud Tasks queue setup completed"
    cd - > /dev/null
}

# Function to test the setup
test_setup() {
    log_info "Testing the setup..."

    # Test if the service is responding
    sleep 2
    if curl -f -s http://localhost:8080 > /dev/null; then
        log_success "Main service is responding on port 8080"
    else
        log_warning "Main service may not be fully ready yet (this is normal on first start)"
    fi

    # Test if Cloud Tasks emulator is responding
    if curl -f -s http://localhost:8123 > /dev/null 2>&1; then
        log_success "Cloud Tasks emulator is responding on port 8123"
    else
        log_warning "Cloud Tasks emulator may not be fully ready yet"
    fi
}

# Function to monitor running containers
monitor_containers() {
    log_info "Monitoring containers... (Press Ctrl+C to stop)"
    echo ""
    log_info "Container status:"
    docker ps --filter "name=cloudtasks-emulator" --filter "name=sendgameupdates" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""

    # Monitor container health and display logs
    while true; do
        # Check if both containers are still running
        running_containers=$(docker ps --filter "name=cloudtasks-emulator" --filter "name=sendgameupdates" --format "{{.Names}}" | wc -l)

        if [ "$running_containers" -lt 2 ]; then
            log_warning "One or more containers have stopped. Checking status..."

            # Show status of our containers
            docker ps -a --filter "name=cloudtasks-emulator" --filter "name=sendgameupdates" --format "table {{.Names}}\t{{.Status}}"

            # Check if containers exited with errors
            failed_containers=$(docker ps -a --filter "name=cloudtasks-emulator" --filter "name=sendgameupdates" --filter "status=exited" --format "{{.Names}}")

            if [ -n "$failed_containers" ]; then
                log_error "Containers failed: $failed_containers"
                log_info "Showing logs for failed containers:"
                for container in $failed_containers; do
                    echo "--- Logs for $container ---"
                    docker logs --tail 20 "$container" 2>&1 || true
                    echo ""
                done
                log_error "Services have failed. Check the logs above for details."
                exit 1
            fi
        fi

        # Sleep and continue monitoring
        sleep 10
    done
}

# Main setup function
main() {
    log_info "Starting CrashTheCrease Backend Local Setup..."
    echo "================================================"

    # Check prerequisites
    log_info "Checking prerequisites..."
    check_go_version

    DOCKER_AVAILABLE=true
    if ! check_docker; then
        DOCKER_AVAILABLE=false
        log_warning "Docker setup will be skipped"
    fi

    # Set up data directory
    setup_data_directory

    # Install dependencies for all Go modules
    log_info "Installing Go module dependencies..."
    install_go_dependencies "watchgameupdates" "WatchGameUpdates Service"
    install_go_dependencies "localCloudTasksTest" "Local Cloud Tasks Test"

    # Build services
    log_info "Building Go services..."
    build_go_service "watchgameupdates" "WatchGameUpdates Service" "watchgameupdates"
    build_go_service "localCloudTasksTest" "Local Cloud Tasks Test" "localCloudTasksTest"

    # Make helper scripts executable
    chmod +x GetEventDataByDate.sh
    chmod +x watchgameupdates/scripts/local_cloud_task_test.sh

    # Docker setup (if available and not skipped)
    if [ "$DOCKER_AVAILABLE" = true ] && [ "${SKIP_DOCKER:-false}" != "true" ]; then
        # Check for port conflicts before starting
        log_info "Checking port availability..."

        check_port_availability 8123 "Cloud Tasks Emulator"
        emulator_port_status=$?

        check_port_availability 8080 "Main Service"
        service_port_status=$?

        if [ $emulator_port_status -eq 2 ] || [ $service_port_status -eq 2 ]; then
            log_error "Critical port conflicts detected. Please resolve them and try again."
            exit 1
        fi

        setup_docker_network
        pull_docker_images
        cleanup_containers

        # Wait a moment for cleanup to complete
        sleep 2

        start_cloud_tasks_emulator
        build_and_run_service

        # Wait a moment for services to stabilize
        sleep 5

        # Set up Cloud Tasks queue
        setup_cloud_tasks

        # Test the setup
        test_setup

        # Keep the script running and monitor containers
        monitor_containers
    fi

    echo "================================================"
    log_success "Setup completed successfully!"
    echo ""

    if [ "$DOCKER_AVAILABLE" = true ] && [ "${SKIP_DOCKER:-false}" != "true" ]; then
        log_info "Services are running in Docker containers:"
        echo "  - Cloud Tasks Emulator: http://localhost:8123"
        echo "  - Main Service: http://localhost:8080"
        echo ""
        log_info "Available for testing:"
        echo "  - Use the GetEventDataByDate.sh script to fetch game data"
        echo "  - Run localCloudTasksTest/localCloudTasksTest to create tasks"
        echo "  - Check Docker containers: docker ps"
        echo ""
        log_info "Press Ctrl+C to stop services (containers will be preserved for faster restart)"
    else
        log_info "Docker setup skipped. Available binaries:"
        echo "  - ./watchgameupdates/watchgameupdates (main service)"
        echo "  - ./localCloudTasksTest/localCloudTasksTest (test client)"
        echo ""
        log_info "Available scripts:"
        echo "  - ./GetEventDataByDate.sh (fetch NHL game data)"
        echo "  - ./watchgameupdates/scripts/local_cloud_task_test.sh (Docker setup reference)"
    fi
}

# Cleanup function for graceful shutdown
cleanup_on_interrupt() {
    log_info "Received interrupt signal, stopping containers..."

    # Stop containers gracefully but don't remove them
    docker stop cloudtasks-emulator sendgameupdates 2>/dev/null || true

    log_success "Containers stopped. They will be cleaned up and rebuilt on next script run."
    log_info "To manually clean up: docker rm cloudtasks-emulator sendgameupdates"
    exit 0
}

# Cleanup function for failures
cleanup_on_failure() {
    if [ "$1" != "0" ]; then
        log_info "Setup failed, stopping containers..."
        docker stop cloudtasks-emulator sendgameupdates 2>/dev/null || true

        if [ "${CLEANUP_ON_EXIT:-true}" = "true" ]; then
            log_info "Removing failed containers..."
            docker rm cloudtasks-emulator sendgameupdates 2>/dev/null || true
        else
            log_info "Leaving containers for debugging. Clean up with: docker rm cloudtasks-emulator sendgameupdates"
        fi
    fi
}

# Set up signal traps
trap 'cleanup_on_interrupt' INT TERM
trap 'cleanup_on_failure $?' EXIT

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-docker)
            SKIP_DOCKER=true
            shift
            ;;
        --no-cleanup)
            CLEANUP_ON_EXIT=false
            shift
            ;;
        --help|-h)
            echo "CrashTheCrease Backend Local Setup Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --skip-docker    Skip Docker setup (build binaries only)"
            echo "  --no-cleanup     Don't cleanup on failure"
            echo "  --help, -h       Show this help message"
            echo ""
            echo "This script will:"
            echo "  1. Check Go installation and version (requires 1.23.3+)"
            echo "  2. Install dependencies for all Go modules"
            echo "  3. Build all services and binaries"
            echo "  4. Set up data directory"
            echo "  5. Set up Docker environment (Cloud Tasks emulator + main service)"
            echo "  6. Create Cloud Tasks queue"
            echo "  7. Test the setup"
            echo "  8. Keep running and monitor containers"
            echo ""
            echo "Behavior:"
            echo "  - Script keeps running until you press Ctrl+C"
            echo "  - Containers are stopped (not deleted) when script exits"
            echo "  - Next run will clean up and rebuild containers"
            echo "  - This provides faster restarts during development"
            echo ""
            echo "Environment Variables:"
            echo "  SKIP_DOCKER=true       Same as --skip-docker"
            echo "  CLEANUP_ON_EXIT=false  Same as --no-cleanup"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main setup
main
