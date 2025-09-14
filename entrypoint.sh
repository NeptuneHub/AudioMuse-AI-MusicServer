#!/bin/bash
set -e

# Fix PostgreSQL permissions
mkdir -p /config/postgres-data
chown -R postgres:postgres /config/postgres-data
chmod 700 /config/postgres-data

echo "Starting services with supervisord..."
/usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf &

# Wait a bit for services to start
sleep 5

# Set basic environment variables for AudioMuse-AI Core (without API token first)
export NAVIDROME_USER="admin"
export NAVIDROME_PASSWORD="admin"
export MEDIASERVER_TYPE="navidrome"
export NAVIDROME_URL="http://localhost:8080"
export SERVICE_TYPE="flask"
export POSTGRES_HOST="127.0.0.1"

echo "Starting AudioMuse-AI Core..."
cd /app/audiomuse-core

# Start RQ workers in background
rq worker -u redis://127.0.0.1:6379/0 &
rq worker -u redis://127.0.0.1:6379/0 &

# Start Flask server in background
python3 app.py &

# Wait for music server to be ready before trying to get API token
echo "Waiting for music server to start..."
until curl -s http://localhost:8080/rest/ping.view?u=admin\&p=admin\&f=json > /dev/null 2>&1; do
    sleep 2
done
echo "Music server is ready."

# Try to get API token and update environment if successful
echo "Attempting to get API token..."
API_TOKEN=$(curl -s "http://localhost:8080/rest/getApiKey.view?u=admin&p=admin&f=json" | jq -r '.subsonic_response.apiKey // empty' 2>/dev/null || echo "")

if [ -n "$API_TOKEN" ] && [ "$API_TOKEN" != "null" ] && [ "$API_TOKEN" != "empty" ]; then
    echo "Got API token: $API_TOKEN"
    # Update the environment for running processes (this won't affect already running Flask app)
    export NAVIDROME_PASSWORD="$API_TOKEN"
    echo "API token set successfully"
else
    echo "Using default admin credentials"
fi

# Keep the container running
wait