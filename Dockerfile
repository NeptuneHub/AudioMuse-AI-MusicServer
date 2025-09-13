# STAGE 1: Fetch all source code for a more robust build
FROM ubuntu:22.04 AS source-fetcher
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /src
# Only clone the Music Server, as the AI Core comes from the base image
RUN git clone https://github.com/NeptuneHub/AudioMuse-AI-MusicServer.git

# STAGE 2: Build Go Backend for Music Server
FROM golang:1.24-bullseye AS backend-builder
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer .
WORKDIR /src/music-server-backend
RUN go mod init music-server-backend
RUN go mod tidy
# Build with CGo enabled so go-sqlite3 works correctly.
RUN go build -o music-server .

# STAGE 3: Install React Frontend Dependencies
FROM node:20-alpine AS frontend-builder
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer .
WORKDIR /src/music-server-frontend
# Only install dependencies, do not build
RUN npm install

# STAGE 4: Final Assembled Image
FROM ghcr.io/neptunehub/audiomuse-ai:latest

ENV DEBIAN_FRONTEND=noninteractive

# Install runtime dependencies for the Music Server (Postgres, Node.js for dev server)
RUN apt-get update && apt-get install -y --no-install-recommends \
    postgresql redis-server supervisor curl jq \
    && rm -rf /var/lib/apt/lists/*
# Install Node.js and npm for the React dev server
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
RUN apt-get install -y nodejs

# Re-organize the filesystem from the base image.
# The base image has AI code in /app, we move it to /app/audiomuse-core
# to make space for the other components.
RUN cd / && mv app audiomuse-core && mkdir app && mv audiomuse-core app/

# --- Copy Configurations and Scripts ---
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY entrypoint.sh /entrypoint.sh
# Copy the new AI Core wrapper script
COPY start-ai-core.sh /app/audiomuse-core/start-ai-core.sh

RUN chmod +x /entrypoint.sh
# Make the new wrapper script executable
RUN chmod +x /app/audiomuse-core/start-ai-core.sh

# --- Copy Application Code ---
WORKDIR /app
# The audiomuse-core directory is now in /app from the step above.
RUN mkdir -p /app/audiomuse-server

# Copy the built Go backend
COPY --from=backend-builder /src/music-server-backend/music-server /app/audiomuse-server/music-server
# Copy the React frontend with its node_modules
COPY --from=frontend-builder /src/music-server-frontend /app/audiomuse-server/music-server-frontend

# Set up directories for supervisor and postgres
RUN mkdir -p /var/run/supervisord /var/log/supervisor /run/postgresql && \
    chown -R postgres:postgres /run/postgresql

EXPOSE 3000 8080 8000

ENTRYPOINT ["/entrypoint.sh"]
# The command to run is supervisord. The entrypoint will handle initialization first.
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]

