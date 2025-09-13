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

# --- Start the actual Python application ---
echo "All dependencies are ready. Starting AudioMuse-AI Core..."

if [ -z "$API_TOKEN" ] || [ "$API_TOKEN" == "null" ]; then
    echo "ERROR: AI Core failed to retrieve API token. Starting without it."
    # Start the app without the API token variables
    exec python3 /app/audiomuse-core/app.py
else
    echo "AI Core successfully retrieved API token. Starting with token."
    # Use exec and the 'env' command to set environment variables
    # specifically for the python process. This is more reliable than export.
    exec env \
        NAVIDROME_PASSWORD="$API_TOKEN" \
        NAVIDROME_USER="admin" \
        MEDIASERVER_TYPE="navidrome" \
        NAVIDROME_URL="http://localhost:8080" \
        python3 /app/audiomuse-core/app.py
fi

