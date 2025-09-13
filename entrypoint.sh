#!/bin/bash
set -e

# --- PostgreSQL Initialization ---
CONFIG_DIR="/config/postgres-data"
PG_CONF="$CONFIG_DIR/postgresql.conf"

# Ensure the PostgreSQL data directory exists and has the correct, strict permissions (0700)
mkdir -p "$CONFIG_DIR"
chown -R postgres:postgres "$CONFIG_DIR"
chmod 700 "$CONFIG_DIR"

if [ ! -f "$CONFIG_DIR/PG_VERSION" ]; then
    echo "PostgreSQL data directory not found. Initializing new database..."
    su postgres -c "/usr/lib/postgresql/14/bin/initdb --username=postgres --no-locale -D '$CONFIG_DIR'"
    echo "listen_addresses = '127.0.0.1'" >> "$PG_CONF"
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w start"
    su postgres -c "psql --command \"CREATE USER audiomuse WITH SUPERUSER PASSWORD 'audiomusepassword';\""
    su postgres -c "psql --command \"CREATE DATABASE audiomusedb OWNER audiomuse;\""
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w stop"
    echo "Database initialization complete."
fi

# --- Service Management ---
echo "Starting service manager..."
# Supervisord will autostart Postgres, Redis, the Go Music Server, and the React Frontend.
# The AI Core services are set to autostart=false and will be started by this script.
/usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf &

echo "Waiting for supervisord daemon to become available..."
until supervisorctl status > /dev/null 2>&1; do
    echo "Supervisord not ready yet - sleeping..."
    sleep 1
done
echo "Supervisord is ready."

# --- Wait for independent services to be ready IN PARALLEL ---
echo "Waiting for independent services (PostgreSQL, Redis, & Music Server) to start..."

# Wait for PostgreSQL in the background
(
    until su postgres -c "pg_isready -h 127.0.0.1 -p 5432" > /dev/null 2>&1; do
        echo "PostgreSQL is unavailable - sleeping..."
        sleep 2
    done
    echo "PostgreSQL is up and running."
) &
PG_WAIT_PID=$!

# Wait for Redis in the background
(
    until bash -c 'exec 3<> /dev/tcp/127.0.0.1/6379' > /dev/null 2>&1; do
        echo "Redis is unavailable - sleeping..."
        sleep 2
    done
    echo "Redis is up and running."
) &
REDIS_WAIT_PID=$!

# Wait for the Go Music Server in the background
(
    until curl -s -f -o /dev/null "http://localhost:8080/rest/ping.view"; do
        echo "Music server is unavailable - sleeping..."
        sleep 2
    done
    echo "Music server is up."
) &
MUSIC_SERVER_WAIT_PID=$!

# Wait for all three background wait-processes to complete
wait $PG_WAIT_PID
wait $REDIS_WAIT_PID
wait $MUSIC_SERVER_WAIT_PID
echo "All independent services are ready."

# --- Start the AI Core (which depends on the other three) ---
echo "Starting AudioMuse-AI Core services..."
supervisorctl start python-flask-core python-rq-worker

echo "Waiting for AudioMuse-AI Core to become available..."
until curl -s -f -o /dev/null "http://localhost:8000/"; do
    echo "AudioMuse-AI Core is unavailable - sleeping..."
    sleep 2
done
echo "AudioMuse-AI Core is up."

# --- Configure AI Core with API Token ---
echo "Requesting API token for admin user..."
API_TOKEN=$(curl -s "http://localhost:8080/rest/getApiKey.view?u=admin&p=admin&f=json" | jq -r '."subsonic-response".apiKey.key')

if [ -z "$API_TOKEN" ] || [ "$API_TOKEN" == "null" ]; then
    echo "ERROR: Failed to retrieve API token. AI core may not function correctly."
else
    echo "Successfully retrieved API token. Exporting variables."
    export NAVIDROME_PASSWORD="$API_TOKEN"
    export NAVIDROME_USER="admin"
    export MEDIASERVER_TYPE="navidrome"
    export NAVIDROME_URL="http://localhost:8080"

    echo "Restarting AudioMuse-AI Core service to apply API token..."
    supervisorctl restart python-flask-core
fi

echo "All services are running."
# The 'wait' command keeps the container alive by waiting for supervisord to exit.
wait

