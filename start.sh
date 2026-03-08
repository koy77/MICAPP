#!/bin/bash

# MICAPP Start Script
# This script builds and runs the MICAPP application from the host machine

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
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

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    # Check if Go is installed
    if ! command_exists go; then
        print_error "Go is not installed. Please install Go first."
        echo "Installation instructions:"
        echo "  https://go.dev/doc/install"
        echo ""
        echo "Quick install on Linux:"
        echo "  wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz"
        echo "  sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz"
        echo "  export PATH=\$PATH:/usr/local/go/bin"
        exit 1
    fi
    
    GO_VERSION=$(go version)
    print_success "Go is installed: $GO_VERSION"
    
    # Check if we're in the correct directory
    if [ ! -d "code" ]; then
        print_error "Code directory not found. Please run this script from the project root directory."
        exit 1
    fi
    
    if [ ! -f "go.mod" ]; then
        print_error "go.mod not found. Please run this script from the project root directory."
        exit 1
    fi
    
    # Check if OPENAI_API_KEY is set
    if [ -z "$OPENAI_API_KEY" ]; then
        print_warning "OPENAI_API_KEY environment variable is not set."
        echo "Please set your OpenAI API key:"
        echo "  export OPENAI_API_KEY=\"your-api-key-here\""
        echo ""
        echo "Or add it to your ~/.bashrc for persistence:"
        echo "  echo 'export OPENAI_API_KEY=\"your-api-key-here\"' >> ~/.bashrc"
        echo "  source ~/.bashrc"
        echo ""
        read -p "Do you want to continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    else
        print_success "OPENAI_API_KEY is set"
    fi
}

# Function to install dependencies
install_dependencies() {
    print_status "Installing Go dependencies..."
    
    if [ ! -f "go.mod" ]; then
        print_error "go.mod not found. Are you in the correct directory?"
        exit 1
    fi
    
    go mod tidy
    
    if [ $? -eq 0 ]; then
        print_success "Dependencies installed successfully"
    else
        print_error "Failed to install dependencies"
        exit 1
    fi
}

# Function to build the application
build_app() {
    print_status "Building MICAPP..."
    
    # Clean previous build
    if [ -f "micapp" ]; then
        rm micapp
        print_status "Removed previous build"
    fi
    
    # Build the application
    print_status "Compiling application..."
    CGO_ENABLED=1 go build -ldflags='-s -w' -o micapp ./code
    
    if [ $? -eq 0 ]; then
        # Make executable
        chmod +x micapp
        
        print_success "Application built successfully"
        
        # Show build info
        BUILD_SIZE=$(du -h micapp | cut -f1)
        print_status "Build size: $BUILD_SIZE"
    else
        print_error "Build failed"
        exit 1
    fi
}

# Function to install the application to the system
install_app() {
    print_status "Installing MICAPP to the system..."
    
    # Save OPENAI_API_KEY to .env if it exists in current session
    if [ ! -z "$OPENAI_API_KEY" ]; then
        echo "OPENAI_API_KEY=$OPENAI_API_KEY" > .env
        print_success "Saved OPENAI_API_KEY to .env"
    fi

    # Create desktop entry
    DESKTOP_FILE="$HOME/.local/share/applications/micapp.desktop"
    mkdir -p "$(dirname "$DESKTOP_FILE")"
    
    cat > "$DESKTOP_FILE" <<EOF
[Desktop Entry]
Name=MICAPP
Comment=Voice Transcription Tool
Exec=$(pwd)/micapp
Icon=$(pwd)/red_cube_icon.png
Path=$(pwd)
Terminal=false
Type=Application
Categories=Utility;
StartupNotify=true
EOF
    
    chmod +x "$DESKTOP_FILE"
    
    # Convert SVG icon to PNG if needed
    if [ -f "red_cube_icon.svg" ] && [ ! -f "red_cube_icon.png" ]; then
        if command_exists convert; then
            convert red_cube_icon.svg red_cube_icon.png
        fi
    fi
    
    print_success "MICAPP installed to $DESKTOP_FILE"
    print_status "It should now appear in your Ubuntu Applications list."
}

# Function to run the application
run_app() {
    print_status "Starting MICAPP and recording dot..."
    print_status "Make sure your microphone is connected and permissions are granted"
    echo ""
    
    # Kill any existing instances
    if pgrep -x "micapp" > /dev/null; then
        print_warning "Found existing MICAPP processes. Killing them..."
        pkill -x "micapp" || true
        sleep 1
    fi

    if pgrep -f "recording-dot.py" > /dev/null; then
        print_warning "Found existing recording-dot processes. Killing them..."
        pkill -f "recording-dot.py" || true
    fi

    # Check if executable exists
    if [ ! -f "micapp" ]; then
        print_error "Executable not found. Please build the application first."
        exit 1
    fi
    
    # Make executable
    chmod +x micapp
    
    # Run the application
    ./micapp
}

# Function to show help
show_help() {
    echo "MICAPP Start Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -b, --build    Only build the application (don't run/install)"
    echo "  -r, --run      Only run the application (don't build/install)"
    echo "  -i, --install  Build and install desktop entry (show in Applications)"
    echo "  -d, --deploy   Clean, build, and reinstall fully (for system menu)"
    echo "  -c, --clean    Clean build artifacts"
    echo "  --deps         Only install dependencies"
    echo ""
    echo "Environment Variables:"
    echo "  OPENAI_API_KEY    Your OpenAI API key (required)"
    echo ""
    echo "Prerequisites:"
    echo "  - Go 1.23+ installed"
    echo "  - CGO dependencies (gcc, pkg-config)"
    echo "  - PortAudio development libraries"
    echo "  - X11 development libraries (for Linux)"
    echo ""
    echo "Examples:"
    echo "  $0                # Build, install desktop entry, and run"
    echo "  $0 --install      # Build and install desktop entry"
    echo "  $0 --deploy       # Full clean reinstall for system menu"
    echo "  $0 --build        # Only build"
    echo "  $0 --run          # Only run (requires existing build)"
    echo "  $0 --clean        # Clean build artifacts"
}

# Function to clean build artifacts
clean_build() {
    print_status "Cleaning build artifacts..."
    
    if [ -f "micapp" ]; then
        rm micapp
        print_success "Removed executable"
    fi
    
    if [ -f "red_cube_icon.png" ]; then
        rm red_cube_icon.png
        print_success "Removed generated icon"
    fi
    
    if [ -d ".micapp" ]; then
        rm -rf .micapp
        print_success "Removed application data directory"
    fi
    
    # Remove desktop entry
    DESKTOP_FILE="$HOME/.local/share/applications/micapp.desktop"
    if [ -f "$DESKTOP_FILE" ]; then
        rm "$DESKTOP_FILE"
        print_success "Removed desktop entry"
    fi
    
    print_success "Clean completed"
}

# Main script logic
main() {
    echo "=========================================="
    echo "    MICAPP Start Script"
    echo "=========================================="
    echo ""
    
    # Parse command line arguments
    BUILD_ONLY=false
    RUN_ONLY=false
    CLEAN_ONLY=false
    DEPS_ONLY=false
    INSTALL_ONLY=false
    DEPLOY_ONLY=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -b|--build)
                BUILD_ONLY=true
                shift
                ;;
            -r|--run)
                RUN_ONLY=true
                shift
                ;;
            -c|--clean)
                CLEAN_ONLY=true
                shift
                ;;
            -d|--deploy)
                DEPLOY_ONLY=true
                shift
                ;;
            --deps)
                DEPS_ONLY=true
                shift
                ;;
            -i|--install)
                INSTALL_ONLY=true
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # Execute based on options
    if [ "$CLEAN_ONLY" = true ]; then
        clean_build
        exit 0
    fi
    
    if [ "$DEPS_ONLY" = true ]; then
        check_prerequisites
        install_dependencies
        exit 0
    fi
    
    if [ "$DEPLOY_ONLY" = true ]; then
        print_status "Starting FULL DEPLOYMENT (clean + build + install)..."
        clean_build
        check_prerequisites
        install_dependencies
        build_app
        install_app
        print_success "FULL DEPLOYMENT completed! MICAPP is now available in your system menu."
        exit 0
    fi
    
    if [ "$INSTALL_ONLY" = true ]; then
        check_prerequisites
        install_dependencies
        build_app
        install_app
        exit 0
    fi

    if [ "$RUN_ONLY" = true ]; then
        run_app
        exit 0
    fi
    
    if [ "$BUILD_ONLY" = true ]; then
        check_prerequisites
        install_dependencies
        build_app
        exit 0
    fi
    
    # Default: build, install (desktop entry), and run
    check_prerequisites
    install_dependencies
    build_app
    install_app
    run_app
}

# Run main function
main "$@"
