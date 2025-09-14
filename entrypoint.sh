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

echo "=== STEP 11: Starting AudioMuse-AI Core with MAXIMUM JOB DEBUGGING ==="

cd /app/audiomuse-core

# Create a Redis monitor to watch ALL Redis activity
redis-cli monitor > /tmp/redis_monitor.log &
REDIS_MONITOR_PID=$!

# Create enhanced startup scripts with comprehensive job tracking
cat > start_flask.sh << 'EOF'
#!/bin/bash
source /etc/profile.d/audiomuse-system.sh

echo "=== AUDIOMUSE-AI FLASK STARTING ==="
echo "Time: $(date)"
echo "Working Directory: $(pwd)"
echo "Environment Variables:"
printenv | grep -E "(NAVIDROME|POSTGRES|REDIS|SERVICE)" | sort

# Test all connections
echo "Testing connections..."
python3 -c "
import os, sys
import psycopg2
import redis
import requests
from rq import Queue

try:
    # Test Redis
    r = redis.from_url(os.environ['REDIS_URL'])
    r.ping()
    print('✓ Redis connection successful')
    
    # Test Queue
    q = Queue(connection=r)
    print(f'✓ Queue created, current length: {len(q)}')
    
    # Test Database
    conn = psycopg2.connect(
        host=os.environ['POSTGRES_HOST'],
        port=os.environ['POSTGRES_PORT'],
        database=os.environ['POSTGRES_DB'],
        user=os.environ['POSTGRES_USER'],
        password=os.environ['POSTGRES_PASSWORD']
    )
    print('✓ Database connection successful')
    conn.close()
    
    # Test Music Server
    resp = requests.get(f\"{os.environ['NAVIDROME_URL']}/rest/ping.view?u={os.environ['NAVIDROME_USER']}&p={os.environ['NAVIDROME_PASSWORD']}&f=json\", timeout=5)
    if resp.status_code == 200:
        print('✓ Music server connection successful')
        print(f'   Response: {resp.text[:100]}')
    else:
        print(f'⚠ Music server responded with status {resp.status_code}')
        
except Exception as e:
    print(f'✗ Connection test failed: {e}')
    import traceback
    traceback.print_exc()
"

echo "=== FLASK LOGS ==="

# Start Flask with ALL debugging enabled
export FLASK_DEBUG=1
export FLASK_ENV=development
export PYTHONUNBUFFERED=1
python3 app.py 2>&1
EOF
chmod +x start_flask.sh

cat > start_worker.sh << 'EOF'
#!/bin/bash
source /etc/profile.d/audiomuse-system.sh

echo "=== AUDIOMUSE-AI WORKER STARTING ==="
echo "Time: $(date)"
echo "Working Directory: $(pwd)"
echo "Environment Variables:"
printenv | grep -E "(NAVIDROME|POSTGRES|REDIS|SERVICE)" | sort

# Test connections with detailed output
echo "Testing worker connections..."
python3 -c "
import os, sys
import redis
import psycopg2
from rq import Queue, Worker
from rq.job import Job
import logging

# Enable detailed logging
logging.basicConfig(level=logging.DEBUG)

try:
    # Test Redis
    r = redis.from_url(os.environ['REDIS_URL'])
    r.ping()
    print('✓ Redis connection successful')
    
    # Test Queue and show all jobs
    q = Queue(connection=r)
    print(f'✓ Queue created, current length: {len(q)}')
    print(f'✓ Queue jobs: {q.job_ids}')
    
    # Show queue registry
    from rq.registry import StartedJobRegistry, FinishedJobRegistry, FailedJobRegistry
    started_registry = StartedJobRegistry(connection=r)
    finished_registry = FinishedJobRegistry(connection=r)
    failed_registry = FailedJobRegistry(connection=r)
    
    print(f'✓ Started jobs: {len(started_registry)}')
    print(f'✓ Finished jobs: {len(finished_registry)}')
    print(f'✓ Failed jobs: {len(failed_registry)}')
    
    if len(failed_registry) > 0:
        print('Failed jobs details:')
        for job_id in failed_registry.get_job_ids():
            try:
                job = Job.fetch(job_id, connection=r)
                print(f'  - {job_id}: {job.exc_info}')
            except Exception as e:
                print(f'  - {job_id}: Could not fetch job details: {e}')
    
    # Test Database
    conn = psycopg2.connect(
        host=os.environ['POSTGRES_HOST'],
        port=os.environ['POSTGRES_PORT'],
        database=os.environ['POSTGRES_DB'],
        user=os.environ['POSTGRES_USER'],
        password=os.environ['POSTGRES_PASSWORD']
    )
    print('✓ Database connection successful')
    conn.close()
    
except Exception as e:
    print(f'✗ Worker connection test failed: {e}')
    import traceback
    traceback.print_exc()
"

echo "=== WORKER LOGS ==="

# Start RQ worker with MAXIMUM debugging
export PYTHONUNBUFFERED=1
export RQ_WORKER_LOG_LEVEL=DEBUG
export PYTHONPATH=/app/audiomuse-core:$PYTHONPATH

# Start worker with job failure logging
rq worker -u redis://127.0.0.1:6379/0 --verbose --with-scheduler --exception-handler rq.handlers.move_to_failed_queue 2>&1
EOF
chmod +x start_worker.sh

# Create a queue monitor script
cat > monitor_queue.sh << 'EOF'
#!/bin/bash
source /etc/profile.d/audiomuse-system.sh

echo "=== QUEUE MONITOR STARTING ==="
while true; do
    echo "[QUEUE-MONITOR $(date '+%H:%M:%S')] Checking queue status..."
    python3 -c "
import redis
from rq import Queue
from rq.registry import StartedJobRegistry, FinishedJobRegistry, FailedJobRegistry
import os

try:
    r = redis.from_url(os.environ['REDIS_URL'])
    q = Queue(connection=r)
    
    started_registry = StartedJobRegistry(connection=r)
    finished_registry = FinishedJobRegistry(connection=r)
    failed_registry = FailedJobRegistry(connection=r)
    
    print(f'Queue: {len(q)} jobs | Started: {len(started_registry)} | Finished: {len(finished_registry)} | Failed: {len(failed_registry)}')
    
    if len(q) > 0:
        print('Pending jobs:')
        for job_id in q.job_ids[:5]:  # Show first 5
            try:
                from rq.job import Job
                job = Job.fetch(job_id, connection=r)
                print(f'  - {job_id}: {job.func_name} | Status: {job.get_status()}')
            except Exception as e:
                print(f'  - {job_id}: Error fetching job: {e}')
    
    if len(failed_registry) > 0:
        print('Recent failed jobs:')
        for job_id in failed_registry.get_job_ids()[:3]:  # Show first 3
            try:
                from rq.job import Job
                job = Job.fetch(job_id, connection=r)
                print(f'  - {job_id}: {job.exc_info}')
            except Exception as e:
                print(f'  - {job_id}: Error fetching failed job: {e}')
                
except Exception as e:
    print(f'Queue monitor error: {e}')
"
    sleep 30
done
EOF
chmod +x monitor_queue.sh

# Create named pipes for log streaming
mkfifo /tmp/flask_log /tmp/worker_log /tmp/queue_log /tmp/redis_log 2>/dev/null || true

# Function to start log forwarders with timestamps
start_log_forwarder() {
    local pipe_name="$1"
    local prefix="$2"
    
    while IFS= read -r line; do
        echo "[$prefix $(date '+%H:%M:%S')] $line"
    done < "$pipe_name" &
}

# Start log forwarders
start_log_forwarder /tmp/flask_log "FLASK"
start_log_forwarder /tmp/worker_log "WORKER"
start_log_forwarder /tmp/queue_log "QUEUE"
start_log_forwarder /tmp/redis_monitor.log "REDIS"

# Monitor Redis activity
echo "[CONTAINER] Starting Redis monitor..."
tail -f /tmp/redis_monitor.log > /tmp/redis_log &

# Start Queue Monitor
echo "[CONTAINER] Starting Queue Monitor..."
./monitor_queue.sh > /tmp/queue_log 2>&1 &
QUEUE_MONITOR_PID=$!

# Start Flask server
echo "[CONTAINER] Starting Flask Server..."
./start_flask.sh > /tmp/flask_log 2>&1 &
FLASK_PID=$!

# Start ONLY ONE RQ worker
echo "[CONTAINER] Starting RQ Worker..."
./start_worker.sh > /tmp/worker_log 2>&1 &
RQ_WORKER_PID=$!

echo "✓ AudioMuse-AI services started:"
echo "  - Flask PID: $FLASK_PID"
echo "  - Worker PID: $RQ_WORKER_PID"
echo "  - Queue Monitor PID: $QUEUE_MONITOR_PID"
echo "  - Redis Monitor PID: $REDIS_MONITOR_PID"

# Wait for Flask to be ready
echo "[CONTAINER] Waiting for AudioMuse-AI Core to start..."
for i in {1..60}; do
    if curl -s http://localhost:8000/health >/dev/null 2>&1 || curl -s http://localhost:8000/ >/dev/null 2>&1; then
        echo "[CONTAINER] ✓ AudioMuse-AI Core is ready after $i attempts"
        break
    fi
    if [ $i -eq 60 ]; then
        echo "[CONTAINER] ⚠ AudioMuse-AI Core not responding after 60 attempts"
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
echo ""
echo "🔧 Credentials:"
echo "   Music Server - Username: ${NAVIDROME_USER}, Password: ${NAVIDROME_PASSWORD}"
echo ""
echo "📋 Enhanced Log Prefixes:"
echo "   [FLASK] - AudioMuse-AI Flask server logs"
echo "   [WORKER] - RQ Worker logs with job processing details"
echo "   [QUEUE] - Queue status monitoring every 30 seconds"
echo "   [REDIS] - Redis command monitoring"
echo "   [CONTAINER] - Container management logs"
echo ""
echo "🚀 Now start an analysis and watch for detailed job processing logs!"

# Simplified health monitoring
while true; do
    sleep 60
    
    # Check and restart services if needed
    if ! check_postgres; then
        echo "[CONTAINER] ⚠ PostgreSQL down, restarting..."
        su postgres -c "/usr/lib/postgresql/14/bin/postgres -D /config/postgres-data -p 5432" &
        POSTGRES_PID=$!
    fi
    
    if ! check_service "redis" "redis-server"; then
        echo "[CONTAINER] ⚠ Redis down, restarting..."
        /usr/bin/redis-server --daemonize yes --loglevel warning
        redis-cli monitor > /tmp/redis_monitor.log &
        REDIS_MONITOR_PID=$!
    fi
    
    if ! kill -0 $FLASK_PID 2>/dev/null; then
        echo "[CONTAINER] ⚠ Flask down, restarting..."
        cd /app/audiomuse-core && ./start_flask.sh > /tmp/flask_log 2>&1 &
        FLASK_PID=$!
    fi
    
    if ! kill -0 $RQ_WORKER_PID 2>/dev/null; then
        echo "[CONTAINER] ⚠ Worker down, restarting..."
        cd /app/audiomuse-core && ./start_worker.sh > /tmp/worker_log 2>&1 &
        RQ_WORKER_PID=$!
    fi
    
    if ! kill -0 $QUEUE_MONITOR_PID 2>/dev/null; then
        echo "[CONTAINER] ⚠ Queue monitor down, restarting..."
        cd /app/audiomuse-core && ./monitor_queue.sh > /tmp/queue_log 2>&1 &
        QUEUE_MONITOR_PID=$!
    fi
done