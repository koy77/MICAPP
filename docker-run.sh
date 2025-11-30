#!/bin/bash

# Docker run script for MICAPP
# This script helps run MICAPP in a Docker container with proper X11 and audio setup

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if OPENAI_API_KEY is set
if [ -z "$OPENAI_API_KEY" ]; then
    print_error "OPENAI_API_KEY environment variable is not set."
    echo "Please set your OpenAI API key:"
    echo "  export OPENAI_API_KEY=\"your-api-key-here\""
    exit 1
fi

# Check if DISPLAY is set
if [ -z "$DISPLAY" ]; then
    print_warning "DISPLAY environment variable is not set. Setting to :0"
    export DISPLAY=:0
fi

# Allow X11 connections
print_status "Setting up X11 permissions..."
xhost +local:docker 2>/dev/null || print_warning "xhost command failed, continuing anyway..."

# Setup PulseAudio socket
print_status "Setting up PulseAudio..."
PULSE_SOCKET="/tmp/pulse-socket"
if [ -S "$PULSE_SOCKET" ]; then
    print_success "PulseAudio socket found"
else
    print_warning "PulseAudio socket not found. Audio may not work."
    print_warning "To enable audio, run: pactl load-module module-native-protocol-unix socket=$PULSE_SOCKET"
fi

# Get user ID and group ID
USER_ID=$(id -u)
GROUP_ID=$(id -g)

print_status "Building Docker image (if needed)..."
docker-compose build

print_status "Starting MICAPP container..."
print_status "User ID: $USER_ID, Group ID: $GROUP_ID"
print_status "Display: $DISPLAY"

# Run with docker-compose
docker-compose up

# Cleanup on exit
print_status "Cleaning up..."
xhost -local:docker 2>/dev/null || true

print_success "Done!"

