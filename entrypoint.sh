#!/bin/bash
set -e

# --- PostgreSQL Initialization ---
CONFIG_DIR="/config/postgres-data"
if [ ! -f "$CONFIG_DIR/PG_VERSION" ]; then
    echo "PostgreSQL data directory not found. Initializing new database..."
    mkdir -p "$CONFIG_DIR"
    chown -R postgres:postgres "$CONFIG_DIR"
    su postgres -c "/usr/lib/postgresql/14/bin/initdb --username=postgres --no-locale -D '$CONFIG_DIR'"
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w start"
    su postgres -c "psql --command \"CREATE USER audiomuse WITH SUPERUSER PASSWORD 'audiomusepassword';\""
    su postgres -c "psql --command \"CREATE DATABASE audiomusedb OWNER audiomuse;\""
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w stop"
    echo "Database initialization complete."
fi

# --- Service Management and API Token Fetching ---
# Start all services managed by supervisord in the background
/usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf &

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

    echo "Restarting AudioMuse-AI Core service..."
    # The program name in supervisord.conf is 'python-flask-core'
    supervisorctl restart python-flask-core
fi

# The 'wait' command is crucial. It tells the script to wait for all background
# processes (like supervisord) to exit. Since supervisord is configured to run
# forever (nodaemon=true), this keeps the container alive.
wait
