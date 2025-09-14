# STAGE 1: Fetch all source code for a more robust build
FROM ubuntu:22.04 AS source-fetcher
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /src
# Force fresh clone every time by adding a build arg
ARG BUILD_DATE
RUN git clone https://github.com/NeptuneHub/AudioMuse-AI-MusicServer.git

# STAGE 2: Build Go Backend for Music Server
FROM golang:1.24-bullseye AS backend-builder
WORKDIR /src/music-server-backend
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer/music-server-backend .
# Initialize Go module if it doesn't exist
RUN if [ ! -f go.mod ]; then go mod init audiomuse-server; fi
RUN go mod tidy
# Build with CGo enabled for go-sqlite3 and place binary in a predictable location
RUN CGO_ENABLED=1 go build -o /app/music-server .

# STAGE 3: Build React Frontend (with fresh build)
FROM node:20-alpine AS frontend-builder
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer .
WORKDIR /src/music-server-frontend

# Clear any existing node_modules and package-lock
RUN rm -rf node_modules package-lock.json

# Install dependencies fresh
RUN npm install

# Build production version instead of using development server
RUN npm run build

# STAGE 4: Final Assembled Image
FROM ghcr.io/neptunehub/audiomuse-ai:latest

ENV DEBIAN_FRONTEND=noninteractive

# Install runtime dependencies for the Music Server
RUN apt-get update && apt-get install -y --no-install-recommends \
    postgresql postgresql-contrib redis-server supervisor curl jq nginx \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
RUN apt-get install -y nodejs

# Re-organize the filesystem from the base image
RUN cd / && mv app audiomuse-core && mkdir app && mv audiomuse-core app/

# Copy pre-built application code from builder stages
WORKDIR /app
RUN mkdir -p /app/audiomuse-server
COPY --from=backend-builder /app/music-server /app/audiomuse-server/music-server
# Copy the built React app (production build)
COPY --from=frontend-builder /src/music-server-frontend/build /app/audiomuse-server/music-server-frontend-build
# Also copy source for development server
COPY --from=frontend-builder /src/music-server-frontend /app/audiomuse-server/music-server-frontend

# Set up PostgreSQL data directory with proper initialization
RUN mkdir -p /config/postgres-data && \
    chown postgres:postgres /config/postgres-data && \
    chmod 700 /config/postgres-data && \
    su postgres -c "/usr/lib/postgresql/*/bin/initdb -D /config/postgres-data" && \
    mkdir -p /var/run/supervisord /var/log/supervisor /run/postgresql && \
    chown -R postgres:postgres /run/postgresql

# Copy configurations and the startup script
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose all service ports including PostgreSQL
EXPOSE 3000 8080 8000 5432

ENTRYPOINT ["/entrypoint.sh"]
CMD []
