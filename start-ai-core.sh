#!/bin/bash
set -e

echo "AI Core Wrapper: Waiting for dependencies to become available..."

# --- Wait for PostgreSQL ---
until su postgres -c "pg_isready -h 127.0.0.1 -p 5432" > /dev/null 2>&1; do
    echo "AI Core is waiting for PostgreSQL..."
    sleep 2
done
echo "AI Core detects PostgreSQL is up."

# --- Wait for Redis ---
until bash -c 'exec 3<> /dev/tcp/127.0.0.1/6379' > /dev/null 2>&1; do
    echo "AI Core is waiting for Redis..."
    sleep 2
done
echo "AI Core detects Redis is up."

# --- Wait for the Go Music Server ---
until curl -s -f -o /dev/null "http://localhost:8080/rest/ping.view"; do
    echo "AI Core is waiting for Music Server..."
    sleep 2
done
echo "AI Core detects Music Server is up."

# --- Get API Token ---
echo "AI Core is requesting API token..."
API_TOKEN=$(curl -s "http://localhost:8080/rest/getApiKey.view?u=admin&p=admin&f=json" | jq -r '."subsonic-response".apiKey.key')

if [ -z "$API_TOKEN" ] || [ "$API_TOKEN" == "null" ]; then
    echo "ERROR: AI Core failed to retrieve API token. AI core may not function correctly."
    # We can exit here or continue without the token, depending on desired behavior.
    # For now, we'll continue so the service still runs for inspection.
else
    echo "AI Core successfully retrieved API token."
    export NAVIDROME_PASSWORD="$API_TOKEN"
    export NAVIDROME_USER="admin"
    export MEDIASERVER_TYPE="navidrome"
    export NAVIDROME_URL="http://localhost:8080"
fi

# --- Start the actual Python application ---
echo "All dependencies are ready. Starting AudioMuse-AI Core..."
# Use exec to replace the bash script with the python process,
# ensuring it becomes the main process managed by supervisord.
exec python3 /app/audiomuse-core/app.py
