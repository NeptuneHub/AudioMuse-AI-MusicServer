#!/bin/bash
set -e

# This script's ONLY job now is to perform the one-time database initialization.
# All service management and dependency waiting is now handled by supervisord
# and the new start-ai-core.sh wrapper script.

CONFIG_DIR="/config/postgres-data"
PG_CONF="$CONFIG_DIR/postgresql.conf"

# Ensure the PostgreSQL data directory exists and has the correct, strict permissions (0700).
mkdir -p "$CONFIG_DIR"
chown -R postgres:postgres "$CONFIG_DIR"
chmod 700 "$CONFIG_DIR"

if [ ! -f "$CONFIG_DIR/PG_VERSION" ]; then
    echo "PostgreSQL data directory not found. Initializing new database..."
    su postgres -c "/usr/lib/postgresql/14/bin/initdb --username=postgres --no-locale -D '$CONFIG_DIR'"
    # Explicitly configure PostgreSQL to listen on the loopback address for TCP/IP connections.
    echo "listen_addresses = '127.0.0.1'" >> "$PG_CONF"
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w start"
    su postgres -c "psql --command \"CREATE USER audiomuse WITH SUPERUSER PASSWORD 'audiomusepassword';\""
    su postgres -c "psql --command \"CREATE DATABASE audiomusedb OWNER audiomuse;\""
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w stop"
    echo "Database initialization complete."
fi

# The 'exec "$@"' command will run the CMD from the Dockerfile.
# In our case, this is supervisord, which will now take over and manage all services.
echo "Initialization complete. Handing over to supervisord..."
exec "$@"

