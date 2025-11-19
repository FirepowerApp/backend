# CrashTheCrease Backend Makefile
# Replaces functionality from setup-local.sh and run_automated_test.sh
#
# Main targets:
#   make setup          - Initial setup (pull images, install deps)
#   make dev            - Start development environment (live APIs)
#   make test           - Run automated integration tests (with mocks)
#   make test-containers - Start test containers and keep running
#   make logs           - View logs from all containers
#   make clean          - Stop and remove all containers
#   make build          - Build all Go binaries
#   make pull           - Pull all required Docker images (with retry)

.PHONY: help setup dev test test-containers clean logs build pull check-deps check-docker check-go pull-with-retry status stop restart

# Color output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m # No Color

# Docker compose files
COMPOSE_FILE := docker-compose.yml
COMPOSE_TEST := docker-compose.test.yml
COMPOSE_DEV := docker-compose.dev.yml

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
	@printf "  - Run 'make dev' to start development environment (live APIs)\n"
	@printf "  - Run 'make test' to run automated tests (with mocks)\n"
	@printf "  - Run 'make test-containers' to start test environment and keep running\n"

dev: check-docker ## Start development environment with live APIs (.env.home)
	@printf "$(BLUE)[INFO]$(NC) Starting development environment (live APIs)...\n"
	@docker compose -f $(COMPOSE_FILE) -f $(COMPOSE_DEV) --profile dev up --build -d
	@printf "$(GREEN)[SUCCESS]$(NC) Development environment started\n"
	@printf "\n$(BLUE)[INFO]$(NC) Services available at:\n"
	@printf "  • Backend: http://localhost:8080\n"
	@printf "  • Cloud Tasks Emulator: http://localhost:8123\n"
	@printf "\n$(BLUE)[INFO]$(NC) View logs with: make logs\n"
	@printf "$(BLUE)[INFO]$(NC) Stop with: make stop\n"

test-containers: check-docker ## Start test containers and keep running (.env.local with mocks)
	@printf "$(BLUE)[INFO]$(NC) Starting test containers (mock APIs)...\n"
	@if ! docker images -q mockdataapi:latest >/dev/null 2>&1; then \
		printf "$(YELLOW)[WARNING]$(NC) mockdataapi:latest image not found.\n"; \
		printf "$(YELLOW)[WARNING]$(NC) The testserver was removed in commit 606aa80.\n"; \
		printf "$(YELLOW)[WARNING]$(NC) To use test mode, you need to rebuild the testserver.\n"; \
		printf "$(YELLOW)[WARNING]$(NC) Continuing without mockdataapi (will use live APIs if configured)...\n"; \
		docker compose -f $(COMPOSE_FILE) --profile dev up --build -d; \
	else \
		docker compose -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) --profile test up --build -d; \
	fi
	@printf "$(GREEN)[SUCCESS]$(NC) Test containers started\n"
	@printf "\n$(BLUE)[INFO]$(NC) Services available at:\n"
	@printf "  • Backend: http://localhost:8080\n"
	@printf "  • Cloud Tasks Emulator: http://localhost:8123\n"
	@if docker images -q mockdataapi:latest >/dev/null 2>&1; then \
		printf "  • Mock NHL API: http://localhost:8125\n"; \
		printf "  • Mock MoneyPuck API: http://localhost:8124\n"; \
	fi
	@printf "\n$(BLUE)[INFO]$(NC) View logs with: make logs\n"
	@printf "$(BLUE)[INFO]$(NC) Stop with: make stop\n"

test: check-docker ## Run full automated integration test
	@printf "$(BLUE)🚀 Starting CrashTheCrease Backend Automated Test$(NC)\n"
	@printf "==================================================\n\n"
	@if ! docker images -q mockdataapi:latest >/dev/null 2>&1; then \
		printf "$(YELLOW)[WARNING]$(NC) mockdataapi:latest image not found.\n"; \
		printf "$(YELLOW)[WARNING]$(NC) Running tests with live APIs instead of mocks...\n"; \
		$(MAKE) test-containers; \
	else \
		printf "$(BLUE)[INFO]$(NC) Starting test containers...\n"; \
		docker compose -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) --profile test up --build -d; \
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
	@elapsed=0; \
	max_wait=900; \
	found=false; \
	while [ $$elapsed -lt $$max_wait ]; do \
		if docker logs watchgameupdates 2>&1 | grep -q "Last play type: game-end, Should reschedule: false"; then \
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
		if docker images -q mockdataapi:latest >/dev/null 2>&1; then \
			printf "  ✓ Mock APIs provided test data\n"; \
		else \
			printf "  ✓ Backend used live NHL and MoneyPuck APIs\n"; \
		fi; \
		printf "  ✓ Cloud tasks emulator handled task scheduling\n"; \
		printf "  ✓ Test sequence completed successfully\n"; \
		printf "  ✓ Backend processed all game events\n\n"; \
		printf "🔍 Test containers stopped but preserved for inspection:\n"; \
		printf "  • Backend logs: docker logs watchgameupdates\n"; \
		printf "  • Clean up with: make clean\n\n"; \
		docker compose -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) stop backend mockdataapi 2>/dev/null || docker compose -f $(COMPOSE_FILE) stop backend 2>/dev/null || true; \
	else \
		printf "$(RED)[ERROR]$(NC) Test timed out or failed after $${elapsed}s\n"; \
		printf "$(BLUE)[INFO]$(NC) Check logs with: make logs\n"; \
		exit 1; \
	fi

##@ Container Management

status: ## Show status of all containers
	@printf "$(BLUE)[INFO]$(NC) Container status:\n"
	@docker compose -f $(COMPOSE_FILE) ps

logs: ## View logs from all running containers
	@docker compose -f $(COMPOSE_FILE) logs -f

logs-backend: ## View logs from backend container only
	@docker logs -f watchgameupdates 2>&1

logs-cloudtasks: ## View logs from cloud tasks emulator only
	@docker logs -f cloudtasks-emulator 2>&1

stop: ## Stop all containers (preserves containers for faster restart)
	@printf "$(BLUE)[INFO]$(NC) Stopping containers...\n"
	@docker compose -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) stop 2>/dev/null || docker compose -f $(COMPOSE_FILE) stop
	@printf "$(GREEN)[SUCCESS]$(NC) Containers stopped\n"

restart: stop ## Restart all containers
	@printf "$(BLUE)[INFO]$(NC) Restarting containers...\n"
	@docker compose -f $(COMPOSE_FILE) start
	@printf "$(GREEN)[SUCCESS]$(NC) Containers restarted\n"

clean: ## Stop and remove all containers and networks
	@printf "$(BLUE)[INFO]$(NC) Cleaning up containers and networks...\n"
	@docker compose -f $(COMPOSE_FILE) -f $(COMPOSE_TEST) -f $(COMPOSE_DEV) down -v 2>/dev/null || true
	@docker stop cloudtasks-emulator watchgameupdates mockdataapi-testserver-1 2>/dev/null || true
	@docker rm cloudtasks-emulator watchgameupdates mockdataapi-testserver-1 2>/dev/null || true
	@printf "$(GREEN)[SUCCESS]$(NC) Cleanup completed\n"

clean-all: clean ## Stop and remove all containers, networks, and images
	@printf "$(BLUE)[INFO]$(NC) Removing built images...\n"
	@docker rmi watchgameupdates:latest mockdataapi:latest 2>/dev/null || true
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

build-testserver: check-go ## Build testserver binary (if testserver exists)
	@if [ -d "testserver" ]; then \
		printf "$(BLUE)[INFO]$(NC) Building testserver binary...\n"; \
		go run build.go -target testserver; \
		printf "$(GREEN)[SUCCESS]$(NC) Testserver binary built: ./bin/testserver\n"; \
	else \
		printf "$(YELLOW)[WARNING]$(NC) testserver directory not found (removed in commit 606aa80)\n"; \
		printf "$(YELLOW)[WARNING]$(NC) Skipping testserver build\n"; \
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

shell-backend: ## Open a shell in the backend container
	@docker exec -it watchgameupdates /bin/sh 2>/dev/null || printf "$(RED)[ERROR]$(NC) Backend container not running. Start it with 'make dev' or 'make test-containers'\n"

shell-cloudtasks: ## Open a shell in the cloud tasks emulator container
	@docker exec -it cloudtasks-emulator /bin/sh 2>/dev/null || printf "$(RED)[ERROR]$(NC) Cloud tasks emulator not running. Start it with 'make dev' or 'make test-containers'\n"

##@ Troubleshooting

doctor: check-deps port-check ## Run all diagnostic checks
	@printf "\n$(BLUE)[INFO]$(NC) Running diagnostic checks...\n"
	@printf "\n$(GREEN)[SUCCESS]$(NC) All diagnostic checks completed\n"
	@printf "\nDocker images:\n"
	@docker images | grep -E 'REPOSITORY|watchgameupdates|mockdataapi|cloud-tasks-emulator' || true
	@printf "\nDocker containers:\n"
	@docker ps -a | grep -E 'CONTAINER|watchgameupdates|mockdataapi|cloudtasks' || true

clean-cache: ## Clean Go build cache and module cache
	@printf "$(BLUE)[INFO]$(NC) Cleaning Go caches...\n"
	@go clean -cache -modcache -testcache
	@printf "$(GREEN)[SUCCESS]$(NC) Go caches cleaned\n"
