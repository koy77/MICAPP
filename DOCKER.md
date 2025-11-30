# Docker Setup Guide for MICAPP

## Quick Start

1. **Set your OpenAI API key:**
   ```bash
   export OPENAI_API_KEY="your-api-key-here"
   ```

2. **Setup X11 and Audio (run once):**
   ```bash
   xhost +local:docker
   pactl load-module module-native-protocol-unix socket=/tmp/pulse-socket
   ```

3. **Run the application:**
   ```bash
   ./docker-run.sh
   ```

## Manual Docker Commands

### Build the image:
```bash
docker-compose build
```

### Run with docker-compose:
```bash
docker-compose up
```

### Run in detached mode:
```bash
docker-compose up -d
```

### Stop the container:
```bash
docker-compose down
```

### View logs:
```bash
docker-compose logs -f
```

## Troubleshooting

### X11 Connection Issues

If you see "Cannot connect to X server" errors:

```bash
# Allow Docker to connect to X11
xhost +local:docker

# Verify DISPLAY is set
echo $DISPLAY

# If not set, export it:
export DISPLAY=:0
```

### Audio Issues

If audio doesn't work:

```bash
# Load PulseAudio module for Docker
pactl load-module module-native-protocol-unix socket=/tmp/pulse-socket

# Verify PulseAudio is running
pulseaudio --check -v

# List audio devices
arecord -l
```

### Permission Issues

If you get permission denied errors:

```bash
# Add your user to docker group (requires logout/login)
sudo usermod -aG docker $USER

# Or run with sudo (not recommended)
sudo docker-compose up
```

### Container Won't Start

Check if ports/devices are already in use:

```bash
# Check if container is already running
docker ps -a | grep micapp

# Remove old container if needed
docker-compose down
docker-compose rm -f
```

## Advanced Usage

### Run with custom environment variables:

```bash
OPENAI_API_KEY="your-key" DISPLAY=:0 docker-compose up
```

### Access container shell:

```bash
docker-compose exec micapp /bin/bash
```

### Rebuild after code changes:

```bash
docker-compose build --no-cache
docker-compose up
```

## Notes

- The `recordings/` directory is mounted as a volume, so recordings persist on the host
- X11 authentication is required for GUI display
- Audio requires PulseAudio socket setup
- The container runs with network_mode: host for better device access

