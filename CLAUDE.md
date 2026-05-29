# CLAUDE.md

This file provides guidance for AI assistants working in this repository.

## Project Overview

CrashTheCrease Backend is a Go-based service that tracks NHL game updates and sends Discord notifications and iOS Live Activity APNs push notifications using Google Cloud Tasks for scheduling.

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

**Prerequisites:** Go 1.23.3+, Podman 5.0+ and podman-compose, Make

```bash
make live      # Start with live NHL/MoneyPuck APIs (.env.home)
make emulator  # Start with mock game data emulator (.env.local)
make stop      # Stop all services
```

## Common Commands

### Building

```bash
go run build.go -target watchgameupdates
go run build.go -target localCloudTasksTest
# Binaries are output to ./bin/
```

### Testing

```bash
# Unit tests (run from watchgameupdates/)
go test ./...
go test -v -cover ./...
go test -v ./internal/services   # Specific package

# Integration tests via scheduler
make schedule-test                # Full system with mock APIs
make schedule                     # Full system with live NHL data
make schedule-team TEAM=TOR       # Single team against running emulator
```

### Running Locally

```bash
make live              # Backend + tasks emulator, live APIs
make emulator          # Backend + tasks emulator + mock NHL/MoneyPuck APIs
make logs              # Follow container logs
make stop              # Stop all containers
make schedule          # Start with scheduler (live data)
make schedule-test     # Start with scheduler (mock data)
make schedule-team TEAM=TOR  # Run scheduler for one team
make watch TEAM=COL    # E2E live test: schedule today's game and follow logs
```

## Architecture

**Data flow:** Cloud Tasks → HTTP Handler → Fetch Game Data → Process Events → Discord + LiveActivity APNs Notifications → Reschedule or Complete

**Key packages in `watchgameupdates/internal/`:**

| Package | Responsibility |
|---|---|
| `handlers/` | HTTP request handling |
| `services/` | Business logic (fetcher, play-by-play, rescheduler) |
| `tasks/` | Google Cloud Tasks integration |
| `models/` | Data structures (Payload, Play, PlayByPlayResponse) |
| `notification/` | Discord and LiveActivity (APNs broadcast push) notifiers |
| `notification/liveactivity/` | iOS Live Activity APNs push (JWT signing, formatter, channel map) |
| `config/` | Environment configuration |
| `logger/` | Shared `slog.Logger` factory (JSON/text output, log level from env) |

**Service ports:**

| Service | Port |
|---|---|
| Backend | 8080 |
| Cloud Tasks Emulator | 8123 |
| Mock MoneyPuck API | 8124 |
| Mock NHL API | 8125 |
| Grafana Alloy UI (when using grafana/loki targets) | 12345 |
| Local Loki API (when using loki-* targets) | 3100 |
| Local Grafana UI (when using loki-* targets) | 3000 |

## Environment Variables

Copy `.env.example` to `.env.home` or `.env.local` and populate:

```
APP_ENV=development
CLOUD_TASKS_EMULATOR_HOST=        # local dev only
GCP_PROJECT_ID=
GCP_LOCATION=
PLAYBYPLAY_API_BASE_URL=          # NHL API or mock
STATS_API_BASE_URL=               # MoneyPuck API or mock
DISCORD_BOT_TOKEN=
DISCORD_CHANNEL_ID=          # Discord channel to post game updates
MESSAGE_INTERVAL_SECONDS=60       # active-play polling interval (default 60)
PERIOD_END_INTERVAL_SECONDS=1200  # post-period-end wait before next poll (default 1200)

# Logging (consumed by the Go app via internal/logger)
LOG_FORMAT=json              # "json" (default) or "text" for human-readable output
LOG_LEVEL=info               # "debug", "info" (default), "warn", "error"

# Live Activity APNs (optional — set LIVEACTIVITY_PUSH_ENABLED=true to enable)
LIVEACTIVITY_PUSH_ENABLED=
APNS_TEAM_ID=                # 10-char Apple Developer Team ID
APNS_KEY_ID=                 # .p8 key ID from App Store Connect
APNS_AUTH_KEY=               # base64-encoded .p8 file contents
APNS_TOPIC=                  # bundle ID, e.g. me.blakenelson.firepower
APNS_HOST=                   # api.sandbox.push.apple.com or api.push.apple.com

# Grafana Cloud Loki (consumed by Grafana Alloy sidecar, not the Go app)
# See docs/grafana-loki-setup.md for setup instructions.
GRAFANA_LOKI_URL=            # e.g. https://logs-prod-006.grafana.net/loki/api/v1/push
GRAFANA_LOKI_USER=           # Grafana Cloud numeric instance ID
GRAFANA_LOKI_API_KEY=        # API key with "Logs Writer" role
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

Use the affected area: `handlers`, `services`, `tasks`, `notification`, `models`, `config`, `docker`, `podman`, `makefile`

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
