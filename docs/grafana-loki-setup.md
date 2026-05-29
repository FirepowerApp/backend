# Grafana Loki Log Shipping Setup

This document covers how to test Grafana Loki log shipping locally, and how to configure it for production with Grafana Cloud. The app writes structured JSON logs to stdout (12-factor compliant). Grafana Alloy collects those logs from Docker and ships them to Loki — the Go code itself is unaware of the destination.

---

## Part 1: Local Testing (No Grafana Cloud Account Required)

Use the local Loki stack to verify the full pipeline end-to-end before setting up any cloud infrastructure.

### What runs

| Service | URL | Purpose |
|---|---|---|
| Loki | http://localhost:3100 | Log storage |
| Grafana UI | http://localhost:3000 | Log explorer (auto-configured, no login) |
| Grafana Alloy | http://localhost:12345 | Log shipper pipeline UI |

The Alloy config at `alloy/config.loki-local.alloy` sends logs to `http://loki:3100` with no auth. The Grafana datasource is auto-provisioned from `grafana/provisioning/datasources/loki.yaml`.

### Steps

**1. Start the stack**

```bash
# With mock game data APIs (recommended for first test)
make loki-emulator

# Or with live NHL/MoneyPuck APIs
make loki-live
```

Both targets build the backend from source, pull the mock data image (emulator only), and start Loki, Grafana, and Alloy alongside the backend.

**2. Verify Alloy is collecting logs**

Open the Alloy pipeline UI at http://localhost:12345. You should see:
- `discovery.docker.containers` — lists running containers as targets
- `loki.source.docker.containers` — shows bytes received
- `loki.write.local` — shows bytes sent to Loki

If targets show `0`, wait 10–15 seconds for Alloy to discover containers.

**3. Query logs in Grafana**

1. Open http://localhost:3000
2. Click **Explore** (compass icon in the left sidebar)
3. Select the **Loki** data source (pre-configured)
4. In the label filter, select `service = backend`
5. Click **Run query**

You should see JSON log lines from the backend container.

**4. Query structured fields with LogQL**

Because the backend emits JSON, Loki can parse log fields on query:

```logql
# All backend logs
{service="backend"}

# Filter by log level
{service="backend"} | json | level="ERROR"

# Filter by game ID
{service="backend"} | json | game_id="2024030411"

# Scheduler logs only
{service="scheduler"}

# All services
{compose_project="backend"}
```

**5. Test with short intervals (faster log volume)**

```bash
# Set short poll interval to generate more log volume quickly
MESSAGE_INTERVAL_SECONDS=10 make loki-emulator
```

**6. Stop**

```bash
make stop
```

---

## Part 2: Production Setup (Grafana Cloud)

### Prerequisites

- A Grafana Cloud account (free tier available)
- The backend running with `make live` or in production

### Steps

**1. Create a Grafana Cloud account**

Sign up at https://grafana.com/auth/sign-up. A free account includes sufficient Loki storage for this use case.

**2. Find your Loki credentials**

1. Log in to Grafana Cloud at https://grafana.com/orgs/<your-org>
2. Click your stack name → **Details**
3. Scroll to the **Loki** section
4. Note the **URL** (e.g. `https://logs-prod-006.grafana.net`) and **User** (numeric instance ID)
5. Click **Generate now** under API Keys → create a key with **Logs Writer** role
6. Copy the API key — it will not be shown again

Alternatively, navigate to **Connections → Add new connection → Grafana Alloy** for guided instructions that include your credentials pre-filled.

**3. Set credentials as environment variables**

Add to your `.env.home` file (never commit credentials):

```bash
GRAFANA_LOKI_URL=https://logs-prod-006.grafana.net/loki/api/v1/push
GRAFANA_LOKI_USER=123456
GRAFANA_LOKI_API_KEY=glc_eyJ...
```

The Loki URL must end with `/loki/api/v1/push`.

**4. Start the stack with Alloy**

```bash
# Cloud Tasks mode
make grafana-live        # Live APIs + Alloy
make grafana-emulator    # Mock APIs + Alloy (for testing)

# Redis worker mode
make grafana-redis       # Live APIs + Alloy
make grafana-redis-test  # Mock APIs + Alloy (for testing)
```

**5. Verify Alloy is shipping**

Open the Alloy pipeline UI at http://localhost:12345. Check that `loki.write.destination` shows bytes being sent. If you see errors, verify the URL ends with `/loki/api/v1/push` and the API key has the correct role.

**6. Query logs in Grafana Cloud**

1. Open your Grafana Cloud instance (e.g. `https://your-org.grafana.net`)
2. Click **Explore** → select the **Loki** data source
3. Query:

```logql
# All backend logs in the last hour
{service="backend"}

# Errors only
{service="backend"} | json | level="ERROR"

# Specific game
{service="backend"} | json | game_id="2024030411"

# Notification sent events
{service="backend"} | json | msg="notification sent successfully"
```

### Security Notes

- The `GRAFANA_LOKI_API_KEY` only needs **Logs Writer** role — do not use Admin keys
- Never commit credentials to git; keep them in `.env.home` (already in `.gitignore`)
- Rotate the API key periodically via the Grafana Cloud Access Policies UI
- The Alloy container passes credentials via environment variables, not config files

### Makefile quick reference

| Command | What it does |
|---|---|
| `make loki-emulator` | Mock APIs + local Loki + Grafana UI (no cloud account needed) |
| `make loki-live` | Live APIs + local Loki + Grafana UI |
| `make grafana-emulator` | Mock APIs + Alloy → Grafana Cloud |
| `make grafana-live` | Live APIs + Alloy → Grafana Cloud |
| `make grafana-redis-test` | Redis + mock APIs + Alloy → Grafana Cloud |
| `make grafana-redis` | Redis + live APIs + Alloy → Grafana Cloud |
