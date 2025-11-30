#!/bin/bash

# VoiceTranscriber Build and Run Script
# This script builds the application and runs it with proper environment setup

set -e  # Exit on any error

echo "🎤 VoiceTranscriber Build and Run Script"
echo "========================================"

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo "❌ Error: go.mod not found. Please run this script from the MICAPP directory."
    exit 1
fi

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed. Please install Go first."
    exit 1
fi

# Check if OpenAI API key is set
if [ -z "$OPENAI_API_KEY" ]; then
    echo "⚠️  Warning: OPENAI_API_KEY environment variable is not set."
    echo "   The application will not work without it."
    echo "   Set it with: export OPENAI_API_KEY='your-api-key-here'"
    echo ""
    read -p "Do you want to continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Exiting..."
        exit 1
    fi
fi

# Clean previous build
echo "🧹 Cleaning previous build..."
rm -f voicetranscriber

# Build the application
echo "🔨 Building VoiceTranscriber..."
go build -o voicetranscriber code/*.go

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo "📁 Binary created: voicetranscriber"
    echo ""
    
    # Make executable
    chmod +x voicetranscriber
    
    # Show file info
    echo "📊 File information:"
    ls -lh voicetranscriber
    echo ""
    
    # Ask if user wants to run the application
    read -p "🚀 Do you want to run the application now? (Y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        echo "Build complete. Run with: ./voicetranscriber"
    else
        echo "🎤 Starting VoiceTranscriber..."
        echo "========================================"
        ./voicetranscriber
    fi
else
    echo "❌ Build failed!"
    exit 1
fi

