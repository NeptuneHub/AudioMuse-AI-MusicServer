# STAGE 1: Fetch all source code for a more robust build
FROM ubuntu:22.04 AS source_fetch_stage
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN git clone https://github.com/NeptuneHub/AudioMuse-AI-MusicServer.git

# STAGE 2: Build Go Backend for Music Server
FROM golang:1.24-bullseye AS backend_build_stage
# Use a clean working directory
WORKDIR /app
# Copy ONLY the backend source code into the build stage
COPY --from=source_fetch_stage /src/AudioMuse-AI-MusicServer/music-server-backend .
# `go mod init` is not needed as go.mod exists. Tidy dependencies instead.
RUN go mod tidy
RUN go build -o music-server .

# STAGE 3: Install React Frontend Dependencies
FROM node:20-alpine AS frontend_build_stage
# Use a clean working directory
WORKDIR /app
# Copy ONLY the frontend source code into the build stage
COPY --from=source_fetch_stage /src/AudioMuse-AI-MusicServer/music-server-frontend .
RUN npm install

# STAGE 4: Final Assembled Image
FROM ghcr.io/neptunehub/audiomuse-ai:latest

ENV DEBIAN_FRONTEND=noninteractive

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    postgresql redis-server supervisor curl jq \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
RUN apt-get install -y nodejs

# Re-organize the filesystem from the base image
RUN cd / && mv app audiomuse-core && mkdir app && mv audiomuse-core app/

# --- Copy Configurations and Scripts ---
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# --- Copy Application Code ---
WORKDIR /app
RUN mkdir -p /app/audiomuse-server

# Copy the built Go backend binary from its build stage
COPY --from=backend_build_stage /app/music-server /app/audiomuse-server/music-server

# Copy the entire built frontend application from its build stage
COPY --from=frontend_build_stage /app /app/audiomuse-server/music-server-frontend

# Set up directories for supervisor and postgres
RUN mkdir -p /var/run/supervisord /var/log/supervisor /run/postgresql && \
    chown -R postgres:postgres /run/postgresql

EXPOSE 3000 8080 8000

ENTRYPOINT ["/entrypoint.sh"]
CMD []

