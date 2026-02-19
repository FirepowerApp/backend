# CrashTheCrease Backend Makefile
# Replaces functionality from setup-local.sh and run_automated_test.sh
#
# Main targets:
#   make setup          - Initial setup (pull images, install deps)
#   make home           - Start home environment (live APIs)
#   make test           - Run automated integration tests (with mocks)
#   make test-containers - Start test containers and keep running
#   make logs           - View logs from current containers
#   make clean          - Stop and remove current containers
#   make build          - Build all Go binaries
#   make pull           - Pull all required Docker images (with retry)
#   make redis-up       - Start Redis worker environment (Asynq + Asynqmon)
#   make redis-stop     - Stop Redis worker environment

.PHONY: help setup home test test-containers clean logs build pull check-deps check-docker check-go pull-with-retry status stop restart clean-all list-containers redis-up redis-stop redis-logs redis-status build-enqueue

# Color output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m # No Color

# Docker compose files
COMPOSE_FILE := docker-compose.yml
COMPOSE_TEST := docker-compose.test.yml
COMPOSE_HOME := docker-compose.home.yml
COMPOSE_LOCAL_MOCK := docker-compose.local-mock.yml
COMPOSE_REDIS := docker-compose.redis.yml

# Local mock API flag (set LOCAL_MOCK=true to use locally built mock API image)
LOCAL_MOCK ?= false

# Generate unique project name with timestamp for container isolation
# This allows multiple executions to have separate containers for historical log access
PROJECT_NAME := backend-$(shell date +%Y%m%d-%H%M%S)
PROJECT_FILE := .current-project

# Retry settings for pulling images
MAX_RETRIES := 4
RETRY_DELAYS := 2 4 8 16

# Default target
.DEFAULT_GOAL := help

##@ General

help: ## Display this help message
	@echo "$(BLUE)CrashTheCrease Backend - Make Commands$(NC)"
	@echo "========================================"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make $(YELLOW)<target>$(NC)\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(BLUE)%-18s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(GREEN)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Setup & Prerequisites

check-go: ## Check if Go is installed and version is correct
	@printf "$(BLUE)[INFO]$(NC) Checking Go installation...\n"
	@if ! command -v go >/dev/null 2>&1; then \
		printf "$(RED)[ERROR]$(NC) Go is not installed. Please install Go 1.23.3 or later.\n"; \
		exit 1; \
	fi
	@GO_VERSION=$$(go version | awk '{print $$3}' | sed 's/go//'); \
	REQUIRED_VERSION="1.23.3"; \
	if [ "$$(printf '%s\n' "$$REQUIRED_VERSION" "$$GO_VERSION" | sort -V | head -n1)" != "$$REQUIRED_VERSION" ]; then \
		printf "$(RED)[ERROR]$(NC) Go version $$GO_VERSION is installed, but version $$REQUIRED_VERSION or later is required.\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)[SUCCESS]$(NC) Go version $$(go version | awk '{print $$3}') is compatible\n"

check-docker: ## Check if Docker is installed and running
	@printf "$(BLUE)[INFO]$(NC) Checking Docker installation...\n"
	@if ! command -v docker >/dev/null 2>&1; then \
		printf "$(RED)[ERROR]$(NC) Docker is not installed. Please install Docker from https://docs.docker.com/get-docker/\n"; \
		exit 1; \
	fi
	@if ! docker info >/dev/null 2>&1; then \
		printf "$(RED)[ERROR]$(NC) Docker is installed but not running. Please start Docker.\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)[SUCCESS]$(NC) Docker is installed and running\n"

check-deps: check-go check-docker ## Check all prerequisites (Go and Docker)
	@printf "$(GREEN)[SUCCESS]$(NC) All prerequisites are met\n"

##@ Image Management

pull-with-retry: ## Pull Docker images with retry logic for network errors
	@printf "$(BLUE)[INFO]$(NC) Pulling required Docker images with retry logic...\n"
	@attempt=1; \
	max_attempts=$$(($(MAX_RETRIES) + 1)); \
	delays=(0 $(RETRY_DELAYS)); \
	while [ $$attempt -le $$max_attempts ]; do \
		printf "$(BLUE)[INFO]$(NC) Pull attempt $$attempt/$$max_attempts for cloud tasks emulator...\n"; \
		if docker pull ghcr.io/aertje/cloud-tasks-emulator:latest 2>&1; then \
			printf "$(GREEN)[SUCCESS]$(NC) Cloud tasks emulator image pulled successfully\n"; \
			break; \
		else \
			if [ $$attempt -eq $$max_attempts ]; then \
				printf "$(RED)[ERROR]$(NC) Failed to pull cloud tasks emulator after $$max_attempts attempts\n"; \
				exit 1; \
			fi; \
			delay=$${delays[$$attempt]}; \
			printf "$(YELLOW)[WARNING]$(NC) Pull failed, retrying in $$delay seconds...\n"; \
			sleep $$delay; \
			attempt=$$((attempt + 1)); \
		fi; \
	done
	@printf "$(GREEN)[SUCCESS]$(NC) All required images pulled\n"

pull: check-docker pull-with-retry ## Pull all required Docker images
	@printf "$(GREEN)[SUCCESS]$(NC) Image pull completed\n"

##@ Development & Testing

setup: check-deps pull ## Initial setup - check prerequisites and pull images
	@printf "$(BLUE)[INFO]$(NC) Installing Go module dependencies...\n"
	@cd watchgameupdates && go mod download && go mod tidy
	@cd localCloudTasksTest && go mod download && go mod tidy
	@printf "$(GREEN)[SUCCESS]$(NC) Go dependencies installed\n"
	@printf "$(BLUE)[INFO]$(NC) Creating data directory...\n"
	@mkdir -p data
	@printf "$(GREEN)[SUCCESS]$(NC) Data directory created\n"
	@printf "$(GREEN)[SUCCESS]$(NC) Setup completed successfully!\n"
	@printf "\n$(BLUE)[INFO]$(NC) Next steps:\n"
	@printf "  - Run 'make home' to start home environment (live APIs)\n"
	@printf "  - Run 'make test' to run automated tests (with mocks)\n"
	@printf "  - Run 'make test-containers' to start test environment and keep running\n"

home: check-docker ## Start home environment with live APIs (.env.home)
	@printf "$(BLUE)[INFO]$(NC) Starting home environment (live APIs)...\n"
	@printf "$(BLUE)[INFO]$(NC) Project: $(PROJECT_NAME)\n"
	@echo "$(PROJECT_NAME)" > $(PROJECT_FILE)
	@printf "$(BLUE)[INFO]$(NC) Ensuring fresh backend build by removing cached image...\n"
	@docker rmi -f watchgameupdates:latest 2>/dev/null || true
	@printf "$(BLUE)[INFO]$(NC) Building latest code (fail-fast on errors)...\n"
	@if ! docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_HOME) build --progress=plain 2>&1 | tee /tmp/build.log; then \
		printf "$(RED)[ERROR]$(NC) Build failed! Check output above for details.\n"; \
		rm -f $(PROJECT_FILE); \
		exit 1; \
	fi
	@printf "$(GREEN)[SUCCESS]$(NC) Build completed\n"
	@printf "$(BLUE)[INFO]$(NC) Starting containers...\n"
	@docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_HOME) --profile home up -d
	@printf "$(GREEN)[SUCCESS]$(NC) Home environment started\n"
	@printf "\n$(BLUE)[INFO]$(NC) Services available at:\n"
	@printf "  • Backend: http://localhost:8080\n"
	@printf "  • Cloud Tasks Emulator: http://localhost:8123\n"
	@printf "\n$(BLUE)[INFO]$(NC) Container names:\n"
	@docker compose -p $(PROJECT_NAME) ps --format "table {{.Name}}\t{{.Status}}"
	@printf "\n$(BLUE)[INFO]$(NC) View logs with: make logs\n"
	@printf "$(BLUE)[INFO]$(NC) Stop with: make stop\n"
	@printf "$(BLUE)[INFO]$(NC) List all containers: make list-containers\n"

test-containers: check-docker ## Start test containers and keep running (.env.local with mocks, use LOCAL_MOCK=true for local mock image)
	@printf "$(BLUE)[INFO]$(NC) Starting test containers (mock APIs)...\n"
	@if [ "$(LOCAL_MOCK)" = "true" ]; then \
		printf "$(BLUE)[INFO]$(NC) LOCAL_MOCK=true - Using locally built mock API image\n"; \
		if ! docker images -q mockdataapi:latest >/dev/null 2>&1; then \
			printf "$(RED)[ERROR]$(NC) Local mock image 'mockdataapi:latest' not found!\n"; \
			printf "$(RED)[ERROR]$(NC) Build your local mock API with: docker build -t mockdataapi:latest /path/to/mockdataserver\n"; \
			exit 1; \
		fi; \
		printf "$(GREEN)[SUCCESS]$(NC) Found local mockdataapi:latest image\n"; \
	fi
	@printf "$(BLUE)[INFO]$(NC) Project: $(PROJECT_NAME)\n"
	@echo "$(PROJECT_NAME)" > $(PROJECT_FILE)
	@printf "$(BLUE)[INFO]$(NC) Ensuring fresh backend build by removing cached image...\n"
	@docker rmi -f watchgameupdates:latest 2>/dev/null || true
	@printf "$(BLUE)[INFO]$(NC) Building latest code (fail-fast on errors)...\n"
	@if [ "$(LOCAL_MOCK)" = "true" ]; then \
		COMPOSE_CMD="docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) -f $(COMPOSE_LOCAL_MOCK)"; \
	else \
		COMPOSE_CMD="docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_TEST)"; \
	fi; \
	if ! $$COMPOSE_CMD build --progress=plain 2>&1 | tee /tmp/build.log; then \
		printf "$(RED)[ERROR]$(NC) Build failed! Check output above for details.\n"; \
		rm -f $(PROJECT_FILE); \
		exit 1; \
	fi
	@printf "$(GREEN)[SUCCESS]$(NC) Build completed\n"
	@if [ "$(LOCAL_MOCK)" = "true" ]; then \
		COMPOSE_CMD="docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) -f $(COMPOSE_LOCAL_MOCK)"; \
		$$COMPOSE_CMD --profile test up -d; \
	elif ! docker images -q blnelson/firepowermockdataserver:latest >/dev/null 2>&1 && ! docker images -q mockdataapi:latest >/dev/null 2>&1; then \
		printf "$(YELLOW)[WARNING]$(NC) No mock images found.\n"; \
		printf "$(YELLOW)[WARNING]$(NC) Continuing without mockdataapi (will use live APIs if configured)...\n"; \
		docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) --profile home up -d; \
	else \
		docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) --profile test up -d; \
	fi
	@printf "$(GREEN)[SUCCESS]$(NC) Test containers started\n"
	@printf "\n$(BLUE)[INFO]$(NC) Services available at:\n"
	@printf "  • Backend: http://localhost:8080\n"
	@printf "  • Cloud Tasks Emulator: http://localhost:8123\n"
	@if [ "$(LOCAL_MOCK)" = "true" ] || docker images -q mockdataapi:latest >/dev/null 2>&1 || docker images -q blnelson/firepowermockdataserver:latest >/dev/null 2>&1; then \
		printf "  • Mock NHL API: http://localhost:8125\n"; \
		printf "  • Mock MoneyPuck API: http://localhost:8124\n"; \
		if [ "$(LOCAL_MOCK)" = "true" ]; then \
			printf "  • Using LOCAL mock image\n"; \
		fi; \
	fi
	@printf "\n$(BLUE)[INFO]$(NC) Container names:\n"
	@docker compose -p $(PROJECT_NAME) ps --format "table {{.Name}}\t{{.Status}}"
	@printf "\n$(BLUE)[INFO]$(NC) View logs with: make logs\n"
	@printf "$(BLUE)[INFO]$(NC) Stop with: make stop\n"

test: check-docker ## Run full automated integration test (use LOCAL_MOCK=true for local mock image)
	@printf "$(BLUE)🚀 Starting CrashTheCrease Backend Automated Test$(NC)\n"
	@printf "==================================================\n\n"
	@if [ "$(LOCAL_MOCK)" = "true" ]; then \
		printf "$(BLUE)[INFO]$(NC) LOCAL_MOCK=true - Using locally built mock API image\n"; \
		if ! docker images -q mockdataapi:latest >/dev/null 2>&1; then \
			printf "$(RED)[ERROR]$(NC) Local mock image 'mockdataapi:latest' not found!\n"; \
			printf "$(RED)[ERROR]$(NC) Build your local mock API with: docker build -t mockdataapi:latest /path/to/mockdataserver\n"; \
			exit 1; \
		fi; \
		printf "$(GREEN)[SUCCESS]$(NC) Found local mockdataapi:latest image\n"; \
	fi
	@printf "$(BLUE)[INFO]$(NC) Project: $(PROJECT_NAME)\n"
	@echo "$(PROJECT_NAME)" > $(PROJECT_FILE)
	@printf "$(BLUE)[INFO]$(NC) Ensuring fresh backend build by removing cached image...\n"
	@docker rmi -f watchgameupdates:latest 2>/dev/null || true
	@printf "$(BLUE)[INFO]$(NC) Building latest code (fail-fast on errors)...\n"
	@if [ "$(LOCAL_MOCK)" = "true" ]; then \
		COMPOSE_CMD="docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) -f $(COMPOSE_LOCAL_MOCK)"; \
	else \
		COMPOSE_CMD="docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_TEST)"; \
	fi; \
	if ! $$COMPOSE_CMD build --progress=plain 2>&1 | tee /tmp/build.log; then \
		printf "$(RED)[ERROR]$(NC) Build failed! Check output above for details.\n"; \
		rm -f $(PROJECT_FILE); \
		exit 1; \
	fi
	@printf "$(GREEN)[SUCCESS]$(NC) Build completed\n"
	@if [ "$(LOCAL_MOCK)" = "true" ]; then \
		COMPOSE_CMD="docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) -f $(COMPOSE_LOCAL_MOCK)"; \
		printf "$(BLUE)[INFO]$(NC) Starting test containers with local mock...\n"; \
		$$COMPOSE_CMD --profile test up -d; \
		printf "$(GREEN)[SUCCESS]$(NC) Test containers started\n"; \
	elif ! docker images -q blnelson/firepowermockdataserver:latest >/dev/null 2>&1 && ! docker images -q mockdataapi:latest >/dev/null 2>&1; then \
		printf "$(YELLOW)[WARNING]$(NC) No mock images found.\n"; \
		printf "$(YELLOW)[WARNING]$(NC) Running tests with live APIs instead of mocks...\n"; \
		docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) --profile home up -d; \
	else \
		printf "$(BLUE)[INFO]$(NC) Starting test containers...\n"; \
		docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) --profile test up -d; \
		printf "$(GREEN)[SUCCESS]$(NC) Test containers started\n"; \
	fi
	@printf "\n$(BLUE)[INFO]$(NC) Waiting for services to be ready...\n"
	@sleep 5
	@printf "$(BLUE)[INFO]$(NC) Initiating test sequence...\n"
	@if [ ! -f "localCloudTasksTest/localCloudTasksTest" ]; then \
		printf "$(BLUE)[INFO]$(NC) Building local cloud tasks test program...\n"; \
		cd localCloudTasksTest && go build -o localCloudTasksTest main.go; \
	fi
	@./localCloudTasksTest/localCloudTasksTest >/dev/null 2>&1 || true
	@printf "$(GREEN)[SUCCESS]$(NC) Test sequence initiated\n"
	@printf "\n$(BLUE)[INFO]$(NC) Monitoring backend logs for completion...\n"
	@printf "$(BLUE)[INFO]$(NC) Looking for: 'Last play type: game-end, Should reschedule: false'\n"
	@BACKEND_CONTAINER=$$(docker compose -p $(PROJECT_NAME) ps -q backend); \
	elapsed=0; \
	max_wait=900; \
	found=false; \
	while [ $$elapsed -lt $$max_wait ]; do \
		if docker logs $$BACKEND_CONTAINER 2>&1 | grep -q "Last play type: game-end, Should reschedule: false"; then \
			found=true; \
			break; \
		fi; \
		sleep 2; \
		elapsed=$$((elapsed + 2)); \
		if [ $$((elapsed % 30)) -eq 0 ] && [ $$elapsed -gt 0 ]; then \
			printf "$(BLUE)[INFO]$(NC) Still monitoring... ($${elapsed}s elapsed)\n"; \
		fi; \
	done; \
	if [ "$$found" = "true" ]; then \
		printf "$(GREEN)[SUCCESS]$(NC) Found completion signal in logs!\n"; \
		printf "$(GREEN)🎉 Automated test completed successfully!$(NC)\n\n"; \
		printf "📋 What was tested:\n"; \
		printf "  ✓ Backend container built and started\n"; \
		if [ "$(LOCAL_MOCK)" = "true" ] || docker images -q mockdataapi:latest >/dev/null 2>&1 || docker images -q blnelson/firepowermockdataserver:latest >/dev/null 2>&1; then \
			printf "  ✓ Mock APIs provided test data\n"; \
			if [ "$(LOCAL_MOCK)" = "true" ]; then \
				printf "  ✓ Used locally built mock API image\n"; \
			fi; \
		else \
			printf "  ✓ Backend used live NHL and MoneyPuck APIs\n"; \
		fi; \
		printf "  ✓ Cloud tasks emulator handled task scheduling\n"; \
		printf "  ✓ Test sequence completed successfully\n"; \
		printf "  ✓ Backend processed all game events\n\n"; \
		printf "🔍 Test containers stopped but preserved for inspection:\n"; \
		printf "  • View logs: make logs-history PROJECT=$(PROJECT_NAME)\n"; \
		printf "  • Clean up with: make clean\n\n"; \
		if [ "$(LOCAL_MOCK)" = "true" ]; then \
			docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) -f $(COMPOSE_LOCAL_MOCK) stop backend mockdataapi 2>/dev/null || docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) stop backend 2>/dev/null || true; \
		else \
			docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) stop backend mockdataapi 2>/dev/null || docker compose -p $(PROJECT_NAME) -f $(COMPOSE_FILE) stop backend 2>/dev/null || true; \
		fi; \
	else \
		printf "$(RED)[ERROR]$(NC) Test timed out or failed after $${elapsed}s\n"; \
		printf "$(BLUE)[INFO]$(NC) Check logs with: make logs\n"; \
		exit 1; \
	fi

##@ Container Management

status: ## Show status of current containers
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		printf "$(BLUE)[INFO]$(NC) Current project: $$PROJECT\n"; \
		docker compose -p $$PROJECT ps; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) No current project. Start containers with 'make home' or 'make test-containers'\n"; \
	fi

logs: ## View logs from current containers
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		printf "$(BLUE)[INFO]$(NC) Showing logs for project: $$PROJECT\n"; \
		docker compose -p $$PROJECT logs -f; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) No current project. Start containers with 'make home' or 'make test-containers'\n"; \
	fi

logs-backend: ## View logs from current backend container only
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		BACKEND_CONTAINER=$$(docker compose -p $$PROJECT ps -q backend 2>/dev/null); \
		if [ -n "$$BACKEND_CONTAINER" ]; then \
			docker logs -f $$BACKEND_CONTAINER 2>&1; \
		else \
			printf "$(RED)[ERROR]$(NC) Backend container not found for project $$PROJECT\n"; \
		fi; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) No current project. Start containers with 'make home' or 'make test-containers'\n"; \
	fi

logs-cloudtasks: ## View logs from cloud tasks emulator only
	@EMULATOR_CONTAINER=$$(docker ps --filter "name=cloudtasks-emulator" -q | head -1); \
	if [ -n "$$EMULATOR_CONTAINER" ]; then \
		docker logs -f $$EMULATOR_CONTAINER 2>&1; \
	else \
		printf "$(RED)[ERROR]$(NC) Cloud tasks emulator not running\n"; \
	fi

logs-history: ## View logs from a specific historical project (use PROJECT=backend-YYYYMMDD-HHMMSS)
	@if [ -z "$(PROJECT)" ]; then \
		printf "$(RED)[ERROR]$(NC) Please specify PROJECT. Example: make logs-history PROJECT=backend-20251119-143022\n"; \
		printf "$(BLUE)[INFO]$(NC) Available projects:\n"; \
		docker ps -a --filter "label=com.docker.compose.project" --format "{{.Label \"com.docker.compose.project\"}}" | grep "^backend-" | sort -u; \
		exit 1; \
	fi
	@printf "$(BLUE)[INFO]$(NC) Showing logs for project: $(PROJECT)\n"
	@BACKEND_CONTAINER=$$(docker ps -a --filter "label=com.docker.compose.project=$(PROJECT)" --filter "label=com.docker.compose.service=backend" --format "{{.ID}}" | head -1); \
	if [ -n "$$BACKEND_CONTAINER" ]; then \
		docker logs $$BACKEND_CONTAINER 2>&1; \
	else \
		printf "$(RED)[ERROR]$(NC) No backend container found for project $(PROJECT)\n"; \
	fi

list-containers: ## List all backend containers (current and historical)
	@printf "$(BLUE)[INFO]$(NC) All backend containers:\n"
	@docker ps -a --filter "label=com.docker.compose.service=backend" --format "table {{.Label \"com.docker.compose.project\"}}\t{{.Names}}\t{{.Status}}\t{{.CreatedAt}}" | head -20

stop: ## Stop current containers (preserves for log access)
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		printf "$(BLUE)[INFO]$(NC) Stopping containers for project: $$PROJECT\n"; \
		docker compose -p $$PROJECT stop; \
		printf "$(GREEN)[SUCCESS]$(NC) Containers stopped (preserved for log access)\n"; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) No current project to stop\n"; \
	fi

restart: ## Restart current containers
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		printf "$(BLUE)[INFO]$(NC) Restarting containers for project: $$PROJECT\n"; \
		docker compose -p $$PROJECT restart; \
		printf "$(GREEN)[SUCCESS]$(NC) Containers restarted\n"; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) No current project to restart\n"; \
	fi

clean: ## Stop and remove current containers
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		printf "$(BLUE)[INFO]$(NC) Cleaning up project: $$PROJECT\n"; \
		docker compose -p $$PROJECT -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) -f $(COMPOSE_HOME) -f $(COMPOSE_LOCAL_MOCK) down -v 2>/dev/null || true; \
		rm -f $(PROJECT_FILE); \
		printf "$(GREEN)[SUCCESS]$(NC) Current project cleaned up\n"; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) No current project to clean\n"; \
	fi

clean-history: ## Remove old historical containers (keeps last 5)
	@printf "$(BLUE)[INFO]$(NC) Cleaning old backend containers (keeping last 5)...\n"
	@PROJECTS=$$(docker ps -a --filter "label=com.docker.compose.service=backend" --format "{{.Label \"com.docker.compose.project\"}}" | grep "^backend-" | sort -u | head -n -5); \
	if [ -n "$$PROJECTS" ]; then \
		for project in $$PROJECTS; do \
			printf "$(BLUE)[INFO]$(NC) Removing project: $$project\n"; \
			docker compose -p $$project down -v 2>/dev/null || true; \
		done; \
		printf "$(GREEN)[SUCCESS]$(NC) Old containers cleaned up\n"; \
	else \
		printf "$(BLUE)[INFO]$(NC) No old containers to clean (5 or fewer exist)\n"; \
	fi

clean-all: ## Remove ALL containers, networks, and images
	@printf "$(YELLOW)[WARNING]$(NC) This will remove ALL backend containers and the cloud tasks emulator\n"
	@printf "Continue? [y/N] " && read ans && [ $${ans:-N} = y ]
	@printf "$(BLUE)[INFO]$(NC) Removing all backend projects...\n"
	@PROJECTS=$$(docker ps -a --filter "label=com.docker.compose.service=backend" --format "{{.Label \"com.docker.compose.project\"}}" | grep "^backend-" | sort -u); \
	for project in $$PROJECTS; do \
		printf "$(BLUE)[INFO]$(NC) Removing project: $$project\n"; \
		docker compose -p $$project down -v 2>/dev/null || true; \
	done
	@docker stop cloudtasks-emulator 2>/dev/null || true
	@docker rm cloudtasks-emulator 2>/dev/null || true
	@docker rmi watchgameupdates:latest mockdataapi:latest 2>/dev/null || true
	@rm -f $(PROJECT_FILE)
	@printf "$(GREEN)[SUCCESS]$(NC) All containers, networks, and images removed\n"

##@ Building

build: check-go ## Build all Go binaries using build.go
	@printf "$(BLUE)[INFO]$(NC) Building all Go binaries...\n"
	@go run build.go -all
	@printf "$(GREEN)[SUCCESS]$(NC) All binaries built in ./bin/\n"

build-backend: check-go ## Build backend binary only
	@printf "$(BLUE)[INFO]$(NC) Building backend binary...\n"
	@go run build.go -target watchgameupdates
	@printf "$(GREEN)[SUCCESS]$(NC) Backend binary built: ./bin/watchgameupdates\n"

build-enqueue: check-go ## Build the Redis queue enqueue CLI tool
	@printf "$(BLUE)[INFO]$(NC) Building enqueue CLI tool...\n"
	@go run build.go -target enqueue
	@printf "$(GREEN)[SUCCESS]$(NC) Enqueue CLI built: ./bin/enqueue\n"

##@ Redis Queue (Worker Mode)

redis-up: check-docker ## Start Redis worker environment (Redis + Asynqmon + backend in worker mode)
	@printf "$(BLUE)[INFO]$(NC) Starting Redis worker environment...\n"
	@printf "$(BLUE)[INFO]$(NC) Project: $(PROJECT_NAME)\n"
	@echo "$(PROJECT_NAME)" > $(PROJECT_FILE)
	@printf "$(BLUE)[INFO]$(NC) Building backend in worker mode...\n"
	@if ! docker compose -p $(PROJECT_NAME) -f $(COMPOSE_REDIS) build --progress=plain 2>&1 | tee /tmp/build.log; then \
		printf "$(RED)[ERROR]$(NC) Build failed! Check output above for details.\n"; \
		rm -f $(PROJECT_FILE); \
		exit 1; \
	fi
	@printf "$(GREEN)[SUCCESS]$(NC) Build completed\n"
	@printf "$(BLUE)[INFO]$(NC) Starting containers...\n"
	@docker compose -p $(PROJECT_NAME) -f $(COMPOSE_REDIS) --profile home up -d
	@printf "$(GREEN)[SUCCESS]$(NC) Redis worker environment started\n"
	@printf "\n$(BLUE)[INFO]$(NC) Services available at:\n"
	@printf "  • Asynqmon Dashboard: http://localhost:8980\n"
	@printf "  • Redis: localhost:6379\n"
	@printf "\n$(BLUE)[INFO]$(NC) Enqueue a task with:\n"
	@printf "  ./bin/enqueue --game=2024030411\n"
	@printf "\n$(BLUE)[INFO]$(NC) Container names:\n"
	@docker compose -p $(PROJECT_NAME) -f $(COMPOSE_REDIS) ps --format "table {{.Name}}\t{{.Status}}"
	@printf "\n$(BLUE)[INFO]$(NC) View logs with: make redis-logs\n"
	@printf "$(BLUE)[INFO]$(NC) Stop with: make redis-stop\n"

redis-test: check-docker ## Start Redis worker environment with mock APIs for testing
	@printf "$(BLUE)[INFO]$(NC) Starting Redis test environment (mock APIs + worker mode)...\n"
	@printf "$(BLUE)[INFO]$(NC) Project: $(PROJECT_NAME)\n"
	@echo "$(PROJECT_NAME)" > $(PROJECT_FILE)
	@if ! docker compose -p $(PROJECT_NAME) -f $(COMPOSE_REDIS) build --progress=plain 2>&1 | tee /tmp/build.log; then \
		printf "$(RED)[ERROR]$(NC) Build failed! Check output above for details.\n"; \
		rm -f $(PROJECT_FILE); \
		exit 1; \
	fi
	@printf "$(GREEN)[SUCCESS]$(NC) Build completed\n"
	@docker compose -p $(PROJECT_NAME) -f $(COMPOSE_REDIS) --profile test up -d
	@printf "$(GREEN)[SUCCESS]$(NC) Redis test environment started\n"
	@printf "\n$(BLUE)[INFO]$(NC) Services available at:\n"
	@printf "  • Asynqmon Dashboard: http://localhost:8980\n"
	@printf "  • Redis: localhost:6379\n"
	@printf "  • Mock NHL API: http://localhost:8125\n"
	@printf "  • Mock MoneyPuck API: http://localhost:8124\n"

redis-stop: ## Stop Redis worker environment
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		printf "$(BLUE)[INFO]$(NC) Stopping Redis containers for project: $$PROJECT\n"; \
		docker compose -p $$PROJECT -f $(COMPOSE_REDIS) stop; \
		printf "$(GREEN)[SUCCESS]$(NC) Redis containers stopped\n"; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) No current project to stop\n"; \
	fi

redis-clean: ## Stop and remove Redis worker containers and volumes
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		printf "$(BLUE)[INFO]$(NC) Cleaning up Redis project: $$PROJECT\n"; \
		docker compose -p $$PROJECT -f $(COMPOSE_REDIS) down -v 2>/dev/null || true; \
		rm -f $(PROJECT_FILE); \
		printf "$(GREEN)[SUCCESS]$(NC) Redis project cleaned up\n"; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) No current project to clean\n"; \
	fi

redis-logs: ## View logs from Redis worker environment
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		printf "$(BLUE)[INFO]$(NC) Showing logs for Redis project: $$PROJECT\n"; \
		docker compose -p $$PROJECT -f $(COMPOSE_REDIS) logs -f; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) No current project. Start with 'make redis-up'\n"; \
	fi

redis-status: ## Show status of Redis worker containers
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		printf "$(BLUE)[INFO]$(NC) Redis project: $$PROJECT\n"; \
		docker compose -p $$PROJECT -f $(COMPOSE_REDIS) ps; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) No current project. Start with 'make redis-up'\n"; \
	fi

##@ Utilities

port-check: ## Check if required ports are available
	@printf "$(BLUE)[INFO]$(NC) Checking port availability...\n"
	@for port in 8080 8123 8124 8125; do \
		if lsof -i :$$port >/dev/null 2>&1; then \
			printf "$(YELLOW)[WARNING]$(NC) Port $$port is in use:\n"; \
			lsof -i :$$port; \
		else \
			printf "$(GREEN)[OK]$(NC) Port $$port is available\n"; \
		fi; \
	done

shell-backend: ## Open a shell in the current backend container
	@if [ -f $(PROJECT_FILE) ]; then \
		PROJECT=$$(cat $(PROJECT_FILE)); \
		BACKEND_CONTAINER=$$(docker compose -p $$PROJECT ps -q backend 2>/dev/null); \
		if [ -n "$$BACKEND_CONTAINER" ]; then \
			docker exec -it $$BACKEND_CONTAINER /bin/sh; \
		else \
			printf "$(RED)[ERROR]$(NC) Backend container not running\n"; \
		fi; \
	else \
		printf "$(RED)[ERROR]$(NC) No current project. Start containers with 'make home' or 'make test-containers'\n"; \
	fi

shell-cloudtasks: ## Open a shell in the cloud tasks emulator container
	@docker exec -it cloudtasks-emulator /bin/sh 2>/dev/null || printf "$(RED)[ERROR]$(NC) Cloud tasks emulator not running\n"

##@ Troubleshooting

doctor: check-deps port-check ## Run all diagnostic checks
	@printf "\n$(BLUE)[INFO]$(NC) Running diagnostic checks...\n"
	@printf "\n$(GREEN)[SUCCESS]$(NC) All diagnostic checks completed\n"
	@printf "\nDocker images:\n"
	@docker images | grep -E 'REPOSITORY|watchgameupdates|mockdataapi|cloud-tasks-emulator' || true
	@printf "\nDocker containers:\n"
	@docker ps -a --filter "label=com.docker.compose.service=backend" --format "table {{.Label \"com.docker.compose.project\"}}\t{{.Names}}\t{{.Status}}" | head -10 || true
	@docker ps -a --filter "name=cloudtasks-emulator" --format "table {{.Names}}\t{{.Status}}" || true

clean-cache: ## Clean Go build cache and module cache
	@printf "$(BLUE)[INFO]$(NC) Cleaning Go caches...\n"
	@go clean -cache -modcache -testcache
	@printf "$(GREEN)[SUCCESS]$(NC) Go caches cleaned\n"
