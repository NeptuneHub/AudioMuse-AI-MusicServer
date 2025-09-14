# STAGE 1: Fetch all source code for a more robust build
FROM ubuntu:22.04 AS source-fetcher
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN git clone https://github.com/NeptuneHub/AudioMuse-AI-MusicServer.git

# STAGE 2: Build Go Backend for Music Server
FROM golang:1.21-bullseye AS backend-builder
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer/music-server-backend ./music-server-backend
WORKDIR /src/music-server-backend
RUN if [ ! -f go.mod ]; then go mod init audiomuse-server; fi
RUN go mod tidy
RUN CGO_ENABLED=1 go build -o /app/music-server .

# STAGE 3: Install React Frontend Dependencies
FROM node:20-alpine AS frontend-builder
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer .
WORKDIR /src/music-server-frontend
RUN npm install

# STAGE 4: Final Assembled Image
FROM ghcr.io/neptunehub/audiomuse-ai:latest

ENV DEBIAN_FRONTEND=noninteractive

# Install runtime dependencies for the Music Server
RUN apt-get update && apt-get install -y --no-install-recommends \
    postgresql redis-server supervisor curl jq \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
RUN apt-get install -y nodejs

# Re-organize the filesystem from the base image
RUN cd / && mv app audiomuse-core && mkdir app && mv audiomuse-core app/

# Copy pre-built application code from builder stages
WORKDIR /app
RUN mkdir -p /app/audiomuse-server
COPY --from=backend-builder /app/music-server /app/audiomuse-server/music-server
COPY --from=frontend-builder /src/music-server-frontend /app/audiomuse-server/music-server-frontend

# Set up PostgreSQL data directory with proper initialization
RUN mkdir -p /config/postgres-data && \
    chown postgres:postgres /config/postgres-data && \
    chmod 700 /config/postgres-data && \
    su postgres -c "initdb -D /config/postgres-data" && \
    mkdir -p /var/run/supervisord /var/log/supervisor /run/postgresql && \
    chown -R postgres:postgres /run/postgresql

# Copy configurations and the startup script
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 3000 8080 8000

ENTRYPOINT ["/entrypoint.sh"]
CMD []
