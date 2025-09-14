# Multi-stage build for AudioMuse All-in-One container

# STAGE 1: Get source files from local directory
FROM alpine:latest AS source-fetcher
WORKDIR /src
# Copy local files instead of git clone
COPY . ./AudioMuse-AI-MusicServer

# STAGE 2: Build Go Backend for Music Server
FROM golang:1.24-alpine AS backend-builder
# Install build dependencies for Alpine
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer .
WORKDIR /src/music-server-backend
RUN go mod init music-server-backend || true
RUN go mod tidy
# Build static binary for Alpine
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o music-server .

# STAGE 3: Build React Frontend for Music Server
FROM node:18-bullseye AS frontend-builder
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer .
WORKDIR /src/music-server-frontend
RUN npm install

# STAGE 4: Final runtime image
FROM node:20-alpine

# Install system dependencies
RUN apk update && apk add --no-cache \
    supervisor \
    sqlite \
    curl \
    bash \
    && rm -rf /var/cache/apk/*

# Create application directory
WORKDIR /app

# Copy built Go backend
COPY --from=backend-builder /src/music-server-backend/music-server ./music-server
RUN chmod +x ./music-server

# Copy React frontend source and dependencies
COPY --from=frontend-builder /src/music-server-frontend ./music-server-frontend

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