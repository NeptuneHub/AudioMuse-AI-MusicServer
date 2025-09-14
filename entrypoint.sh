#!/bin/bash
set -e

echo "=== AudioMuse All-in-One Container Starting ==="

# Set database credentials
POSTGRES_PASSWORD="audiomusepassword"
POSTGRES_USER="audiomuse" 
POSTGRES_DB="audiomusedb"

# Function to check if PostgreSQL is ready
check_postgres() {
    pg_isready -h 127.0.0.1 -p 5432 >/dev/null 2>&1
}

# Function to check if a service is running
check_service() {
    local service_name=$1
    local process_pattern=$2
    pgrep -f "$process_pattern" >/dev/null 2>&1
}

# Function to set system environment variables permanently
set_system_env() {
    local var_name="$1"
    local var_value="$2"
    
    # Method 1: Add to /etc/environment (system-wide)
    if grep -q "^${var_name}=" /etc/environment 2>/dev/null; then
        sed -i "s/^${var_name}=.*/${var_name}=\"${var_value}\"/" /etc/environment
    else
        echo "${var_name}=\"${var_value}\"" >> /etc/environment
    fi
    
    # Method 2: Export in current process and all children
    export "${var_name}=${var_value}"
    
    # Method 3: Add to system profile
    echo "export ${var_name}=\"${var_value}\"" >> /etc/profile.d/audiomuse-system.sh
    
    echo "✓ Set system variable: ${var_name}=${var_value}"
}

echo "=== STEP 1: Setting up PostgreSQL ==="

# Initialize PostgreSQL data directory properly
echo "Setting up PostgreSQL..."
mkdir -p /config/postgres-data
chown -R postgres:postgres /config/postgres-data
chmod 700 /config/postgres-data

# Check if PostgreSQL data directory is initialized
if [ ! -f /config/postgres-data/PG_VERSION ]; then
    echo "Initializing PostgreSQL data directory..."
    su postgres -c "/usr/lib/postgresql/14/bin/initdb -D /config/postgres-data --auth-local=trust --auth-host=md5"
    
    # Configure PostgreSQL for password authentication
    cat > /config/postgres-data/pg_hba.conf << 'EOF'
# TYPE  DATABASE        USER            ADDRESS                 METHOD
# Allow local socket connections with trust for postgres user
local   all             postgres                                trust
local   all             all                                     trust
# TCP/IP connections require password
host    all             all             127.0.0.1/32            md5
host    all             all             ::1/128                 md5
# Allow external connections for debugging
host    all             all             0.0.0.0/0               md5
EOF

    cat >> /config/postgres-data/postgresql.conf << 'EOF'
listen_addresses = '*'
port = 5432
max_connections = 100
shared_buffers = 128MB
unix_socket_directories = '/var/run/postgresql'
logging_collector = on
log_directory = '/var/log/postgresql'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
log_statement = 'all'
EOF
    echo "✓ PostgreSQL initialized successfully"
else
    echo "✓ PostgreSQL data directory already initialized"
fi

# Ensure socket and log directories exist
mkdir -p /var/run/postgresql /var/log/postgresql
chown postgres:postgres /var/run/postgresql /var/log/postgresql

# Start PostgreSQL
echo "Starting PostgreSQL..."
su postgres -c "/usr/lib/postgresql/14/bin/postgres -D /config/postgres-data -p 5432" &
POSTGRES_PID=$!

# Wait for PostgreSQL to start
echo "Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if check_postgres; then
        echo "✓ PostgreSQL is ready after $i attempts"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "✗ PostgreSQL failed to start after 30 attempts"
        exit 1
    fi
    sleep 1
done

echo "=== STEP 2: Setting up Database User and Schema ==="

# Create AudioMuse user and database using LOCAL socket connections
echo "Creating user ${POSTGRES_USER}..."
su postgres -c "psql -c \"CREATE USER ${POSTGRES_USER} WITH PASSWORD '${POSTGRES_PASSWORD}';\"" 2>/dev/null || {
    echo "User already exists, updating password..."
    su postgres -c "psql -c \"ALTER USER ${POSTGRES_USER} WITH PASSWORD '${POSTGRES_PASSWORD}';\""
}

echo "Creating database ${POSTGRES_DB}..."
su postgres -c "psql -c \"CREATE DATABASE ${POSTGRES_DB} OWNER ${POSTGRES_USER};\"" 2>/dev/null || echo "Database already exists"

echo "Granting privileges..."
su postgres -c "psql -c \"GRANT ALL PRIVILEGES ON DATABASE ${POSTGRES_DB} TO ${POSTGRES_USER};\""

# Test the TCP connection with the new user
echo "Testing database connection via TCP..."
PGPASSWORD=${POSTGRES_PASSWORD} psql -h 127.0.0.1 -p 5432 -U ${POSTGRES_USER} -d ${POSTGRES_DB} -c "SELECT version();" || {
    echo "✗ Database connection test failed!"
    exit 1
}
echo "✓ Database connection test successful!"

echo "=== STEP 3: Starting Redis ==="
/usr/bin/redis-server --daemonize yes --loglevel warning
echo "✓ Redis started"

echo "=== STEP 4: Checking Frontend Code ==="
echo "Frontend directory contents:"
ls -la /app/audiomuse-server/music-server-frontend/
echo ""

echo "=== STEP 5: Starting Go Music Server ==="
cd /app/audiomuse-server
echo "Starting Music Server..."
./music-server &
MUSIC_SERVER_PID=$!
echo "✓ Music Server started with PID $MUSIC_SERVER_PID"

echo "=== STEP 6: Starting React Frontend ==="
cd /app/audiomuse-server/music-server-frontend
if [ ! -d "node_modules" ]; then
    echo "Installing npm dependencies..."
    npm install
fi
echo "Starting React development server..."
npm start &
FRONTEND_PID=$!
echo "✓ Frontend development server started with PID $FRONTEND_PID"

echo "=== STEP 7: Waiting for Services to be Ready ==="

# Wait for Music Server to be ready
echo "Waiting for Music Server to be ready..."
for i in {1..60}; do
    if curl -s http://localhost:8080/rest/ping.view?u=admin\&p=admin\&f=json >/dev/null 2>&1; then
        echo "✓ Music Server is ready after $i attempts"
        break
    fi
    if [ $i -eq 60 ]; then
        echo "⚠ Music Server not responding after 60 attempts, continuing anyway..."
    fi
    sleep 2
done

echo "=== STEP 8: Setting SYSTEM Environment Variables ==="

# Initialize system environment files
echo "# AudioMuse System Environment Variables" > /etc/environment
echo "#!/bin/bash" > /etc/profile.d/audiomuse-system.sh

# Set system variables - START WITH ADMIN PASSWORD
set_system_env "NAVIDROME_USER" "admin"
set_system_env "NAVIDROME_PASSWORD" "admin"
set_system_env "MEDIASERVER_TYPE" "navidrome"
set_system_env "NAVIDROME_URL" "http://localhost:8080"
set_system_env "SERVICE_TYPE" "flask"
set_system_env "POSTGRES_HOST" "127.0.0.1"
set_system_env "POSTGRES_PORT" "5432"
set_system_env "POSTGRES_USER" "${POSTGRES_USER}"
set_system_env "POSTGRES_PASSWORD" "${POSTGRES_PASSWORD}"
set_system_env "POSTGRES_DB" "${POSTGRES_DB}"
set_system_env "REDIS_URL" "redis://127.0.0.1:6379/0"
set_system_env "TEMP_DIR" "/app/temp_audio"

mkdir -p "${TEMP_DIR}"
chmod +x /etc/profile.d/audiomuse-system.sh
source /etc/profile.d/audiomuse-system.sh

echo ""
echo "✓ SYSTEM environment variables set with admin password:"
echo "  - NAVIDROME_PASSWORD: ${NAVIDROME_PASSWORD}"

echo "=== STEP 9: Testing Music Server API ==="

echo "Testing music server API capabilities..."
PING_RESULT=$(curl -s "http://localhost:8080/rest/ping.view?u=admin&p=admin&f=json" 2>/dev/null || echo "FAILED")
echo "Basic ping test result: $PING_RESULT"

# Keep using admin password for now
echo "✓ Using NAVIDROME_PASSWORD='admin'"

echo "=== STEP 10: Creating AudioMuse-AI Environment File ==="

# Write final environment to .env file
cat > /app/audiomuse-core/.env << EOF
NAVIDROME_USER=${NAVIDROME_USER}
NAVIDROME_PASSWORD=${NAVIDROME_PASSWORD}
MEDIASERVER_TYPE=${MEDIASERVER_TYPE}
NAVIDROME_URL=${NAVIDROME_URL}
SERVICE_TYPE=${SERVICE_TYPE}
POSTGRES_HOST=${POSTGRES_HOST}
POSTGRES_PORT=${POSTGRES_PORT}
POSTGRES_USER=${POSTGRES_USER}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_DB=${POSTGRES_DB}
REDIS_URL=${REDIS_URL}
TEMP_DIR=${TEMP_DIR}
EOF

echo "✓ Environment file created with NAVIDROME_PASSWORD=${NAVIDROME_PASSWORD}"

echo "=== STEP 11: Starting AudioMuse-AI Core with ALL LOGS IN CONTAINER ==="

cd /app/audiomuse-core

# Create named pipes for log streaming
mkfifo /tmp/flask_log /tmp/worker1_log /tmp/worker2_log

# Function to start log forwarders that prefix output
start_log_forwarder() {
    local pipe_name="$1"
    local prefix="$2"
    
    while IFS= read -r line; do
        echo "[$prefix] $line"
    done < "$pipe_name" &
}

# Start log forwarders
start_log_forwarder /tmp/flask_log "FLASK"
start_log_forwarder /tmp/worker1_log "WORKER-1"
start_log_forwarder /tmp/worker2_log "WORKER-2"

# Create startup scripts that output to named pipes
cat > start_flask.sh << 'EOF'
#!/bin/bash
source /etc/profile.d/audiomuse-system.sh

{
    echo "=== AUDIOMUSE-AI FLASK STARTING ==="
    echo "Time: $(date)"
    echo "Working Directory: $(pwd)"
    echo "Environment Variables:"
    printenv | grep -E "(NAVIDROME|POSTGRES|REDIS|SERVICE)" | sort
    echo "=== FLASK LOGS ==="
    
    # Start Flask with all output
    FLASK_DEBUG=1 python3 app.py 2>&1
} > /tmp/flask_log
EOF
chmod +x start_flask.sh

cat > start_worker1.sh << 'EOF'
#!/bin/bash
source /etc/profile.d/audiomuse-system.sh

{
    echo "=== AUDIOMUSE-AI WORKER-1 STARTING ==="
    echo "Time: $(date)"
    echo "Working Directory: $(pwd)"
    echo "Environment Variables:"
    printenv | grep -E "(NAVIDROME|POSTGRES|REDIS|SERVICE)" | sort
    echo "=== WORKER-1 LOGS ==="
    
    # Start RQ worker with all output
    rq worker -u redis://127.0.0.1:6379/0 --verbose 2>&1
} > /tmp/worker1_log
EOF
chmod +x start_worker1.sh

cat > start_worker2.sh << 'EOF'
#!/bin/bash
source /etc/profile.d/audiomuse-system.sh

{
    echo "=== AUDIOMUSE-AI WORKER-2 STARTING ==="
    echo "Time: $(date)"
    echo "Working Directory: $(pwd)"
    echo "Environment Variables:"
    printenv | grep -E "(NAVIDROME|POSTGRES|REDIS|SERVICE)" | sort
    echo "=== WORKER-2 LOGS ==="
    
    # Start RQ worker with all output
    rq worker -u redis://127.0.0.1:6379/0 --verbose 2>&1
} > /tmp/worker2_log
EOF
chmod +x start_worker2.sh

# Start all AudioMuse-AI services
echo "=== STARTING AUDIOMUSE-AI SERVICES ==="

echo "[CONTAINER] Starting RQ Worker 1..."
./start_worker1.sh &
RQ_WORKER_1_PID=$!

echo "[CONTAINER] Starting RQ Worker 2..."
./start_worker2.sh &
RQ_WORKER_2_PID=$!

echo "[CONTAINER] Starting Flask Server..."
./start_flask.sh &
FLASK_PID=$!

echo "✓ All AudioMuse-AI services started:"
echo "  - Flask PID: $FLASK_PID"
echo "  - Worker 1 PID: $RQ_WORKER_1_PID"
echo "  - Worker 2 PID: $RQ_WORKER_2_PID"

# Wait for Flask to be ready
echo "[CONTAINER] Waiting for AudioMuse-AI Core to start..."
for i in {1..60}; do
    if curl -s http://localhost:8000/health >/dev/null 2>&1 || curl -s http://localhost:8000/ >/dev/null 2>&1; then
        echo "[CONTAINER] ✓ AudioMuse-AI Core is ready after $i attempts"
        break
    fi
    if [ $i -eq 60 ]; then
        echo "[CONTAINER] ⚠ AudioMuse-AI Core not responding after 60 attempts"
        echo "[CONTAINER] Process status:"
        ps aux | grep -E "(python|rq)" | grep -v grep
    fi
    sleep 2
done

echo "=== STARTUP COMPLETE ==="
echo ""
echo "🎵 AudioMuse All-in-One Container is ready!"
echo ""
echo "📍 Services available at:"
echo "   🎮 AudioMuse-AI Core:    http://localhost:8000"
echo "   🎵 Music Server Backend: http://localhost:8080"
echo "   🌐 Music Server Frontend: http://localhost:3000"
echo "   🗄️  PostgreSQL Database:  localhost:5432"
echo ""
echo "🔧 Credentials:"
echo "   Music Server - Username: ${NAVIDROME_USER}, Password: ${NAVIDROME_PASSWORD}"
echo "   Database - Host: ${POSTGRES_HOST}:${POSTGRES_PORT}, DB: ${POSTGRES_DB}"
echo ""
echo "📋 Log Prefixes in Container Output:"
echo "   [FLASK] - AudioMuse-AI Flask server logs"
echo "   [WORKER-1] - RQ Worker 1 logs"
echo "   [WORKER-2] - RQ Worker 2 logs"
echo "   [CONTAINER] - Container management logs"
echo ""

# Health monitoring with visible status
while true; do
    sleep 60
    
    # Check services and log to container output
    if ! check_postgres; then
        echo "[CONTAINER] ⚠ $(date): PostgreSQL is down, restarting..."
        su postgres -c "/usr/lib/postgresql/14/bin/postgres -D /config/postgres-data -p 5432" &
        POSTGRES_PID=$!
    fi
    
    if ! check_service "redis" "redis-server"; then
        echo "[CONTAINER] ⚠ $(date): Redis is down, restarting..."
        /usr/bin/redis-server --daemonize yes --loglevel warning
    fi
    
    if ! kill -0 $MUSIC_SERVER_PID 2>/dev/null; then
        echo "[CONTAINER] ⚠ $(date): Music Server is down, restarting..."
        cd /app/audiomuse-server && ./music-server &
        MUSIC_SERVER_PID=$!
    fi
    
    if ! kill -0 $FRONTEND_PID 2>/dev/null; then
        echo "[CONTAINER] ⚠ $(date): Frontend is down, restarting..."
        cd /app/audiomuse-server/music-server-frontend && npm start &
        FRONTEND_PID=$!
    fi
    
    if ! kill -0 $FLASK_PID 2>/dev/null; then
        echo "[CONTAINER] ⚠ $(date): Flask server is down, restarting..."
        cd /app/audiomuse-core && ./start_flask.sh &
        FLASK_PID=$!
    fi
    
    if ! kill -0 $RQ_WORKER_1_PID 2>/dev/null; then
        echo "[CONTAINER] ⚠ $(date): RQ Worker 1 is down, restarting..."
        cd /app/audiomuse-core && ./start_worker1.sh &
        RQ_WORKER_1_PID=$!
    fi
    
    if ! kill -0 $RQ_WORKER_2_PID 2>/dev/null; then
        echo "[CONTAINER] ⚠ $(date): RQ Worker 2 is down, restarting..."
        cd /app/audiomuse-core && ./start_worker2.sh &
        RQ_WORKER_2_PID=$!
    fi
    
    # Periodic status check
    echo "[CONTAINER] $(date): Health check - All services running"
done