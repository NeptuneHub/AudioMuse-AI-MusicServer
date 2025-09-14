#!/bin/bash
set -e

# Fix PostgreSQL permissions
mkdir -p /config/postgres-data
chown -R postgres:postgres /config/postgres-data
chmod 700 /config/postgres-data

echo "Starting services with supervisord..."
/usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf &

# Wait for music server to be ready
echo "Waiting for music server to start..."
until curl -s http://localhost:8080/rest/ping.view?u=admin\&p=admin\&f=json > /dev/null 2>&1; do
    sleep 2
done
echo "Music server is ready."

# Get API token
echo "Getting API token..."
API_TOKEN=$(curl -s "http://localhost:8080/rest/getApiKey.view?u=admin&p=admin&f=json" | jq -r '.subsonic_response.apiKey // empty')

if [ -z "$API_TOKEN" ] || [ "$API_TOKEN" = "null" ]; then
    echo "Failed to get API token, using default credentials"
    export NAVIDROME_PASSWORD="admin"
else
    echo "Got API token: $API_TOKEN"
    export NAVIDROME_PASSWORD="$API_TOKEN"
fi

# Set environment variables for AudioMuse-AI Core
export NAVIDROME_USER="admin"
export MEDIASERVER_TYPE="navidrome"
export NAVIDROME_URL="http://localhost:8080"
export SERVICE_TYPE="flask"
export POSTGRES_HOST="127.0.0.1"

echo "Starting AudioMuse-AI Core..."
cd /app/audiomuse-core

# Start RQ workers in background
rq worker -u redis://127.0.0.1:6379/0 &
rq worker -u redis://127.0.0.1:6379/0 &

# Start Flask server
exec python3 app.py
