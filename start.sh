#!/bin/bash

# VoiceTranscriber Start Script
# This script builds and runs the VoiceTranscriber application

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
    
    # Check if Docker is installed
    if ! command_exists docker; then
        print_error "Docker is not installed. Please install Docker first."
        echo "Installation instructions:"
        echo "  https://docs.docker.com/get-docker/"
        exit 1
    fi
    
    # Check if Docker is running
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker daemon."
        exit 1
    fi
    
    print_success "Docker is installed and running"
    
    # Check if we're in the correct directory
    if [ ! -d "code" ]; then
        print_error "Code directory not found. Please run this script from the project root directory."
        exit 1
    fi
    
    if [ ! -f "Dockerfile" ]; then
        print_error "Dockerfile not found. Please run this script from the project root directory."
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
    print_status "Installing Go dependencies using Docker..."
    
    if [ ! -f "go.mod" ]; then
        print_error "go.mod not found. Are you in the correct directory?"
        exit 1
    fi
    
    # Use Docker to install dependencies
    docker run --rm \
        -v "$(pwd):/build" \
        -w /build \
        golang:1.23-bullseye \
        go mod download
    
    if [ $? -eq 0 ]; then
        print_success "Dependencies installed successfully"
    else
        print_error "Failed to install dependencies"
        exit 1
    fi
}

# Function to build the application
build_app() {
    print_status "Building VoiceTranscriber using Docker..."
    
    # Clean previous build
    if [ -f "voicetranscriber" ]; then
        rm voicetranscriber
        print_status "Removed previous build"
    fi
    
    # Check if builder image exists, if not build it
    if ! docker images | grep -q "micapp-builder"; then
        print_status "Building Docker builder image..."
        docker build --target builder -t micapp-builder:latest -f Dockerfile .
        if [ $? -ne 0 ]; then
            print_error "Failed to build Docker builder image"
            exit 1
        fi
    fi
    
    # Build the application using Docker
    print_status "Compiling application in Docker container..."
    docker run --rm \
        -v "$(pwd):/build" \
        -w /build \
        micapp-builder:latest \
        sh -c "CGO_ENABLED=1 GOOS=linux go build -ldflags='-s -w' -o voicetranscriber ./code"
    
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
    echo "  -b, --build    Only build the application using Docker (don't run)"
    echo "  -r, --run      Only run the application (don't build)"
    echo "  -c, --clean    Clean build artifacts"
    echo "  -d, --deps     Only install dependencies using Docker"
    echo ""
    echo "Environment Variables:"
    echo "  OPENAI_API_KEY    Your OpenAI API key (required)"
    echo ""
    echo "Note: This script uses Docker for building. Make sure Docker is installed and running."
    echo ""
    echo "Examples:"
    echo "  $0                # Build (using Docker) and run"
    echo "  $0 --build        # Only build using Docker"
    echo "  $0 --run          # Only run (requires existing build)"
    echo "  $0 --clean        # Clean and rebuild"
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
    
    # Optionally clean Docker builder image
    read -p "Do you want to remove Docker builder image? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        docker rmi micapp-builder:latest 2>/dev/null || true
        print_success "Removed Docker builder image"
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
