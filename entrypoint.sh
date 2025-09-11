#!/bin/bash
set -e

PGDATA_DIR="/var/lib/postgresql/14/main"
CONFIG_DIR="/config/postgres-data"

# Check if the user-mounted config directory is empty. If so, initialize PG there.
if [ ! -d "$CONFIG_DIR" ] || [ -z "$(ls -A "$CONFIG_DIR")" ]; then
    echo "PostgreSQL data directory not found or empty. Initializing database..."
    # Ensure the default data dir exists for the init logic
    mkdir -p "$PGDATA_DIR"
    chown -R postgres:postgres /var/lib/postgresql
    
    # Initialize in the default location first
    su postgres -c "/usr/lib/postgresql/14/bin/initdb -D '$PGDATA_DIR' --username=postgres"
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$PGDATA_DIR' start"
    sleep 5
    su postgres -c "psql --command \"CREATE USER audiomuse WITH SUPERUSER PASSWORD 'audiomusepassword';\""
    su postgres -c "psql --command \"CREATE DATABASE audiomusedb OWNER audiomuse;\""
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$PGDATA_DIR' stop"

    # Move the initialized data to the config volume
    mkdir -p "$CONFIG_DIR"
    mv $PGDATA_DIR/* "$CONFIG_DIR/"
    rm -rf "$PGDATA_DIR" # remove the now-empty default dir
fi

# Symlink the config directory to where postgres expects it
ln -sfn "$CONFIG_DIR" "$PGDATA_DIR"
chown -R postgres:postgres "$CONFIG_DIR"

# Execute the CMD
exec "$@"

