# MICAPP

Voice transcription application with GUI built with Go and Fyne framework.

## Features

- Voice recording and transcription via switchable providers: **Deepgram (nova-3)**, **Groq (whisper-large-v3-turbo)** or **OpenAI Whisper**
- GUI interface built with Fyne
- Audio storage with rotation (keeps last 10 recordings)
- WebSocket API for real-time text updates (`ws://localhost:8989/ws`)
- Global hotkey **Alt + Q** to start/stop recording
- Always-on-top window behavior
- Text correction using LLM
- Screenshot capture functionality
- Crash/perf diagnostics: `crash.log`, `.micapp.session`, ms timings in `app.log`, `perf-report.sh`

## WebSocket API

MICAPP includes a WebSocket server for real-time text updates.

- **URL**: `ws://localhost:8989/ws`
- **Protocol**: Simple JSON events

### Event: `text_update`
Sent whenever the transcription is updated.

```json
{
  "type": "text_update",
  "data": {
    "text": "The full current text content...",
    "mode": "start" // or "add"
  }
}
```

## Global Hotkeys

- **Alt + Q**: Start/Stop recording from any application.
- **Ctrl + Alt + Mouse Drag**: Capture a screenshot of a selected area.
- **Escape**: Cancel current recording or processing.

## Window Behavior

- **Always on Top**: The application window stays above other windows.
- **Fixed Size**: The window size is fixed (300x700) and will not expand.
- **Auto-Positioning**: Automatically moves to the top-left (0, 200) on startup.

## Quick Start

### Configure transcription (`.env` in the project root):

```ini
TRANSCRIBE_PROVIDER=deepgram        # deepgram | groq | openai
DEEPGRAM_API_KEY=your-key-here      # https://console.deepgram.com
# GROQ_API_KEY=...                  # https://console.groq.com/keys
# OPENAI_API_KEY=...                # https://platform.openai.com/api-keys
```

The app loads `.env` at startup (variables already present in the environment win).
If the selected provider has no key, the app logs a warning and automatically
falls back to another configured provider instead of failing to start.

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
  -e TRANSCRIBE_PROVIDER=$TRANSCRIBE_PROVIDER \
  -e DEEPGRAM_API_KEY=$DEEPGRAM_API_KEY \
  -e GROQ_API_KEY=$GROQ_API_KEY \
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
4. Use Ctrl+Alt+Drag to capture screenshots
5. Transcribed text is automatically copied to clipboard

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `TRANSCRIBE_PROVIDER` | No | `deepgram` \| `groq` \| `openai` (default: `openai`) |
| `DEEPGRAM_API_KEY` | for `deepgram` | Deepgram API key (model `nova-3`, ~$0.0043/min pre-recorded) |
| `GROQ_API_KEY` | for `groq` | Groq API key (model `whisper-large-v3-turbo`, ~$0.04/audio-hour) |
| `OPENAI_API_KEY` | for `openai` | OpenAI API key (model `whisper-1`) |

Optional per-provider overrides: `DEEPGRAM_MODEL`, `DEEPGRAM_BASE_URL`,
`GROQ_TRANSCRIBE_MODEL`, `GROQ_BASE_URL`, `OPENAI_TRANSCRIBE_MODEL`, `OPENAI_BASE_URL`.

Switching providers = one line in `.env` + restart the app.

## Diagnostics & Logging

- `app.log` — per-run log; `PERF [capture #N] …` lines carry millisecond timings for the whole capture → editor → visible pipeline.
- `crash.log` — panics (with stack traces), fatal signals, slow editor opens, input-queue backlog, xclip timeouts. `kill -USR1 <pid>` dumps all goroutine stacks into it on demand.
- `.micapp.session` — clean-exit marker; the next start reports whether the previous session crashed or was killed.
- `./perf-report.sh` — compact summary of the diagnostics above.

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
