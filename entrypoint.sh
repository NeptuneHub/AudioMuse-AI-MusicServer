#!/bin/bash
set -e

echo "=== AudioMuse Music Server Container Starting ==="

# Ensure log directory exists (though we won't use it much)
mkdir -p /var/log/supervisor

echo "=== Starting Supervisor ==="
echo "Music Server Backend will be available on port 8080"
echo "Music Server Frontend will be available on port 3000"
echo "All logs will be displayed below:"
echo ""

# Start supervisor to manage both processes
exec /usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf