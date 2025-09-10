# This single Dockerfile builds the entire AudioMuse stack in a development-like mode.
# Each service runs on its own port, and Nginx is completely removed.

# STAGE 1: Fetch all source code first for a more robust build
FROM ubuntu:22.04 AS source-fetcher
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN git clone https://github.com/NeptuneHub/AudioMuse-AI-MusicServer.git
RUN git clone https://github.com/NeptuneHub/AudioMuse-AI.git

# STAGE 2: Download ML Models for AudioMuse-AI Core
FROM ubuntu:22.04 AS models
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /app/model
RUN wget -q -P /app/model \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/danceability-msd-musicnn-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/mood_aggressive-audioset-vggish-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/mood_aggressive-msd-musicnn-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/mood_happy-audioset-vggish-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/mood_happy-msd-musicnn-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/mood_party-audioset-vggish-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/mood_party-msd-musicnn-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/mood_relaxed-audioset-vggish-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/mood_relaxed-msd-musicnn-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/mood_sad-audioset-vggish-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/mood_sad-msd-musicnn-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/msd-msd-musicnn-1.pb \
    https://github.com/NeptuneHub/AudioMuse-AI/releases/download/v1.0.0-model/msd-musicnn-1.pb

# STAGE 3: Build Python Dependencies for AudioMuse-AI Core
FROM ubuntu:22.04 AS python-builder
RUN apt-get update && apt-get install -y --no-install-recommends python3 python3-pip python3-dev libopenblas-dev liblapack-dev && rm -rf /var/lib/apt/lists/*
RUN --mount=type=cache,target=/root/.cache/pip \
    pip3 install --prefix=/install \
      Flask Flask-Cors redis requests scikit-learn rq pyyaml six voyager rapidfuzz \
      psycopg2-binary ftfy flasgger sqlglot google-generativeai pydub \
      tensorflow==2.15.0 librosa

# STAGE 4: Install React Frontend Dependencies
FROM node:20-alpine AS frontend-builder
WORKDIR /
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer /AudioMuse-AI-MusicServer
WORKDIR /AudioMuse-AI-MusicServer/music-server-frontend
# Only install dependencies, do not build
RUN npm install

# STAGE 5: Build Go Backend for Music Server using a glibc-based image
FROM golang:1.24-bullseye AS backend-builder
WORKDIR /
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer /AudioMuse-AI-MusicServer
WORKDIR /AudioMuse-AI-MusicServer/music-server-backend
RUN go mod init music-server-backend
RUN go mod tidy
# Build with cgo enabled so go-sqlite3 works
RUN go build -o music-server .

# STAGE 6: Final Assembled Image
FROM ubuntu:22.04

ENV LANG=C.UTF-8 \
    PYTHONUNBUFFERED=1 \
    DEBIAN_FRONTEND=noninteractive

# Install all runtime dependencies, including Node.js for the React dev server
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 python3-pip curl \
    postgresql redis-server supervisor sed \
    libfftw3-3 libyaml-0-2 libsamplerate0 libsndfile1 ffmpeg \
    libpq-dev gcc g++ \
    && rm -rf /var/lib/apt/lists/*
# Install Node.js and npm for the React dev server
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
RUN apt-get install -y nodejs

# --- Embedded Supervisor Configuration ---
RUN echo '[supervisord]' > /etc/supervisor/conf.d/supervisord.conf && \
    echo 'nodaemon=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'user=root' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '[program:redis]' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'command=/usr/bin/redis-server --loglevel warning' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autostart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autorestart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'user=redis' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile=/dev/stdout' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile=/dev/stderr' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '[program:postgres]' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'command=/usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/data' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autostart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autorestart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'user=postgres' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile=/dev/stdout' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile=/dev/stderr' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '[program:go-music-server]' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'command=/app/audiomuse-server/music-server' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'directory=/app/audiomuse-server' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autostart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autorestart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile=/dev/stdout' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile=/dev/stderr' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'environment=' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '    GIN_MODE="release"' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '[program:python-flask-core]' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'command=python3 /app/audiomuse-core/app.py' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'directory=/app/audiomuse-core' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autostart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autorestart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile=/dev/stdout' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile=/dev/stderr' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'environment=' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '    SERVICE_TYPE="flask",' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '    POSTGRES_HOST="127.0.0.1"' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '[program:python-rq-worker]' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'command=rq worker -u redis://127.0.0.1:6379/0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'directory=/app/audiomuse-core' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autostart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autorestart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'process_name=%(program_name)s_%(process_num)02d' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'numprocs=2' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile=/dev/stdout' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile=/dev/stderr' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo '[program:react-frontend]' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'command=npm start' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'directory=/app/audiomuse-server/music-server-frontend' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autostart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'autorestart=true' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile=/dev/stdout' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stdout_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile=/dev/stderr' >> /etc/supervisor/conf.d/supervisord.conf && \
    echo 'stderr_logfile_maxbytes=0' >> /etc/supervisor/conf.d/supervisord.conf

# --- Embedded Entrypoint Script ---
RUN echo '#!/bin/bash' > /entrypoint.sh && \
    echo 'set -e' >> /entrypoint.sh && \
    echo 'PGDATA_DIR="/var/lib/postgresql/data"' >> /entrypoint.sh && \
    echo 'if [ ! -d "$PGDATA_DIR" ] || [ -z "$(ls -A "$PGDATA_DIR")" ]; then' >> /entrypoint.sh && \
    echo '    echo "PostgreSQL data directory not found or empty. Initializing database..."' >> /entrypoint.sh && \
    echo '    su postgres -c "/usr/lib/postgresql/14/bin/initdb -D \"$PGDATA_DIR\" --username=postgres"' >> /entrypoint.sh && \
    echo '    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D \"$PGDATA_DIR\" start"' >> /entrypoint.sh && \
    echo '    sleep 5' >> /entrypoint.sh && \
    echo '    su postgres -c "psql --command \"CREATE USER audiomuse WITH SUPERUSER PASSWORD '\''audiomusepassword'\'';\""' >> /entrypoint.sh && \
    echo '    su postgres -c "psql --command \"CREATE DATABASE audiomusedb OWNER audiomuse;\""' >> /entrypoint.sh && \
    echo '    su postgres -c "/usr/lib/postgresql/14/bin/pg_ctl -D \"$PGDATA_DIR\" stop"' >> /entrypoint.sh && \
    echo 'fi' >> /entrypoint.sh && \
    echo 'exec "$@"' >> /entrypoint.sh

# Make the script executable
RUN chmod +x /entrypoint.sh

# Create directories and copy application code
RUN mkdir -p /var/run/supervisord /var/log/supervisor /var/lib/postgresql/data /run/postgresql /app/audiomuse-server
RUN chown -R postgres:postgres /var/lib/postgresql/data /run/postgresql /var/log/supervisor
WORKDIR /app
COPY --from=python-builder /install/ /usr/
COPY --from=source-fetcher /src/AudioMuse-AI /app/audiomuse-core
COPY --from=models /app/model/ /app/audiomuse-core/model/
COPY --from=backend-builder /AudioMuse-AI-MusicServer/music-server-backend/music-server /app/audiomuse-server/music-server
COPY --from=frontend-builder /AudioMuse-AI-MusicServer/music-server-frontend /app/audiomuse-server/music-server-frontend

ENV PYTHONPATH=/usr/local/lib/python3/dist-packages:/app/audiomuse-core
EXPOSE 3000 8080 8000

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]


