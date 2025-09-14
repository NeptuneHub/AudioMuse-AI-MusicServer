#!/bin/bash
set -e

CONFIG_DIR="/config/postgres-data"

# --- 1. ALWAYS ensure correct ownership and permissions ---
echo "Ensuring correct permissions for PostgreSQL data directory..."
mkdir -p "$CONFIG_DIR"
chown -R postgres:postgres "$CONFIG_DIR"
chmod 700 "$CONFIG_DIR"

# --- 2. Perform one-time DB initialization if needed ---
if [ ! -f "$CONFIG_DIR/PG_VERSION" ]; then
    echo "PostgreSQL data directory not found. Initializing new database..."
    su postgres -c "/usr/lib/postgresql/14/bin/initdb --username=postgres --no-locale -D '$CONFIG_DIR'"
    
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w start"
    
    su postgres -c "psql --command \"CREATE USER audiomuse WITH SUPERUSER PASSWORD 'audiomusepassword';\""
    su postgres -c "psql --command \"CREATE DATABASE audiomusedb OWNER audiomuse;\""
    
    echo "Seeding initial configuration for music server..."
    su postgres -c "psql audiomusedb --command \"CREATE TABLE IF NOT EXISTS configuration (key TEXT PRIMARY KEY, value TEXT);\""
    su postgres -c "psql audiomusedb --command \"INSERT INTO configuration (key, value) VALUES ('audiomuse_ai_core_url', 'http://localhost:8000') ON CONFLICT (key) DO NOTHING;\""

    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w stop"
    echo "Database initialization complete."
fi

# --- 3. Start Supervisord to manage background services ---
echo "Starting background services (Postgres, Redis, Music Server)..."
/usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf &

# --- 4. Wait for all background services to be ready ---
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

# --- 5. Get API Token with Robust Retry Logic ---
echo "All services are up. Requesting API token..."
API_TOKEN=""
# Loop until the API endpoint returns a 200 OK AND we get a valid token.
until [ -n "$API_TOKEN" ] && [ "$API_TOKEN" != "null" ]; do
    echo "Attempting to fetch API token..."
    
    # First, check if the endpoint is even ready by looking for a 200 status code.
    HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/rest/getApiKey.view?u=admin&p=admin&f=json")
    
    if [ "$HTTP_STATUS" -eq 200 ]; then
        # If the endpoint is ready, then try to get and parse the token.
        API_TOKEN=$(curl -s "http://localhost:8080/rest/getApiKey.view?u=admin&p=admin&f=json" | jq -r '."subsonic-response".apiKey.key // ""') || true
    else
        echo "API endpoint returned status $HTTP_STATUS. Waiting for it to become ready..."
        API_TOKEN="" # Ensure token is empty to force a retry
    fi
    
    if [ -z "$API_TOKEN" ] || [ "$API_TOKEN" == "null" ]; then
        echo "Failed to get a valid API token, retrying in 5 seconds..."
        sleep 5
    fi
done
echo "Successfully retrieved API token."

# --- 6. Start the AI Core (Flask App and RQ Workers) ---
echo "Starting AudioMuse-AI Core services..."

export NAVIDROME_PASSWORD="$API_TOKEN"
export NAVIDROME_USER="admin"
export MEDIASERVER_TYPE="navidrome"
export NAVIDROME_URL="http://localhost:8080"
export SERVICE_TYPE="flask"
export POSTGRES_HOST="127.0.0.1"

cd /app/audiomuse-core

echo "Starting RQ workers..."
rq worker -u redis://12.0.0.1:6379/0 &
rq worker -u redis://127.0.0.1:6379/0 &

echo "Starting Flask server..."
exec python3 app.py

