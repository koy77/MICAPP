# Multi-stage build for MICAPP
# Stage 1: Build stage
FROM golang:1.23-bullseye AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    libasound2-dev \
    libpulse-dev \
    portaudio19-dev \
    libx11-dev \
    libxrandr-dev \
    libgl1-mesa-dev \
    libxcursor-dev \
    libxinerama-dev \
    libxi-dev \
    libxext-dev \
    libxfixes-dev \
    libxrender-dev \
    libxss1 \
    libglib2.0-0 \
    libgtk-3-0 \
    xclip \
    xdotool \
    wmctrl \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY code/ ./code/
COPY red_cube_icon.svg ./

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o voicetranscriber ./code

# Stage 2: Runtime stage
FROM debian:bullseye-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    libasound2 \
    libpulse0 \
    libportaudio2 \
    libx11-6 \
    libxrandr2 \
    libgl1-mesa-glx \
    libxcursor1 \
    libxinerama1 \
    libxi6 \
    libxext6 \
    libxfixes3 \
    libxrender1 \
    libxss1 \
    libglib2.0-0 \
    libgtk-3-0 \
    ca-certificates \
    xclip \
    xdotool \
    wmctrl \
    pulseaudio \
    && rm -rf /var/lib/apt/lists/*

# Create app user
RUN useradd -m -u 1000 appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/voicetranscriber .
COPY --from=builder /build/red_cube_icon.svg .

# Copy other necessary files
COPY index.html ./

# Create recordings directory
RUN mkdir -p recordings && chown -R appuser:appuser /app

# Switch to app user
USER appuser

# Set environment variables
ENV DISPLAY=:0
ENV PULSE_SERVER=unix:/tmp/pulse-socket

# Expose volume for recordings
VOLUME ["/app/recordings"]

# Default command
CMD ["./voicetranscriber"]

