#!/bin/bash
set -e

# --- 1. Perform one-time DB initialization if needed ---
CONFIG_DIR="/config/postgres-data"
if [ ! -f "$CONFIG_DIR/PG_VERSION" ]; then
    echo "PostgreSQL data directory not found. Initializing new database..."
    mkdir -p "$CONFIG_DIR"
    chown -R postgres:postgres "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"
    su postgres -c "/usr/lib/postgresql/14/bin/initdb --username=postgres --no-locale -D '$CONFIG_DIR'"
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w start"
    su postgres -c "psql --command \"CREATE USER audiomuse WITH SUPERUSER PASSWORD 'audiomusepassword';\""
    su postgres -c "psql --command \"CREATE DATABASE audiomusedb OWNER audiomuse;\""
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w stop"
    echo "Database initialization complete."
fi

# --- 2. Start Supervisord to manage background services ---
echo "Starting background services (Postgres, Redis, Music Server)..."
/usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf &

# --- 3. Wait for all background services to be ready ---
echo "Waiting for background services to become available..."

# Wait for PostgreSQL
until su postgres -c "pg_isready -h 127.0.0.1 -p 5432" > /dev/null 2>&1; do
    echo "Waiting for PostgreSQL..."
    sleep 2
done
echo "PostgreSQL is up."

# Wait for Redis
until bash -c 'exec 3<> /dev/tcp/127.0.0.1/6379' > /dev/null 2>&1; do
    echo "Waiting for Redis..."
    sleep 2
done
echo "Redis is up."

# Wait for the Go Music Server API
until curl -s -f -o /dev/null "http://localhost:8080/rest/ping.view"; do
    echo "Waiting for Music Server..."
    sleep 2
done
echo "Music Server is up."

# --- 4. Get API Token with Retry Logic ---
echo "All services are up. Requesting API token..."
API_TOKEN=""
until [ -n "$API_TOKEN" ] && [ "$API_TOKEN" != "null" ]; do
    echo "Attempting to fetch API token..."
    # Note: Using port 8080 for the backend API server.
    API_TOKEN=$(curl -s "http://localhost:8080/rest/getApiKey.view?u=admin&p=admin&f=json" | jq -r '."subsonic-response".apiKey.key // ""') || true
    if [ -z "$API_TOKEN" ] || [ "$API_TOKEN" == "null" ]; then
        echo "Failed to get API token, retrying in 5 seconds..."
        sleep 5
    fi
done
echo "Successfully retrieved API token."

# --- 5. Start the AI Core (Flask App and RQ Workers) ---
echo "Starting AudioMuse-AI Core services..."

# Set environment variables for the following processes
export NAVIDROME_PASSWORD="$API_TOKEN"
export NAVIDROME_USER="admin"
export MEDIASERVER_TYPE="navidrome"
export NAVIDROME_URL="http://localhost:8080"
export SERVICE_TYPE="flask"
export POSTGRES_HOST="127.0.0.1"

# Change to the correct directory
cd /app/audiomuse-core

# Start RQ workers in the background
echo "Starting RQ workers..."
rq worker -u redis://127.0.0.1:6379/0 &
rq worker -u redis://127.0.0.1:6379/0 &

# Start the main Flask app in the foreground using exec
# 'exec' replaces the script with the python process, making it the main process
echo "Starting Flask server..."
exec python3 app.py

