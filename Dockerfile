# Multi-stage build for AudioMuse All-in-One container

# STAGE 1: Get source files from local directory
FROM alpine:latest AS source-fetcher
WORKDIR /src
# Copy local files instead of git clone
COPY . ./AudioMuse-AI-MusicServer

# STAGE 2: Build Go Backend for Music Server
FROM golang:1.24-bullseye AS backend-builder
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer .
WORKDIR /src/music-server-backend
RUN go mod init music-server-backend || true
RUN go mod tidy
RUN CGO_ENABLED=1 go build -o music-server .

# STAGE 3: Build React Frontend for Music Server
FROM node:20-bullseye AS frontend-builder
WORKDIR /src
COPY --from=source-fetcher /src/AudioMuse-AI-MusicServer .
WORKDIR /src/music-server-frontend
ARG REACT_APP_API_URL
ENV REACT_APP_API_URL=${REACT_APP_API_URL}
RUN npm install
RUN npm run build

# STAGE 4: Final runtime image
FROM node:20-bullseye

ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies and a lightweight static server for the frontend
RUN apt-get update && apt-get install -y --no-install-recommends \
    supervisor \
    sqlite3 \
    curl \
    bash \
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