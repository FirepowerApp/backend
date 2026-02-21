# CrashTheCrease Backend Makefile
#
# Targets:
#   make live      - Start handler and tasks queue with live game data APIs (.env.home)
#   make emulator  - Pull game data emulator from registry and start all components (.env.local)
#   make stop      - Stop all running containers
#   make logs      - Follow logs from running containers

.PHONY: help live emulator stop logs

BLUE  := \033[0;34m
GREEN := \033[0;32m
NC    := \033[0m

.DEFAULT_GOAL := help

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(BLUE)%-10s$(NC) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

live: ## Start handler and tasks queue with live game data APIs (.env.home)
	@docker compose -f docker-compose.yml -f docker-compose.live.yml up --build -d
	@printf "$(GREEN)[OK]$(NC) Started\n"
	@printf "  Backend:        http://localhost:8080\n"
	@printf "  Tasks emulator: http://localhost:8123\n"
	@printf "View logs: make logs  |  Stop: make stop\n"

emulator: ## Pull game data emulator from registry and start all components (.env.local)
	@printf "$(BLUE)[INFO]$(NC) Pulling game data emulator...\n"
	@docker pull blnelson/firepowermockdataserver:latest
	@docker compose -f docker-compose.yml -f docker-compose.emulator.yml up --build -d
	@printf "$(GREEN)[OK]$(NC) Started\n"
	@printf "  Backend:            http://localhost:8080\n"
	@printf "  Tasks emulator:     http://localhost:8123\n"
	@printf "  Game data emulator: http://localhost:8125 (NHL), http://localhost:8124 (MoneyPuck)\n"
	@printf "View logs: make logs  |  Stop: make stop\n"

stop: ## Stop all running containers
	@docker compose -f docker-compose.yml -f docker-compose.live.yml -f docker-compose.emulator.yml down 2>/dev/null || true
	@printf "$(GREEN)[OK]$(NC) Containers stopped\n"

logs: ## Follow logs from running containers
	@docker compose logs -f
