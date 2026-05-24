# CrashTheCrease Backend Makefile
#
# Targets:
#   make live          - Start handler and tasks queue with live game data APIs (.env.home)
#   make emulator      - Pull game data emulator from registry and start all components (.env.local)
#   make stop          - Stop all running containers
#   make logs          - Follow logs from running containers
#   make schedule      - Start full system and run scheduler with live NHL data
#   make schedule-test - Start full system and run scheduler with test data
#   make schedule-team TEAM=TRI [DATE=YYYY-MM-DD] - Run scheduler for a single team (DATE overrides UTC default)
#   make redis-up            - Start Redis worker environment (Redis + Asynqmon + backend in worker mode)
#   make redis-test          - Start Redis worker environment with mock APIs for testing
#   make redis-schedule      - Start Redis environment and run scheduler with live NHL data
#   make redis-schedule-test - Start Redis environment and run scheduler with mock data
#   make redis-schedule-team TEAM=TOR [DATE=YYYY-MM-DD] - Run Redis scheduler for a single team
#   make redis-stop          - Stop Redis worker environment
#   make redis-logs          - Follow logs from the Redis worker environment
#   make build-enqueue       - Build the Redis queue enqueue CLI tool

TEAM ?= COL

.PHONY: help live emulator stop logs schedule schedule-test watch schedule-team redis-up redis-test redis-schedule redis-schedule-test redis-schedule-team redis-stop redis-logs build-enqueue

BLUE  := \033[0;34m
GREEN := \033[0;32m
NC    := \033[0m

COMPOSE_REDIS := docker-compose.redis.yml

.DEFAULT_GOAL := help

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(BLUE)%-15s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

live: ## Start handler and tasks queue with live game data APIs (.env.home)
	@podman-compose -f docker-compose.yml -f docker-compose.live.yml up --build -d
	@printf "$(GREEN)[OK]$(NC) Started\n"
	@printf "  Backend:        http://localhost:8080\n"
	@printf "  Tasks emulator: http://localhost:8123\n"
	@printf "View logs: make logs  |  Stop: make stop\n"

emulator: ## Pull game data emulator from registry and start all components (.env.local)
	@printf "$(BLUE)[INFO]$(NC) Pulling game data emulator...\n"
	@podman pull docker.io/blnelson/firepowermockdataserver:latest
	@podman-compose -f docker-compose.yml -f docker-compose.emulator.yml up --build -d
	@printf "$(GREEN)[OK]$(NC) Started\n"
	@printf "  Backend:            http://localhost:8080\n"
	@printf "  Tasks emulator:     http://localhost:8123\n"
	@printf "  Game data emulator: http://localhost:8125 (NHL), http://localhost:8124 (MoneyPuck)\n"
	@printf "View logs: make logs  |  Stop: make stop\n"

stop: ## Stop all running containers
	@podman-compose -f docker-compose.yml -f docker-compose.live.yml -f docker-compose.emulator.yml -f docker-compose.watch.yml down 2>/dev/null || true
	@podman-compose -f $(COMPOSE_REDIS) down 2>/dev/null || true
	@printf "$(GREEN)[OK]$(NC) Containers stopped\n"

logs: ## Follow logs from running containers
	@podman-compose -f docker-compose.yml logs --follow backend & \
	 podman-compose -f docker-compose.yml logs --follow cloudtasks-emulator & \
	 wait

schedule: ## Start full system and run scheduler with live NHL data
	@printf "$(BLUE)[INFO]$(NC) Starting full system with scheduler (live data)...\n"
	@podman-compose -f docker-compose.yml -f docker-compose.live.yml --profile scheduler up --build -d
	@printf "$(GREEN)[OK]$(NC) Started with scheduler\n"
	@printf "  Backend:        http://localhost:8080\n"
	@printf "  Tasks emulator: http://localhost:8123\n"
	@printf "View logs: make logs  |  Stop: make stop\n"

watch: ## E2E live test for a team: schedules today's real game and follows logs (e.g. make watch TEAM=COL)
	@printf "$(BLUE)[INFO]$(NC) Starting backend + Cloud Tasks emulator...\n"
	@podman-compose -f docker-compose.yml -f docker-compose.watch.yml up --build -d
	@printf "$(BLUE)[INFO]$(NC) Waiting for services...\n"
	@until nc -z localhost 8080 2>/dev/null; do sleep 1; done
	@printf "$(BLUE)[INFO]$(NC) Scheduling live game for team: $(TEAM)\n"
	@TEAM_FILTER=$(TEAM) podman-compose \
	  -f docker-compose.yml \
	  -f docker-compose.watch.yml \
	  --profile scheduler \
	  run --rm scheduler
	@printf "$(GREEN)[OK]$(NC) Game scheduled — following logs (look for 'APNs push: channel='):\n"
	@printf "$(BLUE)[TIP]$(NC) Stop with: make stop\n"
	@podman-compose -f docker-compose.yml -f docker-compose.watch.yml logs --follow backend

schedule-team: ## Run scheduler for one team (usage: make schedule-team TEAM=TOR [DATE=2026-05-21])
	@if [ -z "$(TEAM)" ]; then printf "Error: TEAM is required. Usage: make schedule-team TEAM=TOR\n"; exit 1; fi
	@printf "$(BLUE)[INFO]$(NC) Running scheduler for team $(TEAM)...\n"
	@podman-compose -f docker-compose.yml -f docker-compose.live.yml run --rm \
	  -e TEAM_FILTER=$(TEAM) \
	  -e INCLUDE_LIVE_GAMES=true \
	  $(if $(DATE),-e SCHEDULE_DATE=$(DATE),) \
	  scheduler
	@printf "$(GREEN)[OK]$(NC) Scheduler finished for $(TEAM)\n"

schedule-test: ## Start full system and run scheduler with test data
	@printf "$(BLUE)[INFO]$(NC) Pulling game data emulator...\n"
	@podman pull docker.io/blnelson/firepowermockdataserver:latest
	@printf "$(BLUE)[INFO]$(NC) Starting full system with scheduler (test data)...\n"
	@podman-compose -f docker-compose.yml -f docker-compose.emulator.yml --profile scheduler up --build -d
	@printf "$(GREEN)[OK]$(NC) Started with scheduler (test data)\n"
	@printf "  Backend:            http://localhost:8080\n"
	@printf "  Tasks emulator:     http://localhost:8123\n"
	@printf "  Game data emulator: http://localhost:8125 (NHL), http://localhost:8124 (MoneyPuck)\n"
	@printf "View logs: make logs  |  Stop: make stop\n"

##@ Redis Queue (Worker Mode)

redis-up: ## Start Redis worker environment (Redis + Asynqmon + backend in worker mode)
	@printf "$(BLUE)[INFO]$(NC) Starting Redis worker environment...\n"
	@podman-compose -f $(COMPOSE_REDIS) --profile home up --build -d
	@printf "$(GREEN)[OK]$(NC) Redis worker environment started\n"
	@printf "  Asynqmon dashboard: http://localhost:8980\n"
	@printf "  Redis:              localhost:6379\n"
	@printf "Enqueue a task with: ./bin/enqueue --game=2024030411\n"
	@printf "View logs: make redis-logs  |  Stop: make redis-stop\n"

redis-test: ## Start Redis worker environment with mock APIs for testing
	@printf "$(BLUE)[INFO]$(NC) Starting Redis test environment (mock APIs + worker mode)...\n"
	@podman-compose -f $(COMPOSE_REDIS) --profile test up --build -d
	@printf "$(GREEN)[OK]$(NC) Redis test environment started\n"
	@printf "  Asynqmon dashboard:  http://localhost:8980\n"
	@printf "  Redis:               localhost:6379\n"
	@printf "  Mock NHL API:        http://localhost:8125\n"
	@printf "  Mock MoneyPuck API:  http://localhost:8124\n"

redis-schedule: ## Start Redis worker environment and run scheduler with live NHL data
	@printf "$(BLUE)[INFO]$(NC) Starting Redis environment with scheduler (live data)...\n"
	@podman-compose -f $(COMPOSE_REDIS) --profile scheduler up --build -d
	@printf "$(GREEN)[OK]$(NC) Started with scheduler\n"
	@printf "  Asynqmon dashboard: http://localhost:8980\n"
	@printf "  Redis:              localhost:6379\n"
	@printf "View logs: make redis-logs  |  Stop: make redis-stop\n"

redis-schedule-test: ## Start Redis worker environment and run scheduler with mock data
	@printf "$(BLUE)[INFO]$(NC) Pulling game data emulator...\n"
	@podman pull docker.io/blnelson/firepowermockdataserver:latest
	@printf "$(BLUE)[INFO]$(NC) Starting Redis environment with scheduler (mock data)...\n"
	@podman-compose -f $(COMPOSE_REDIS) --profile scheduler-test up --build -d
	@printf "$(GREEN)[OK]$(NC) Started with scheduler (mock data)\n"
	@printf "  Asynqmon dashboard:  http://localhost:8980\n"
	@printf "  Redis:               localhost:6379\n"
	@printf "  Mock NHL API:        http://localhost:8125\n"
	@printf "  Mock MoneyPuck API:  http://localhost:8124\n"
	@printf "View logs: make redis-logs  |  Stop: make redis-stop\n"

redis-schedule-team: ## Run Redis scheduler for one team (usage: make redis-schedule-team TEAM=TOR [DATE=2026-05-21])
	@if [ -z "$(TEAM)" ]; then printf "Error: TEAM is required. Usage: make redis-schedule-team TEAM=TOR\n"; exit 1; fi
	@printf "$(BLUE)[INFO]$(NC) Running Redis scheduler for team $(TEAM)...\n"
	@podman-compose -f $(COMPOSE_REDIS) run --rm \
	  -e TEAM_FILTER=$(TEAM) \
	  -e INCLUDE_LIVE_GAMES=true \
	  $(if $(DATE),-e SCHEDULE_DATE=$(DATE),) \
	  scheduler
	@printf "$(GREEN)[OK]$(NC) Redis scheduler finished for $(TEAM)\n"

redis-stop: ## Stop Redis worker environment
	@podman-compose -f $(COMPOSE_REDIS) down 2>/dev/null || true
	@printf "$(GREEN)[OK]$(NC) Redis containers stopped\n"

redis-logs: ## Follow logs from the Redis worker environment
	@podman-compose -f $(COMPOSE_REDIS) logs -f

build-enqueue: ## Build the Redis queue enqueue CLI tool
	@printf "$(BLUE)[INFO]$(NC) Building enqueue CLI tool...\n"
	@go run build.go -target enqueue
	@printf "$(GREEN)[OK]$(NC) Enqueue CLI built: ./bin/enqueue\n"
