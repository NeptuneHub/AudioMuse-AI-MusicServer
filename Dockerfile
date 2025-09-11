# STAGE 1: Use the pre-built AudioMuse-AI image as our foundation.
# This image already contains the Python core, dependencies, and ML models.
FROM ghcr.io/neptunehub/audiomuse-ai:latest AS audiomuse-base

# STAGE 2: Build the Go Backend for the Music Server
FROM golang:1.24-bullseye AS backend-builder
WORKDIR /src
# Clone only the music server repository
RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*
RUN git clone https://github.com/NeptuneHub/AudioMuse-AI-MusicServer.git
WORKDIR /src/AudioMuse-AI-MusicServer/music-server-backend
# Initialize the Go module before tidying dependencies
RUN go mod init music-server-backend
RUN go mod tidy
# Build the server executable
RUN CGO_ENABLED=0 go build -o music-server .

# STAGE 3: Prepare the React Frontend
FROM node:20-alpine AS frontend-preparer
WORKDIR /src
COPY --from=backend-builder /src/AudioMuse-AI-MusicServer /AudioMuse-AI-MusicServer
WORKDIR /AudioMuse-AI-MusicServer/music-server-frontend
# Only install dependencies. The dev server will build in memory.
RUN npm install

# STAGE 4: Final Image - Add services to the base image
FROM audiomuse-base

# Set environment variables
ENV LANG=C.UTF-8 \
    PYTHONUNBUFFERED=1 \
    DEBIAN_FRONTEND=noninteractive

# Install remaining services: Postgres, Redis, Supervisor, and Node.js
# The base image already has Python.
RUN apt-get update && apt-get install -y --no-install-recommends \
    postgresql redis-server supervisor curl \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js for the React dev server
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs

# --- Supervisor Configuration ---
# This is largely the same, but simplified as we don't need to configure Python paths
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# --- Entrypoint for DB Initialization ---
# This script initializes the PostgreSQL database on the first run
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Create directories and copy application code from our build stages
WORKDIR /app
# The audiomuse-core is already in the base image at /app/audiomuse-core
# We just need to add the music server components
RUN mkdir -p /app/audiomuse-server
COPY --from=backend-builder /src/AudioMuse-AI-MusicServer/music-server-backend/music-server /app/audiomuse-server/music-server
COPY --from=frontend-preparer /AudioMuse-AI-MusicServer/music-server-frontend /app/audiomuse-server/music-server-frontend

# Expose the ports for the different services
EXPOSE 3000 8080 8000

# Set the entrypoint and default command
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]


