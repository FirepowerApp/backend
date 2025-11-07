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

# Generate unique timestamp suffix for this run
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Container and network names (with timestamp for new containers)
BACKEND_CONTAINER="watchgameupdates-${TIMESTAMP}"
BACKEND_IMAGE="watchgameupdates:${TIMESTAMP}"
TESTSERVER_CONTAINER="mockdataapi-testserver-1"
CLOUDTASKS_CONTAINER="cloudtasks-emulator-${TIMESTAMP}"
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

stop_old_containers_on_ports() {
    log_info "Checking for containers using required ports..."

    # Stop any containers using port 8080 (backend)
    local port_8080_container=$(docker ps --format "{{.Names}}\t{{.Ports}}" | grep ":8080->" | awk '{print $1}' | head -n1)
    if [ -n "$port_8080_container" ]; then
        log_info "Stopping container using port 8080: $port_8080_container"
        docker stop "$port_8080_container" >/dev/null 2>&1 || true
        log_success "Container stopped (preserved for log inspection)"
    fi

    # Stop any containers using port 8123 (cloud tasks)
    local port_8123_container=$(docker ps --format "{{.Names}}\t{{.Ports}}" | grep ":8123->" | awk '{print $1}' | head -n1)
    if [ -n "$port_8123_container" ]; then
        log_info "Stopping container using port 8123: $port_8123_container"
        docker stop "$port_8123_container" >/dev/null 2>&1 || true
        log_success "Container stopped (preserved for log inspection)"
    fi
}

check_existing_containers() {
    log_info "Checking for existing test containers..."

    # List all our previous test containers (stopped and running)
    existing_containers=$(docker ps -a --filter "label=ctc.script=run_automated_test" --format "table {{.Names}}\t{{.Status}}\t{{.CreatedAt}}" 2>/dev/null || true)

    if [ -n "$existing_containers" ]; then
        log_info "Found previous test containers from earlier runs:"
        echo "$existing_containers" | head -n 5
        if [ $(echo "$existing_containers" | wc -l) -gt 5 ]; then
            log_info "... and more. Use 'docker ps -a --filter label=ctc.script=run_automated_test' to see all."
        fi
        echo ""
        log_info "Previous containers are preserved for log inspection"
        log_info "To clean up old stopped test containers: docker rm \$(docker ps -aq --filter label=ctc.script=run_automated_test --filter status=exited)"
    fi
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
    log_info "Building backend Docker image: $BACKEND_IMAGE"
    docker build -t "$BACKEND_IMAGE" watchgameupdates/ >/dev/null 2>&1
    log_success "Backend image built"

    log_info "Starting backend container: $BACKEND_CONTAINER"
    docker run -d \
        -p 8080:8080 \
        --name "$BACKEND_CONTAINER" \
        --network "$NETWORK_NAME" \
        --label "ctc.script=run_automated_test" \
        --label "ctc.service=watchgameupdates" \
        --label "ctc.timestamp=$TIMESTAMP" \
        --label "ctc.created=$(date -Iseconds)" \
        --label "ctc.env-file=$ENV_FILE" \
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
    log_info "Starting cloud tasks emulator: $CLOUDTASKS_CONTAINER"

    if docker run -d \
        --name "$CLOUDTASKS_CONTAINER" \
        --network "$NETWORK_NAME" \
        -p 8123:8123 \
        --label "ctc.script=run_automated_test" \
        --label "ctc.service=cloudtasks-emulator" \
        --label "ctc.timestamp=$TIMESTAMP" \
        --label "ctc.created=$(date -Iseconds)" \
        "$CLOUDTASKS_IMAGE" --host=0.0.0.0 >/dev/null 2>&1; then
        log_success "Cloud tasks emulator started"
    else
        log_error "Failed to start cloud tasks emulator"
        log_error "This may be due to port 8123 being in use or the image not being available"
        log_info "You can try manually running: docker run -d --name $CLOUDTASKS_CONTAINER --network $NETWORK_NAME -p 8123:8123 $CLOUDTASKS_IMAGE --host=0.0.0.0"
        exit 1
    fi
}

wait_for_services() {
    log_info "Waiting for services to be ready..."
    sleep 5  # Give services time to start up

    # Check if testserver is responding (only if testserver is expected to be running)
    if [ "$ENV_FILE" != ".env.home" ]; then
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
    else
        log_info "Skipping testserver health check (using live APIs)"
    fi

    # Check if backend is responding (expect 400/405 for GET request)
    local retries=0
    local max_retries=30
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
    log_info "Stopping test containers..."

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

    log_success "Test containers stopped (preserved for log inspection)"
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
    echo ""
    echo "Notes:"
    echo "  • Each run creates uniquely timestamped containers"
    echo "  • Containers are preserved after stopping for log inspection"
    echo "  • Old containers can be cleaned up manually when no longer needed"
}

wait_for_interrupt() {
    log_info "Containers are running and ready for use"
    echo ""
    log_info "Container names for this run:"
    echo "  • Backend: $BACKEND_CONTAINER"
    echo "  • Cloud Tasks: $CLOUDTASKS_CONTAINER"
    if [ "$ENV_FILE" != ".env.home" ]; then
        echo "  • Testserver: $TESTSERVER_CONTAINER"
    fi
    echo ""
    log_info "Services available at:"
    log_info "  • Backend: http://localhost:8080"
    if [ "$ENV_FILE" != ".env.home" ]; then
        log_info "  • Testserver: http://localhost:8125"
    fi
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

    # Step 1: Stop any containers using our ports
    stop_old_containers_on_ports

    # Step 2: Show existing containers info
    check_existing_containers

    # Step 3: Create network if needed
    create_network

    # Step 4: Build and start all services
    build_and_run_backend

    # Only start testserver if not using home environment (which uses live APIs)
    if [ "$ENV_FILE" != ".env.home" ]; then
        start_testserver
    else
        log_info "Skipping testserver (using live APIs with .env.home)"
    fi

    start_cloudtasks_emulator

    # Step 5: Wait for services to be ready
    wait_for_services

    if [ "$CONTAINERS_ONLY" = true ]; then
        # Containers-only mode: wait for interrupt
        wait_for_interrupt
    else
        # Full test mode: run tests and monitor
        # Step 6: Run the test
        run_cloud_task_test

        # Step 7: Monitor logs for completion
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

        # Step 8: Stop containers (but keep them for inspection)
        stop_all_containers

        echo ""
        log_success "🎉 Automated test completed successfully!"
        echo ""
        echo "📋 What was tested:"
        echo "  ✓ Backend container built and started"
        if [ "$ENV_FILE" != ".env.home" ]; then
            echo "  ✓ Testserver provided mock NHL and MoneyPuck API data"
        else
            echo "  ✓ Backend used live NHL and MoneyPuck API data"
        fi
        echo "  ✓ Cloud tasks emulator handled task scheduling"
        echo "  ✓ Test sequence initiated and completed successfully"
        echo "  ✓ Backend processed all game events until completion"
        echo ""
        echo "🔍 Test containers are stopped but preserved for inspection:"
        echo "  • Backend logs: docker logs $BACKEND_CONTAINER"
        echo "  • Cloud Tasks logs: docker logs $CLOUDTASKS_CONTAINER"
        if [ "$ENV_FILE" != ".env.home" ]; then
            echo "  • Testserver logs: docker logs $TESTSERVER_CONTAINER"
        fi
        echo ""
        echo "🧹 To clean up this test run's containers:"
        if [ "$ENV_FILE" != ".env.home" ]; then
            echo "  docker rm $BACKEND_CONTAINER $CLOUDTASKS_CONTAINER $TESTSERVER_CONTAINER"
        else
            echo "  docker rm $BACKEND_CONTAINER $CLOUDTASKS_CONTAINER"
        fi
        echo ""
        echo "ℹ️  Next test run will create new timestamped containers"
    fi
}

# Handle script interruption
cleanup_on_interrupt() {
    echo ""
    log_warning "Script interrupted. Stopping containers..."
    stop_all_containers

    if [ "$CONTAINERS_ONLY" = true ]; then
        echo ""
        log_success "🧹 Test containers stopped successfully"
        echo ""
        echo "🔍 To inspect stopped containers:"
        echo "  • Backend logs: docker logs $BACKEND_CONTAINER"
        echo "  • Cloud Tasks logs: docker logs $CLOUDTASKS_CONTAINER"
        if [ "$ENV_FILE" != ".env.home" ]; then
            echo "  • Testserver logs: docker logs $TESTSERVER_CONTAINER"
        fi
        echo ""
        echo "🗑️ To remove stopped containers:"
        if [ "$ENV_FILE" != ".env.home" ]; then
            echo "  docker rm $BACKEND_CONTAINER $CLOUDTASKS_CONTAINER $TESTSERVER_CONTAINER"
        else
            echo "  docker rm $BACKEND_CONTAINER $CLOUDTASKS_CONTAINER"
        fi
        echo ""
        echo "ℹ️  Next test run will create new timestamped containers"
    fi

    exit 0
}

trap cleanup_on_interrupt INT TERM

# Run main function
main "$@"
