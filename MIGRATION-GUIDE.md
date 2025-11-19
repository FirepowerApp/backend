# Migration Guide: Shell Scripts → Docker Compose + Makefile

This guide helps you transition from the shell scripts (`setup-local.sh` and `run_automated_test.sh`) to the new Docker Compose + Makefile system.

## 🎯 Why Migrate?

- **Less Code**: ~1000 lines of bash → ~150 lines of YAML
- **Industry Standard**: Docker Compose is the standard for multi-container local development
- **Better Error Handling**: Built-in retry logic and health checks
- **Easier Maintenance**: Declarative configuration vs procedural scripts
- **CI/CD Ready**: Same commands work locally and in CI

## 📋 Quick Reference

### Daily Commands

| Old Way | New Way | Notes |
|---------|---------|-------|
| `./setup-local.sh` | `make dev` | Starts dev environment |
| `./scripts/run_automated_test.sh` | `make test` | Runs full test suite |
| `./scripts/run_automated_test.sh --containers-only` | `make test-containers` | Starts and keeps running |
| `./scripts/run_automated_test.sh --env-home` | `make dev` | Uses live APIs |
| `docker logs watchgameupdates` | `make logs-backend` | View backend logs |
| `docker stop ...` | `make stop` | Stop containers |
| `docker rm ...` | `make clean` | Remove containers |

## 🚀 Step-by-Step Migration

### Step 1: One-Time Setup

```bash
# Check prerequisites
make check-deps

# Pull required images and install dependencies
make setup
```

This replaces the initial run of `./setup-local.sh`.

### Step 2: Choose Your Workflow

#### Workflow A: Development with Live APIs

**Old way:**
```bash
./scripts/run_automated_test.sh --containers-only --env-home
```

**New way:**
```bash
make dev
```

What it does:
- ✅ Starts Cloud Tasks Emulator
- ✅ Builds and starts backend
- ✅ Uses `.env.home` (live APIs)
- ✅ Keeps running until you stop it

#### Workflow B: Testing with Mock APIs

**Old way:**
```bash
./scripts/run_automated_test.sh --containers-only
```

**New way:**
```bash
make test-containers
```

What it does:
- ✅ Starts Cloud Tasks Emulator
- ✅ Starts Mock Data API (if available)
- ✅ Builds and starts backend
- ✅ Uses `.env.local` (mock APIs)
- ✅ Keeps running until you stop it

#### Workflow C: Full Automated Test

**Old way:**
```bash
./scripts/run_automated_test.sh
```

**New way:**
```bash
make test
```

What it does:
- ✅ Starts all containers
- ✅ Runs test sequence
- ✅ Monitors logs for completion
- ✅ Reports results
- ✅ Stops test containers (keeps emulator)

### Step 3: Verify Everything Works

```bash
# Run diagnostics
make doctor

# Check what's running
make status

# View logs
make logs
```

## 🔍 Understanding the Differences

### Health Checks

**Old way (manual polling):**
```bash
for i in {1..30}; do
    if curl -f -s http://localhost:8080 >/dev/null 2>&1; then
        break
    fi
    sleep 2
done
```

**New way (declarative):**
```yaml
healthcheck:
  test: ["CMD-SHELL", "wget --spider http://localhost:8080"]
  interval: 5s
  retries: 30
```

### Service Dependencies

**Old way (manual sequencing):**
```bash
start_cloud_tasks_emulator
wait_for_services
build_and_run_service
```

**New way (declarative):**
```yaml
backend:
  depends_on:
    cloudtasks-emulator:
      condition: service_healthy
```

### Retry Logic

**Old way (not implemented):**
- No retry on image pull failures

**New way (automatic):**
```bash
make pull  # Retries 4 times with exponential backoff
# 2s, 4s, 8s, 16s delays
```

### Container Persistence

**Both ways preserve the Cloud Tasks Emulator:**

**Old way:**
```bash
# setup-local.sh didn't remove cloud tasks on exit
# run_automated_test.sh explicitly preserved it
```

**New way:**
```bash
make stop   # Stops but preserves all containers
make clean  # Removes all containers
```

## 🆕 New Features

### 1. Integrated Help

```bash
make help
```

Shows all available commands with descriptions.

### 2. Better Diagnostics

```bash
make doctor
```

Runs comprehensive checks:
- Go version
- Docker status
- Port availability
- Image status
- Container status

### 3. Selective Operations

```bash
make logs-backend      # Just backend logs
make logs-cloudtasks   # Just emulator logs
make shell-backend     # Shell into backend
make port-check        # Check port conflicts
```

### 4. Multiple Environments

```bash
make dev               # Live APIs (.env.home)
make test-containers   # Mock APIs (.env.local)
make test              # Full test suite
```

### 5. Clean Separation

```bash
make stop      # Stop (preserve for quick restart)
make clean     # Remove containers (keep images)
make clean-all # Remove containers AND images
```

## ⚠️ Important Notes

### 1. Environment Files

The new system uses environment files in `watchgameupdates/`:

- `.env.local` - Test mode with mocks (created)
- `.env.home` - Dev mode with live APIs (created)
- `.env.example` - Template (existing)

**Action Required**: Review and update these files with your settings.

### 2. Mock Data API

The `mockdataapi-testserver-1` container was removed in commit 606aa80.

**Options:**
- Use dev mode: `make dev` (uses live APIs)
- Rebuild testserver from git history (if needed)

### 3. Container Names

Container names remain the same:
- `cloudtasks-emulator`
- `watchgameupdates`
- `mockdataapi-testserver-1`

This ensures compatibility with any external tools or scripts.

### 4. Ports

All ports remain the same:
- `8080` - Backend
- `8123` - Cloud Tasks Emulator
- `8124` - Mock MoneyPuck API
- `8125` - Mock NHL API

### 5. Docker Network

The network name remains `net` for compatibility.

## 🐛 Troubleshooting Migration

### "Port already in use"

**Cause**: Old containers still running

**Solution:**
```bash
# Stop old containers
docker stop cloudtasks-emulator watchgameupdates mockdataapi-testserver-1

# Or remove them
docker rm -f cloudtasks-emulator watchgameupdates mockdataapi-testserver-1

# Then start with new system
make dev
```

### "Image not found: mockdataapi"

**Cause**: Testserver was removed in commit 606aa80

**Solution:**
```bash
# Use dev mode instead (live APIs)
make dev
```

### "make: command not found"

**Cause**: Make not installed

**Solution:**
```bash
# macOS
brew install make

# Ubuntu/Debian
sudo apt-get install make

# Or use docker compose directly
docker compose --profile dev up
```

### Services won't start

**Diagnosis:**
```bash
# Run diagnostics
make doctor

# Check prerequisites
make check-deps

# Check ports
make port-check

# View logs
make logs
```

## 🔄 Transitioning Your Workflow

### Week 1: Parallel Testing

Run both systems side-by-side to compare:

```bash
# Old way
./setup-local.sh

# In another terminal, new way
make dev
```

Compare behavior and logs.

### Week 2: Primary Usage

Use Docker Compose as primary, keep scripts as backup:

```bash
# Primary
make dev

# Fallback if issues
./setup-local.sh
```

### Week 3+: Full Migration

Use only Docker Compose:

```bash
make dev    # Daily development
make test   # Testing
make clean  # Cleanup
```

## 📚 Learning the New System

### Day 1: Basic Commands

```bash
make help              # Learn available commands
make setup             # One-time setup
make dev               # Start development
make logs              # View logs
make stop              # Stop everything
```

### Day 2: Advanced Usage

```bash
make status            # Check container status
make port-check        # Verify ports
make shell-backend     # Interactive debugging
make doctor            # Diagnose issues
```

### Day 3: Customization

```bash
# Edit environment files
vim watchgameupdates/.env.local
vim watchgameupdates/.env.home

# Restart with new config
make restart
```

## 🎓 Additional Resources

- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Makefile Tutorial](https://makefiletutorial.com/)
- [DOCKER-COMPOSE.md](./DOCKER-COMPOSE.md) - Comprehensive documentation

## ✅ Migration Checklist

- [ ] Run `make check-deps` to verify prerequisites
- [ ] Run `make setup` to initialize
- [ ] Review `.env.local` and `.env.home` files
- [ ] Update Discord bot token (if needed)
- [ ] Test with `make dev`
- [ ] Test with `make test` (if mockdataapi available)
- [ ] Bookmark `make help` for reference
- [ ] Update any CI/CD scripts to use `make test`
- [ ] Archive old shell scripts (don't delete yet)

## 🚀 Ready to Go?

```bash
# Start here
make setup

# Then choose your mode
make dev               # Development with live APIs
# OR
make test-containers   # Testing with mocks

# View everything
make status
make logs

# Stop when done
make stop
```

## 📞 Getting Help

If you run into issues:

1. Check the troubleshooting section above
2. Run `make doctor` for diagnostics
3. Review logs with `make logs`
4. Consult [DOCKER-COMPOSE.md](./DOCKER-COMPOSE.md)
5. Compare with old scripts as reference

Remember: The old scripts are still there as a safety net during migration!
