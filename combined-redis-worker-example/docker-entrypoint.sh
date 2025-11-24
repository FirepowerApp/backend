#!/bin/sh
set -e

# Start Redis in background
echo "Starting Redis..."
redis-server --daemonize yes --appendonly yes --bind 0.0.0.0

# Wait for Redis to be ready
echo "Waiting for Redis to start..."
until redis-cli ping 2>/dev/null; do
  sleep 1
done
echo "Redis is ready!"

# Start worker in foreground (keeps container alive)
echo "Starting Asynq worker..."
exec /usr/local/bin/worker
