#!/bin/bash
set -e

CONFIG_DIR="/config/postgres-data"

# A robust way to check if the database is initialized is to look for a key file.
if [ ! -f "$CONFIG_DIR/PG_VERSION" ]; then
    echo "PostgreSQL data directory not found. Initializing new database..."
    
    # Ensure the target directory exists and is owned by the postgres user.
    mkdir -p "$CONFIG_DIR"
    chown -R postgres:postgres "$CONFIG_DIR"
    
    # Run initdb as the postgres user, creating the database directly in our persistent volume.
    # The --no-locale flag avoids potential locale errors inside the container.
    su postgres -c "/usr/lib/postgresql/14/bin/initdb --username=postgres --no-locale -D '$CONFIG_DIR'"
    
    # Temporarily start the server in the background to configure it.
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w start"
    
    # Create the application user and database.
    su postgres -c "psql --command \"CREATE USER audiomuse WITH SUPERUSER PASSWORD 'audiomusepassword';\""
    su postgres -c "psql --command \"CREATE DATABASE audiomusedb OWNER audiomuse;\""
    
    # Stop the temporary server. Supervisor will manage it from now on.
    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D '$CONFIG_DIR' -w stop"
    
    echo "Database initialization complete."
fi

# Execute the main command passed to the container (CMD in Dockerfile), which is supervisord.
exec "$@"

