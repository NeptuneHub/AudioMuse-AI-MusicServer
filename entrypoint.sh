#!/bin/bash
set -e

# --- PostgreSQL Initialization ---
CONFIG_DIR="/config/postgres-data"
PG_CONF="$CONFIG_DIR/postgresql.conf"

# Ensure the PostgreSQL data directory exists and has the correct, strict permissions (0700)
# before supervisord tries to start the postgres process. This is crucial when using mounted
# volumes, as they might be created with insecure default permissions.
mkdir -p "$CONFIG_DIR"
chown -R postgres:postgres "$CONFIG_DIR"
chmod 700 "$CONFIG_DIR"

if [ ! -f "$CONFIG_DIR/PG_VERSION" ]; then
    echo "PostgreSQL data directory not found. Initializing new database..."
    # The directory is already created with correct permissions, so we just need to initialize.
    su postgres -c "/usr/lib/postgresql/14/bin/initdb --username=postgres --no-locale -D '$CONFIG_DIR'"
    # Explicitly configure PostgreSQL to listen on the loopback address for TCP/IP connections.
    echo "listen_addresses = '127.0.0.1'" >> "$PG_CONF"
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w start"
    su postgres -c "psql --command \"CREATE USER audiomuse WITH SUPERUSER PASSWORD 'audiomusepassword';\""
    su postgres -c "psql --command \"CREATE DATABASE audiomusedb OWNER audiomuse;\""
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w stop"
    echo "Database initialization complete."
fi

# --- Service Management and API Token Fetching ---
# Start all services managed by supervisord (except the Python core) in the background
/usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf &

echo "Waiting for supervisord daemon to become available..."
until supervisorctl status > /dev/null 2>&1; do
    echo "Supervisord not ready yet - sleeping..."
    sleep 1
done
echo "Supervisord is ready."

echo "Waiting for PostgreSQL database to become available..."
# Use pg_isready to reliably check if PostgreSQL is ready to accept connections.
until su postgres -c "pg_isready -h 127.0.0.1 -p 5432" > /dev/null 2>&1; do
    echo "PostgreSQL is unavailable - sleeping..."
    sleep 2
done
echo "PostgreSQL is up and running."

# Now that the database is ready, start the Python Flask application
echo "Starting AudioMuse-AI Core service..."
supervisorctl start python-flask-core

echo "Waiting for AudioMuse-AI Core to become available..."
# We must wait for the Python service on port 8000 to be ready before other services try to use it.
until curl -s -f -o /dev/null "http://localhost:8000/"; do
    echo "AudioMuse-AI Core is unavailable - sleeping..."
    sleep 2
done
echo "AudioMuse-AI Core is up."

echo "Waiting for music server to become available..."
until curl -s -f -o /dev/null "http://localhost:8080/rest/ping.view"; do
    echo "Music server is unavailable - sleeping..."
    sleep 2
done
echo "Music server is up."

echo "Requesting API token for admin user..."
# The JSON key 'subsonic-response' contains a hyphen and must be quoted for jq.
API_TOKEN=$(curl -s "http://localhost:8080/rest/getApiKey.view?u=admin&p=admin&f=json" | jq -r '."subsonic-response".apiKey.key')

if [ -z "$API_TOKEN" ] || [ "$API_TOKEN" == "null" ]; then
    echo "ERROR: Failed to retrieve API token. AI core may not function correctly."
else
    echo "Successfully retrieved API token. Exporting variables."
    # These env vars are used by the python-flask-core process
    export NAVIDROME_PASSWORD="$API_TOKEN"
    export NAVIDROME_USER="admin"
    export MEDIASERVER_TYPE="navidrome"
    export NAVIDROME_URL="http://localhost:8080"

    echo "Restarting AudioMuse-AI Core service to apply API token..."
    # The program name in supervisord.conf is 'python-flask-core'
    supervisorctl restart python-flask-core
fi

# The 'wait' command is crucial. It tells the script to wait for all background
# processes (like supervisord) to exit. Since supervisord is configured to run
# forever (nodaemon=true), this keeps the container alive.
wait

