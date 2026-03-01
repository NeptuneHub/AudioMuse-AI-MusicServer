# Multi-stage build for AudioMuse All-in-One container

# STAGE 1: Get source files from local directory
FROM alpine:latest AS source-fetcher
WORKDIR /src
# Copy local files instead of git clone
COPY . ./AudioMuse-AI-MusicServer

# STAGE 2: Build Go Backend for Music Server
FROM golang:1.25-bookworm AS backend-builder
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer .
WORKDIR /src/music-server-backend
# dependencies are tracked in go.mod/go.sum, so just download them
RUN go mod download
RUN CGO_ENABLED=1 go build -o music-server .

# STAGE 3: Build React Frontend for Music Server
# Use bookworm to stay consistent with backend-builder and avoid libc mismatches
FROM node:20-bookworm AS frontend-builder
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer .
WORKDIR /src/music-server-frontend
ARG REACT_APP_API_URL
ENV REACT_APP_API_URL=${REACT_APP_API_URL}
RUN npm install
RUN npm run build

# STAGE 4: Final runtime image
# runtime must have at least the same libc version as the Go build stage
FROM node:20-bookworm

ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies and a lightweight static server for the frontend
RUN apt-get update && apt-get install -y --no-install-recommends \
    supervisor \
    sqlite3 \
    curl \
    bash \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# Install 'serve' globally to serve the built frontend
RUN npm install -g serve@14.1.2

# Create application directory
WORKDIR /app

# Copy built Go backend
COPY --from=backend-builder /src/music-server-backend/music-server ./music-server
RUN chmod +x ./music-server

# Copy React frontend source and dependencies
# Copy only the built frontend (static files)
COPY --from=frontend-builder /src/music-server-frontend/build ./music-server-frontend/build
# Ensure a /favicon.ico exists in the build by copying the PNG if ICO is missing
RUN if [ -f /app/music-server-frontend/build/audiomuseai.png ] && [ ! -f /app/music-server-frontend/build/favicon.ico ]; then cp /app/music-server-frontend/build/audiomuseai.png /app/music-server-frontend/build/favicon.ico; fi

# Copy configurations and startup script
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Create necessary directories
RUN mkdir -p /var/log/supervisor \
    && mkdir -p /config

# Expose ports
EXPOSE 3000 8080

# Use supervisor to manage multiple processes
ENTRYPOINT ["/entrypoint.sh"]