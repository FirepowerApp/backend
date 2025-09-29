#!/bin/bash

###############################################################################
# Automated Test Script for CrashTheCrease Backend
#
# This script automates the complete testing workflow by:
# 1. Cleaning up existing containers
# 2. Building and running all required containers
# 3. Initiating the test sequence (optional)
# 4. Monitoring logs for completion (optional)
# 5. Cleaning up containers after test completion
#
# Usage:
#   ./scripts/run_automated_test.sh                    # Full test execution (uses .env.local)
#   ./scripts/run_automated_test.sh --containers-only  # Start containers only (uses .env.local)
#   ./scripts/run_automated_test.sh --env-home         # Full test execution (uses .env.home)
#   ./scripts/run_automated_test.sh --containers-only --env-home  # Start containers only (uses .env.home)
###############################################################################

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Container and network names
BACKEND_CONTAINER="watchgameupdates"
BACKEND_IMAGE="watchgameupdates"
TESTSERVER_CONTAINER="mockdataapi-testserver-1"
CLOUDTASKS_CONTAINER="cloudtasks-emulator"
CLOUDTASKS_IMAGE="ghcr.io/aertje/cloud-tasks-emulator:latest"
NETWORK_NAME="net"

# Log monitoring settings
LOG_TARGET="Last play type: game-end, Should reschedule: false"
MAX_WAIT_TIME=900  # Maximum wait time in seconds (15 minutes)

# Script configuration
CONTAINERS_ONLY=false
ENV_FILE=".env.local"  # Default environment file

###############################################################################
# Helper Functions
###############################################################################

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

# Function to check if container exists
container_exists() {
    docker ps -a --format "table {{.Names}}" | grep -q "^$1$"
}

# Function to check if container is running
container_running() {
    docker ps --format "table {{.Names}}" | grep -q "^$1$"
}

# Function to check if network exists
network_exists() {
    docker network ls --format "{{.Name}}" | grep -q "^$1$"
}

###############################################################################
# Cleanup Functions
###############################################################################

cleanup_container() {
    local container_name=$1
    if container_exists "$container_name"; then
        log_info "Stopping and removing existing container: $container_name"
        docker stop "$container_name" >/dev/null 2>&1 || true
        docker rm "$container_name" >/dev/null 2>&1 || true
        log_success "Container $container_name cleaned up"
    fi
}

cleanup_all() {
    log_info "Cleaning up existing containers and services..."

    # Clean up individual containers
    cleanup_container "$BACKEND_CONTAINER"
    cleanup_container "$CLOUDTASKS_CONTAINER"

    log_success "All containers cleaned up"
}

###############################################################################
# Setup Functions
###############################################################################

create_network() {
    if ! network_exists "$NETWORK_NAME"; then
        log_info "Creating Docker network: $NETWORK_NAME"
        docker network create "$NETWORK_NAME" >/dev/null 2>&1
        log_success "Network $NETWORK_NAME created"
    else
        log_info "Network $NETWORK_NAME already exists"
    fi
}

build_and_run_backend() {
    log_info "Building backend Docker image..."
    docker build -t "$BACKEND_IMAGE" watchgameupdates/ >/dev/null 2>&1
    log_success "Backend image built"

    log_info "Starting backend container with env file: $ENV_FILE..."
    docker run -d \
        -p 8080:8080 \
        --name "$BACKEND_CONTAINER" \
        --network "$NETWORK_NAME" \
        --env-file "watchgameupdates/$ENV_FILE" \
        "$BACKEND_IMAGE" >/dev/null 2>&1
    log_success "Backend container started"
}

start_testserver() {
    log_info "Starting existing testserver container..."

    # Check if the mockdataapi container exists
    if ! container_exists "$TESTSERVER_CONTAINER"; then
        log_error "Container '$TESTSERVER_CONTAINER' does not exist!"
        log_error "Please ensure the mockdataapi container is built and available."
        log_error "Expected container name: $TESTSERVER_CONTAINER"
        exit 1
    fi

    # Start the container if it's not running
    if ! container_running "$TESTSERVER_CONTAINER"; then
        log_info "Starting container: $TESTSERVER_CONTAINER"
        if ! docker start "$TESTSERVER_CONTAINER" >/dev/null 2>&1; then
            log_error "Failed to start container: $TESTSERVER_CONTAINER"
            exit 1
        fi
    else
        log_info "Container $TESTSERVER_CONTAINER is already running"
    fi

    # Connect testserver container to our main network for inter-container communication
    if ! docker network inspect "$NETWORK_NAME" --format '{{range .Containers}}{{.Name}} {{end}}' | grep -q "$TESTSERVER_CONTAINER"; then
        log_info "Connecting $TESTSERVER_CONTAINER to network $NETWORK_NAME"
        docker network connect "$NETWORK_NAME" "$TESTSERVER_CONTAINER" 2>/dev/null || true
    fi

    log_success "Testserver started"
}

start_cloudtasks_emulator() {
    log_info "Starting cloud tasks emulator..."
    docker run -d \
        --name "$CLOUDTASKS_CONTAINER" \
        --network "$NETWORK_NAME" \
        -p 8123:8123 \
        "$CLOUDTASKS_IMAGE" --host=0.0.0.0 >/dev/null 2>&1
    log_success "Cloud tasks emulator started"
}

wait_for_services() {
    log_info "Waiting for services to be ready..."
    sleep 5  # Give services time to start up

    # Check if testserver is responding
    local retries=0
    local max_retries=30
    while ! curl -s http://localhost:8125/v1/gamecenter/test/play-by-play >/dev/null 2>&1; do
        retries=$((retries + 1))
        if [ $retries -gt $max_retries ]; then
            log_warning "Testserver health check failed, continuing anyway..."
            break
        fi
        sleep 1
    done

    # Check if backend is responding (expect 400/405 for GET request)
    retries=0
    while true; do
        http_code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080 2>/dev/null || echo "000")
        if [ "$http_code" = "400" ] || [ "$http_code" = "405" ] || [ "$http_code" = "200" ]; then
            break
        fi
        retries=$((retries + 1))
        if [ $retries -gt $max_retries ]; then
            log_warning "Backend health check failed, continuing anyway..."
            break
        fi
        sleep 1
    done

    log_success "Services are ready"
}

###############################################################################
# Test Execution Functions
###############################################################################

run_cloud_task_test() {
    log_info "Initiating test sequence with local cloud tasks test..."

    if [ ! -f "localCloudTasksTest/localCloudTasksTest" ]; then
        log_info "Building local cloud tasks test program..."
        cd localCloudTasksTest
        go build -o localCloudTasksTest main.go
        cd ..
        log_success "Local cloud tasks test program built"
    fi

    # Run the test program
    ./localCloudTasksTest/localCloudTasksTest >/dev/null 2>&1

    log_success "Test sequence initiated"
}

monitor_backend_logs() {
    log_info "Monitoring backend logs for completion signal..."
    log_info "Looking for: '$LOG_TARGET'"

    local start_time=$(date +%s)
    local found=false

    while true; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))

        # Check for timeout
        if [ $elapsed -gt $MAX_WAIT_TIME ]; then
            log_error "Timeout reached ($MAX_WAIT_TIME seconds) without finding completion signal"
            return 1
        fi

        # Check logs for the target message
        if docker logs "$BACKEND_CONTAINER" 2>&1 | grep -q "$LOG_TARGET"; then
            found=true
            break
        fi

        # Show progress every 30 seconds
        if [ $((elapsed % 30)) -eq 0 ] && [ $elapsed -gt 0 ]; then
            log_info "Still monitoring... (${elapsed}s elapsed)"
        fi

        sleep 2
    done

    if [ "$found" = true ]; then
        log_success "Found completion signal in logs!"
        log_success "Test completed successfully"
        return 0
    else
        log_error "Failed to find completion signal"
        return 1
    fi
}

###############################################################################
# Final Cleanup Functions
###############################################################################

stop_all_containers() {
    log_info "Stopping all test containers..."

    # Stop individual containers (don't remove them)
    if container_running "$BACKEND_CONTAINER"; then
        docker stop "$BACKEND_CONTAINER" >/dev/null 2>&1 || true
    fi

    if container_running "$CLOUDTASKS_CONTAINER"; then
        docker stop "$CLOUDTASKS_CONTAINER" >/dev/null 2>&1 || true
    fi

    # Stop testserver container
    if container_running "$TESTSERVER_CONTAINER"; then
        docker stop "$TESTSERVER_CONTAINER" >/dev/null 2>&1 || true
    fi

    log_success "All containers stopped (containers preserved for inspection)"
}

###############################################################################
# Flag Parsing
###############################################################################

parse_flags() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --containers-only)
                CONTAINERS_ONLY=true
                shift
                ;;
            --env-home)
                ENV_FILE=".env.home"
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown flag: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --containers-only    Start containers only and keep running until manually stopped"
    echo "  --env-home          Use .env.home instead of .env.local for backend container"
    echo "  -h, --help          Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                              # Run full automated test (uses .env.local)"
    echo "  $0 --containers-only           # Start containers and keep running (uses .env.local)"
    echo "  $0 --env-home                  # Run full automated test (uses .env.home)"
    echo "  $0 --containers-only --env-home # Start containers and keep running (uses .env.home)"
}

wait_for_interrupt() {
    log_info "Containers are running and ready for use"
    log_info "Services available at:"
    log_info "  • Backend: http://localhost:8080"
    log_info "  • Testserver: http://localhost:8125"
    log_info "  • Cloud Tasks Emulator: http://localhost:8123"
    echo ""
    log_info "Press Ctrl+C to stop all containers and exit..."

    # Wait indefinitely until interrupted
    while true; do
        sleep 1
    done
}

###############################################################################
# Main Execution
###############################################################################

main() {
    # Parse command line flags
    parse_flags "$@"

    if [ "$CONTAINERS_ONLY" = true ]; then
        echo "🐳 Starting CrashTheCrease Backend Containers"
        echo "============================================="
    else
        echo "🚀 Starting CrashTheCrease Backend Automated Test"
        echo "=================================================="
    fi

    # Ensure we're in the right directory
    if [ ! -f "build.go" ] || [ ! -d "watchgameupdates" ]; then
        log_error "Please run this script from the project root directory"
        exit 1
    fi

    # Step 1: Cleanup existing containers
    cleanup_all

    # Step 2: Create network if needed
    create_network

    # Step 3: Build and start all services
    build_and_run_backend
    start_testserver
    start_cloudtasks_emulator

    # Step 4: Wait for services to be ready
    wait_for_services

    if [ "$CONTAINERS_ONLY" = true ]; then
        # Containers-only mode: wait for interrupt
        wait_for_interrupt
    else
        # Full test mode: run tests and monitor
        # Step 5: Run the test
        run_cloud_task_test

        # Step 6: Monitor logs for completion
        if monitor_backend_logs; then
            log_success "Test execution completed successfully!"
        else
            log_error "Test execution failed or timed out"
            log_info "Check container logs for more details:"
            log_info "  Backend: docker logs $BACKEND_CONTAINER"
            log_info "  Testserver: docker logs $TESTSERVER_CONTAINER"
            stop_all_containers
            exit 1
        fi

        # Step 7: Stop containers (but keep them for inspection)
        stop_all_containers

        echo ""
        log_success "🎉 Automated test completed successfully!"
        echo ""
        echo "📋 What was tested:"
        echo "  ✓ Backend container built and started"
        echo "  ✓ Testserver provided mock NHL and MoneyPuck API data"
        echo "  ✓ Cloud tasks emulator handled task scheduling"
        echo "  ✓ Test sequence initiated and completed successfully"
        echo "  ✓ Backend processed all game events until completion"
        echo ""
        echo "🔍 Containers are stopped but preserved for inspection:"
        echo "  • Backend logs: docker logs $BACKEND_CONTAINER"
        echo "  • Testserver logs: docker logs $TESTSERVER_CONTAINER"
        echo ""
        echo "🧹 To clean up containers completely:"
        echo "  docker rm $BACKEND_CONTAINER $CLOUDTASKS_CONTAINER $TESTSERVER_CONTAINER"
    fi
}

# Handle script interruption
cleanup_on_interrupt() {
    echo ""
    log_warning "Script interrupted. Stopping containers..."
    stop_all_containers

    if [ "$CONTAINERS_ONLY" = true ]; then
        echo ""
        log_success "🧹 Containers stopped successfully"
        echo ""
        echo "🔍 To inspect stopped containers:"
        echo "  • Backend logs: docker logs $BACKEND_CONTAINER"
        echo "  • Testserver logs: docker logs $TESTSERVER_CONTAINER"
        echo ""
        echo "🗑️ To remove containers completely:"
        echo "  docker rm $BACKEND_CONTAINER $CLOUDTASKS_CONTAINER $TESTSERVER_CONTAINER"
    fi

    exit 0
}

trap cleanup_on_interrupt INT TERM

# Run main function
main "$@"
