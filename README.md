# MICAPP

Voice transcription application with GUI built with Go and Fyne framework.

## Features

- Voice recording and transcription using OpenAI Whisper API
- GUI interface built with Fyne
- Audio storage with multiple bitrate options
- Text correction using LLM
- Screenshot capture functionality

## Docker Setup (Recommended)

The easiest way to run MICAPP is using Docker, which handles all dependencies automatically.

### Prerequisites

- Docker and Docker Compose installed
- X11 server running (for GUI)
- PulseAudio running (for audio)
- OpenAI API key

### Quick Start with Docker

1. **Set your OpenAI API key:**
   ```bash
   export OPENAI_API_KEY="your-api-key-here"
   ```

2. **Allow X11 connections:**
   ```bash
   xhost +local:docker
   ```

3. **Setup PulseAudio socket (for audio):**
   ```bash
   pactl load-module module-native-protocol-unix socket=/tmp/pulse-socket
   ```

4. **Run the application:**
   ```bash
   ./docker-run.sh
   ```

   Or using docker-compose directly:
   ```bash
   docker-compose up
   ```

### Building Docker Image

To build the Docker image manually:

```bash
docker-compose build
```

Or using docker directly:

```bash
docker build -t micapp:latest .
```

### Running with Docker (without docker-compose)

```bash
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

## Manual Setup (Without Docker)

If you prefer to run without Docker, you'll need to install dependencies manually.

### Prerequisites

- Go 1.23.0 or later
- Audio libraries: libasound2-dev, libpulse-dev, portaudio19-dev
- X11 libraries for GUI
- xclip, xdotool, wmctrl utilities

### Installation

1. **Install Go:**
   ```bash
   sudo apt update
   sudo apt install -y golang-go
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

5. **Set OpenAI API key:**
   ```bash
   export OPENAI_API_KEY="your-api-key-here"
   ```

6. **Install Go dependencies:**
   ```bash
   go mod download
   ```

7. **Build the application:**
   ```bash
   go build -o voicetranscriber ./code
   ```

8. **Run the application:**
   ```bash
   ./voicetranscriber
   ```

## Usage

1. Click "Start" to begin recording
2. Click "Send" (or press Escape) to stop recording and transcribe
3. Click "Add" to append new transcription to existing text
4. Use Ctrl+Shift+Drag to capture screenshots
5. Transcribed text is automatically copied to clipboard

## Environment Variables

- `OPENAI_API_KEY` - Required. Your OpenAI API key for transcription

## Troubleshooting

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

### Manual Setup Issues

**Audio recording fails:**
- Check microphone permissions in system settings
- Verify PulseAudio is running: `pulseaudio --check -v`
- List audio devices: `arecord -l`

**GUI not displaying:**
- Check DISPLAY variable: `echo $DISPLAY`
- Verify X11 is running: `xhost`

## License

MIT License
