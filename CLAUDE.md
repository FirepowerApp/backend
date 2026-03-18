# CLAUDE.md

This file provides guidance for AI assistants working in this repository.

## Project Overview

CrashTheCrease Backend is a Go-based service that tracks NHL game updates and sends Discord notifications using Google Cloud Tasks for scheduling.

## Design Principles

All changes to this codebase should follow the [Twelve-Factor App](https://12factor.net/) methodology:

| Factor | Guideline |
|---|---|
| **Codebase** | One codebase tracked in Git, many deploys |
| **Dependencies** | Explicitly declare and isolate dependencies via `go.mod` |
| **Config** | Store config in environment variables, never in code |
| **Backing services** | Treat external services (Discord, NHL API, Cloud Tasks) as attached resources |
| **Build, release, run** | Strictly separate build and run stages |
| **Processes** | Execute the app as stateless processes |
| **Port binding** | Export services via port binding |
| **Concurrency** | Scale out via the process model |
| **Disposability** | Maximize robustness with fast startup and graceful shutdown |
| **Dev/prod parity** | Keep development, staging, and production as similar as possible |
| **Logs** | Treat logs as event streams (stdout) |
| **Admin processes** | Run admin/management tasks as one-off processes |

When making changes, ensure:
- Configuration comes from environment variables, not hardcoded values
- Services are stateless and can be restarted without data loss
- External dependencies are injected and swappable (e.g., mock APIs for testing)
- Logs go to stdout/stderr with no local file dependencies

## Development Setup

**Prerequisites:** Go 1.23.3+, Docker, Make

```bash
make setup   # One-time setup: check deps, pull images, download Go modules
make home    # Start with live NHL/MoneyPuck APIs
make stop    # Stop all services
```

## Common Commands

### Building

```bash
make build            # Build all binaries
make build-backend    # Build backend only
```

### Testing

```bash
# Unit tests (run from watchgameupdates/)
go test ./...
go test -v -cover ./...
go test -v ./internal/services   # Specific package

# Integration tests
make test                        # Full suite with mock APIs
make test LOCAL_MOCK=true        # Use locally-built mock API image
```

### Running Locally

```bash
make home              # Start with live APIs (development)
make test-containers   # Start with mock APIs (testing)
make logs              # View container logs
make clean             # Remove containers
```

### Diagnostics

```bash
make doctor       # Run diagnostics
make port-check   # Check port availability
make check-deps   # Verify Go and Docker
```

## Architecture

**Data flow:** Cloud Tasks → HTTP Handler → Fetch Game Data → Process Events → Discord Notifications → Reschedule or Complete

**Key packages in `watchgameupdates/internal/`:**

| Package | Responsibility |
|---|---|
| `handlers/` | HTTP request handling |
| `services/` | Business logic (fetcher, play-by-play, rescheduler) |
| `tasks/` | Google Cloud Tasks integration |
| `models/` | Data structures (Payload, Play, PlayByPlayResponse) |
| `notification/` | Discord webhook notifications |
| `config/` | Environment configuration |

**Service ports:**

| Service | Port |
|---|---|
| Backend | 8080 |
| Cloud Tasks Emulator | 8123 |
| Mock MoneyPuck API | 8124 |
| Mock NHL API | 8125 |

## Environment Variables

Copy `.env.example` to `.env.home` or `.env.local` and populate:

```
APP_ENV=development
CLOUD_TASKS_EMULATOR_HOST=   # local dev only
GCP_PROJECT_ID=
GCP_LOCATION=
PLAYBYPLAY_API_BASE_URL=     # NHL API or mock
STATS_API_BASE_URL=          # MoneyPuck API or mock
DISCORD_BOT_TOKEN=
```

## Commit Guidelines

All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/) specification.

### Format

```
<type>(<scope>): <short summary>

[optional body]

[optional footer(s)]
```

### Types

| Type | When to use |
|---|---|
| `feat` | A new feature |
| `fix` | A bug fix |
| `docs` | Documentation changes only |
| `style` | Formatting, missing semicolons, etc. (no logic change) |
| `refactor` | Code change that is neither a fix nor a feature |
| `test` | Adding or correcting tests |
| `chore` | Build process, dependency updates, tooling |
| `perf` | Performance improvements |
| `ci` | CI/CD configuration changes |

### Scopes (optional but encouraged)

Use the affected area: `handlers`, `services`, `tasks`, `notification`, `models`, `config`, `docker`, `makefile`

### Examples

```
feat(notification): add goal scorer name to Discord message

fix(services): handle nil response from NHL API gracefully

chore(docker): update base image to golang 1.24

docs: add architecture diagram to README

test(handlers): add unit tests for WatchGameUpdatesHandler
```

### Rules

- Use the imperative mood in the summary ("add" not "added", "fix" not "fixed")
- Do not capitalize the first letter of the summary
- Do not end the summary with a period
- Keep the summary under 72 characters
- Breaking changes must include `BREAKING CHANGE:` in the footer or a `!` after the type/scope (e.g., `feat!:`)
