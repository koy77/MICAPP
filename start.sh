#!/bin/bash

# VoiceTranscriber Start Script
# This script builds and runs the VoiceTranscriber application from the host machine

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
    print_status "Building VoiceTranscriber..."
    
    # Clean previous build
    if [ -f "voicetranscriber" ]; then
        rm voicetranscriber
        print_status "Removed previous build"
    fi
    
    # Build the application
    print_status "Compiling application..."
    CGO_ENABLED=1 go build -ldflags='-s -w' -o voicetranscriber ./code
    
    if [ $? -eq 0 ]; then
        # Make executable
        chmod +x voicetranscriber
        
        print_success "Application built successfully"
        
        # Show build info
        BUILD_SIZE=$(du -h voicetranscriber | cut -f1)
        print_status "Build size: $BUILD_SIZE"
    else
        print_error "Build failed"
        exit 1
    fi
}

# Function to run the application
run_app() {
    print_status "Starting VoiceTranscriber..."
    print_status "Make sure your microphone is connected and permissions are granted"
    echo ""
    
    # Check if executable exists
    if [ ! -f "voicetranscriber" ]; then
        print_error "Executable not found. Please build the application first."
        exit 1
    fi
    
    # Make executable
    chmod +x voicetranscriber
    
    # Run the application
    ./voicetranscriber
}

# Function to show help
show_help() {
    echo "VoiceTranscriber Start Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -b, --build    Only build the application (don't run)"
    echo "  -r, --run      Only run the application (don't build)"
    echo "  -c, --clean    Clean build artifacts"
    echo "  -d, --deps     Only install dependencies"
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
    echo "  $0                # Build and run"
    echo "  $0 --build        # Only build"
    echo "  $0 --run          # Only run (requires existing build)"
    echo "  $0 --clean        # Clean build artifacts"
}

# Function to clean build artifacts
clean_build() {
    print_status "Cleaning build artifacts..."
    
    if [ -f "voicetranscriber" ]; then
        rm voicetranscriber
        print_success "Removed executable"
    fi
    
    if [ -d ".voicetranscriber" ]; then
        rm -rf .voicetranscriber
        print_success "Removed application data directory"
    fi
    
    print_success "Clean completed"
}

# Main script logic
main() {
    echo "=========================================="
    echo "    VoiceTranscriber Start Script"
    echo "=========================================="
    echo ""
    
    # Parse command line arguments
    BUILD_ONLY=false
    RUN_ONLY=false
    CLEAN_ONLY=false
    DEPS_ONLY=false
    
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
            -d|--deps)
                DEPS_ONLY=true
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
    
    # Default: build and run
    check_prerequisites
    install_dependencies
    build_app
    run_app
}

# Run main function
main "$@"
