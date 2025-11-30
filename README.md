# MICAPP

Voice transcription application with GUI built with Go and Fyne framework.

## Features

- Voice recording and transcription using OpenAI Whisper API
- GUI interface built with Fyne
- Audio storage with multiple bitrate options
- Text correction using LLM
- Screenshot capture functionality

## Quick Start

### Set your OpenAI API key (required for both methods):
```bash
export OPENAI_API_KEY="your-api-key-here"
```

---

## Native Build (Recommended)

Build and run directly on your host machine using Go.

### Prerequisites

- Go 1.23.0 or later
- Audio libraries: libasound2-dev, libpulse-dev, portaudio19-dev
- X11 libraries for GUI
- xclip, xdotool, wmctrl utilities

### Install Dependencies (Ubuntu/Debian)

1. **Install Go:**
   ```bash
   wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
   sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
   ```

   Add to `~/.bashrc`:
   ```bash
   export PATH=$PATH:/usr/local/go/bin
   export GOPATH=$HOME/go
   export PATH=$PATH:$GOPATH/bin
   ```

2. **Install audio dependencies:**
   ```bash
   sudo apt install -y libasound2-dev libpulse-dev portaudio19-dev
   ```

3. **Install GUI dependencies:**
   ```bash
   sudo apt install -y libx11-dev libxrandr-dev libgl1-mesa-dev \
     libxcursor-dev libxinerama-dev libxi-dev libxext-dev \
     libxfixes-dev libxrender-dev libxss1 libglib2.0-0 libgtk-3-0
   ```

4. **Install utilities:**
   ```bash
   sudo apt install -y xclip xdotool wmctrl
   ```

### Build & Run (Native)

**Using the start script (recommended):**
```bash
./start.sh              # Build and run
./start.sh --build      # Only build
./start.sh --run        # Only run (requires existing build)
./start.sh --deps       # Only install Go dependencies
./start.sh --clean      # Clean build artifacts
./start.sh --help       # Show all options
```

**Manual commands:**
```bash
# Install Go dependencies
go mod download

# Build the application
CGO_ENABLED=1 go build -ldflags='-s -w' -o voicetranscriber ./code

# Run the application
./voicetranscriber
```

---

## Docker Build

Build and run using Docker containers. Handles all dependencies automatically.

### Prerequisites

- Docker and Docker Compose installed
- X11 server running (for GUI)
- PulseAudio running (for audio)

### Setup X11 and Audio

1. **Allow X11 connections:**
   ```bash
   xhost +local:docker
   ```

2. **Setup PulseAudio socket (for audio):**
   ```bash
   pactl load-module module-native-protocol-unix socket=/tmp/pulse-socket
   ```

### Build & Run (Docker)

**Using docker-compose (recommended):**
```bash
# Build and run
docker-compose up

# Build only
docker-compose build

# Run in background
docker-compose up -d
```

**Using docker-run script:**
```bash
./docker-run.sh
```

**Using docker directly:**
```bash
# Build the image
docker build -t micapp:latest .

# Run the container
docker run -it \
  --rm \
  -e DISPLAY=$DISPLAY \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  -v /tmp/.X11-unix:/tmp/.X11-unix:rw \
  -v /tmp/pulse-socket:/tmp/pulse-socket \
  -v $(pwd)/recordings:/app/recordings \
  --device /dev/snd \
  --device /dev/input \
  --network host \
  micapp:latest
```

---

## Usage

1. Click "Start" to begin recording
2. Click "Send" (or press Escape) to stop recording and transcribe
3. Click "Add" to append new transcription to existing text
4. Use Ctrl+Shift+Drag to capture screenshots
5. Transcribed text is automatically copied to clipboard

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | Yes | Your OpenAI API key for transcription |

## Troubleshooting

### Native Build Issues

**Build fails with CGO errors:**
- Make sure GCC is installed: `sudo apt install build-essential`
- Verify all dev libraries are installed (see prerequisites)

**Audio recording fails:**
- Check microphone permissions in system settings
- Verify PulseAudio is running: `pulseaudio --check -v`
- List audio devices: `arecord -l`

**GUI not displaying:**
- Check DISPLAY variable: `echo $DISPLAY`
- Verify X11 is running: `xhost`

### Docker Issues

**X11 connection refused:**
```bash
xhost +local:docker
```

**Audio not working:**
```bash
pactl load-module module-native-protocol-unix socket=/tmp/pulse-socket
```

**Permission denied errors:**
Make sure your user is in the `docker` group:
```bash
sudo usermod -aG docker $USER
```

## License

MIT License
