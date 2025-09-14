#!/bin/bash
set -e

echo "=== AudioMuse Music Server Container Starting ==="

# Ensure log directory exists
mkdir -p /var/log/supervisor

# Set permissions
chown -R root:root /var/log/supervisor

echo "=== Starting Supervisor ==="
echo "Music Server Backend will be available on port 8080"
echo "Music Server Frontend will be available on port 3000"

# Start supervisor to manage both processes
exec /usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf