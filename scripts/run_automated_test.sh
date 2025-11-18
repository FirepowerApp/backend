#!/bin/bash

###############################################################################
# Automated Test Script for CrashTheCrease Backend
#
# This script automates the complete testing workflow by:
# 1. Fetching latest gameDataEmulator container from registry (with smart fallback)
# 2. Cleaning up existing containers
# 3. Building and running all required containers
# 4. Initiating the test sequence (optional)
# 5. Monitoring logs for completion (optional)
# 6. Cleaning up containers after test completion
#
# Container Registry Management:
#   - Automatically pulls latest gameDataEmulator from Docker Hub (blnelson/firepowermockdataserver)
#   - Compares with local version and updates if different
#   - Falls back to local cache on network errors
#   - Removes old image versions to save space
#
# Usage:
#   ./scripts/run_automated_test.sh                    # Full test execution (uses .env.local)
#   ./scripts/run_automated_test.sh --containers-only  # Start containers only (uses .env.local)
#   ./scripts/run_automated_test.sh --env-home         # Full test execution (uses .env.home, live APIs)
#   ./scripts/run_automated_test.sh --strict-registry  # Fail if can't get latest from registry
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
CLOUDTASKS_CONTAINER="cloudtasks-emulator"
CLOUDTASKS_IMAGE="ghcr.io/aertje/cloud-tasks-emulator:latest"
EMULATOR_CONTAINER="firepowermockdataserver"
EMULATOR_IMAGE="blnelson/firepowermockdataserver:latest"
NETWORK_NAME="net"

# Log monitoring settings
LOG_TARGET="Last play type: game-end, Should reschedule: false"
MAX_WAIT_TIME=900  # Maximum wait time in seconds (15 minutes)

# Script configuration
CONTAINERS_ONLY=false
ENV_FILE=".env.local"  # Default environment file
STRICT_REGISTRY=false  # Fail if can't get latest from registry

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

    # Clean up individual containers (leave cloud tasks emulator running)
    cleanup_container "$BACKEND_CONTAINER"
    cleanup_container "$EMULATOR_CONTAINER"

    log_success "Test containers cleaned up (cloud tasks emulator preserved)"
}

###############################################################################
# Setup Functions
###############################################################################

# Check if error is network-related
is_network_error() {
    local error_msg="$1"
    # Common network error patterns
    if echo "$error_msg" | grep -qiE "(connection refused|could not resolve host|network is unreachable|timeout|temporary failure|no route to host|connection timed out)"; then
        return 0
    fi
    return 1
}

# Ensure we have the latest game data emulator container from registry
ensure_emulator_container() {
    log_info "Checking game data emulator container..."

    # Check if image exists locally
    local has_local=false
    local local_digest=""
    if docker image inspect "$EMULATOR_IMAGE" >/dev/null 2>&1; then
        has_local=true
        local_digest=$(docker image inspect "$EMULATOR_IMAGE" --format '{{index .RepoDigests 0}}' 2>/dev/null || echo "")
        log_info "Found local image: $EMULATOR_IMAGE"
        if [ -n "$local_digest" ]; then
            log_info "Local digest: $local_digest"
        fi
    fi

    # Try to pull the latest from registry
    log_info "Attempting to pull latest image from registry..."
    local pull_output
    local pull_exit_code=0
    pull_output=$(docker pull "$EMULATOR_IMAGE" 2>&1) || pull_exit_code=$?

    if [ $pull_exit_code -eq 0 ]; then
        log_success "Successfully pulled latest image from registry"

        # Get new digest
        local new_digest=$(docker image inspect "$EMULATOR_IMAGE" --format '{{index .RepoDigests 0}}' 2>/dev/null || echo "")

        # Check if image changed
        if [ "$has_local" = true ] && [ -n "$local_digest" ] && [ -n "$new_digest" ]; then
            if [ "$local_digest" != "$new_digest" ]; then
                log_info "New version detected (digest changed)"
                log_info "Old: $local_digest"
                log_info "New: $new_digest"

                # Remove old image if it's different (cleanup old versions)
                if [ -n "$local_digest" ]; then
                    local old_image_id="${local_digest##*/}"
                    log_info "Cleaning up old image version..."
                    docker rmi "$old_image_id" >/dev/null 2>&1 || true
                fi
            else
                log_info "Local image is already up to date"
            fi
        fi

        return 0
    fi

    # Pull failed - check if it's a network error
    if is_network_error "$pull_output"; then
        log_warning "Network error while pulling image from registry"

        if [ "$STRICT_REGISTRY" = true ]; then
            log_error "Strict registry mode enabled - failing due to network error"
            log_error "Error: $pull_output"
            return 1
        fi

        # Network error - fall back to local if available
        if [ "$has_local" = true ]; then
            log_warning "Falling back to locally cached image due to network error"
            log_info "Using local image: $EMULATOR_IMAGE"
            return 0
        else
            log_error "No local image available and cannot reach registry due to network error"
            log_error "Error: $pull_output"
            return 1
        fi
    else
        # Non-network error - always fail
        log_error "Failed to pull image from registry (non-network error)"
        log_error "Error: $pull_output"

        if [ "$has_local" = true ] && [ "$STRICT_REGISTRY" = false ]; then
            log_warning "Using local image despite registry error (non-strict mode)"
            log_info "Using local image: $EMULATOR_IMAGE"
            return 0
        fi

        return 1
    fi
}

start_emulator() {
    # Ensure we have the latest container
    if ! ensure_emulator_container; then
        log_error "Failed to ensure game data emulator container is available"
        return 1
    fi

    log_info "Starting game data emulator container..."

    if ! docker run -d \
        --name "$EMULATOR_CONTAINER" \
        --network "$NETWORK_NAME" \
        -p 8124:8124 \
        -p 8125:8125 \
        "$EMULATOR_IMAGE" >/dev/null 2>&1; then
        log_error "Failed to start game data emulator container"
        return 1
    fi

    log_success "Game data emulator started"
    return 0
}

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

start_cloudtasks_emulator() {
    if container_running "$CLOUDTASKS_CONTAINER"; then
        log_info "Cloud tasks emulator is already running, skipping startup"

        # Ensure the emulator is connected to our network
        if ! docker network inspect "$NETWORK_NAME" --format '{{range .Containers}}{{.Name}} {{end}}' | grep -q "$CLOUDTASKS_CONTAINER"; then
            log_info "Connecting $CLOUDTASKS_CONTAINER to network $NETWORK_NAME"
            docker network connect "$NETWORK_NAME" "$CLOUDTASKS_CONTAINER" 2>/dev/null || true
        fi

        log_success "Using existing cloud tasks emulator"
    else
        # Check if container exists but is stopped
        if container_exists "$CLOUDTASKS_CONTAINER"; then
            log_info "Cloud tasks emulator container exists but is stopped, starting it..."
            if docker start "$CLOUDTASKS_CONTAINER" >/dev/null 2>&1; then
                # Ensure it's connected to our network
                if ! docker network inspect "$NETWORK_NAME" --format '{{range .Containers}}{{.Name}} {{end}}' | grep -q "$CLOUDTASKS_CONTAINER"; then
                    log_info "Connecting $CLOUDTASKS_CONTAINER to network $NETWORK_NAME"
                    docker network connect "$NETWORK_NAME" "$CLOUDTASKS_CONTAINER" 2>/dev/null || true
                fi
                log_success "Cloud tasks emulator started"
            else
                log_error "Failed to start existing cloud tasks emulator container"
                log_info "Removing existing container and creating a new one..."
                docker rm "$CLOUDTASKS_CONTAINER" >/dev/null 2>&1 || true
            fi
        fi

        # Only create new container if we don't have a running one
        if ! container_running "$CLOUDTASKS_CONTAINER"; then
            log_info "Starting cloud tasks emulator..."
            if docker run -d \
                --name "$CLOUDTASKS_CONTAINER" \
                --network "$NETWORK_NAME" \
                -p 8123:8123 \
                "$CLOUDTASKS_IMAGE" --host=0.0.0.0 >/dev/null 2>&1; then
                log_success "Cloud tasks emulator started"
            else
                log_error "Failed to start cloud tasks emulator"
                log_error "This may be due to port 8123 being in use or the image not being available"
                log_info "You can try manually running: docker run -d --name $CLOUDTASKS_CONTAINER --network $NETWORK_NAME -p 8123:8123 $CLOUDTASKS_IMAGE --host=0.0.0.0"
                exit 1
            fi
        fi
    fi
}

wait_for_services() {
    log_info "Waiting for services to be ready..."
    sleep 5  # Give services time to start up

    # Check if emulator is responding (only if not using home environment)
    if [ "$ENV_FILE" != ".env.home" ]; then
        local retries=0
        local max_retries=30
        while ! curl -s http://localhost:8125/v1/gamecenter/test/play-by-play >/dev/null 2>&1; do
            retries=$((retries + 1))
            if [ $retries -gt $max_retries ]; then
                log_warning "Game data emulator health check failed, continuing anyway..."
                break
            fi
            sleep 1
        done
    else
        log_info "Skipping emulator health check (using live APIs)"
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
    log_info "Stopping test containers (preserving cloud tasks emulator)..."

    # Stop individual containers (don't remove them)
    if container_running "$BACKEND_CONTAINER"; then
        docker stop "$BACKEND_CONTAINER" >/dev/null 2>&1 || true
    fi

    # Stop emulator container if running
    if container_running "$EMULATOR_CONTAINER"; then
        docker stop "$EMULATOR_CONTAINER" >/dev/null 2>&1 || true
    fi

    log_success "Test containers stopped (cloud tasks emulator left running)"
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
            --strict-registry)
                STRICT_REGISTRY=true
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
    echo "  --strict-registry   Fail if unable to fetch latest container from registry for any reason"
    echo "  -h, --help          Show this help message"
    echo ""
    echo "Container Registry Behavior:"
    echo "  By default, the script tries to pull the latest gameDataEmulator from Docker Hub."
    echo "  - If successful: uses the latest image, removes old local versions"
    echo "  - If network error: falls back to local cached image (or fails if none)"
    echo "  - If other error: uses local cached image in non-strict mode"
    echo "  - With --strict-registry: fails immediately if unable to get latest for any reason"
    echo ""
    echo "Examples:"
    echo "  $0                              # Run full automated test (uses .env.local)"
    echo "  $0 --containers-only           # Start containers and keep running (uses .env.local)"
    echo "  $0 --env-home                  # Run full automated test (uses .env.home)"
    echo "  $0 --strict-registry           # Run test, fail if can't get latest emulator"
    echo "  $0 --containers-only --env-home # Start containers and keep running (uses .env.home)"
}

wait_for_interrupt() {
    log_info "Containers are running and ready for use"
    log_info "Services available at:"
    log_info "  • Backend: http://localhost:8080"
    if [ "$ENV_FILE" != ".env.home" ]; then
        log_info "  • Game Data Emulator: http://localhost:8125 (play-by-play), http://localhost:8124 (stats)"
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

    # Step 1: Cleanup existing containers
    cleanup_all

    # Step 2: Create network if needed
    create_network

    # Step 3: Build and start all services
    build_and_run_backend

    # Only start emulator if not using home environment (which uses live APIs)
    if [ "$ENV_FILE" != ".env.home" ]; then
        if ! start_emulator; then
            log_error "Failed to start game data emulator"
            stop_all_containers
            exit 1
        fi
    else
        log_info "Skipping game data emulator (using live APIs with .env.home)"
    fi

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
            if [ "$ENV_FILE" != ".env.home" ]; then
                log_info "  Game Data Emulator: docker logs $EMULATOR_CONTAINER"
            fi
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
        if [ "$ENV_FILE" != ".env.home" ]; then
            echo "  ✓ Game data emulator provided mock NHL and MoneyPuck API data"
        else
            echo "  ✓ Backend used live NHL and MoneyPuck API data"
        fi
        echo "  ✓ Cloud tasks emulator handled task scheduling"
        echo "  ✓ Test sequence initiated and completed successfully"
        echo "  ✓ Backend processed all game events until completion"
        echo ""
        echo "🔍 Test containers are stopped but preserved for inspection:"
        echo "  • Backend logs: docker logs $BACKEND_CONTAINER"
        if [ "$ENV_FILE" != ".env.home" ]; then
            echo "  • Game Data Emulator logs: docker logs $EMULATOR_CONTAINER"
        fi
        echo ""
        echo "🧹 To clean up stopped test containers:"
        if [ "$ENV_FILE" != ".env.home" ]; then
            echo "  docker rm $BACKEND_CONTAINER $EMULATOR_CONTAINER"
        else
            echo "  docker rm $BACKEND_CONTAINER"
        fi
        echo ""
        echo "ℹ️  Cloud tasks emulator left running at http://localhost:8123"
        echo "   To stop it manually: docker stop $CLOUDTASKS_CONTAINER"
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
        if [ "$ENV_FILE" != ".env.home" ]; then
            echo "  • Game Data Emulator logs: docker logs $EMULATOR_CONTAINER"
        fi
        echo ""
        echo "🗑️ To remove stopped containers:"
        if [ "$ENV_FILE" != ".env.home" ]; then
            echo "  docker rm $BACKEND_CONTAINER $EMULATOR_CONTAINER"
        else
            echo "  docker rm $BACKEND_CONTAINER"
        fi
        echo ""
        echo "ℹ️  Cloud tasks emulator left running at http://localhost:8123"
        echo "   To stop it manually: docker stop $CLOUDTASKS_CONTAINER"
    fi

    exit 0
}

trap cleanup_on_interrupt INT TERM

# Run main function
main "$@"
