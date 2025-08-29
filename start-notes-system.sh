#!/bin/bash

# GravityNotes System Launcher
# Starts both the Discord bot and notes file watcher
# Gracefully shuts down both when Ctrl+C is pressed

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NOTES_CLI="$SCRIPT_DIR/notes"
DISCORD_BOT_DIR="$SCRIPT_DIR/discord-bot"
DISCORD_BOT_SCRIPT="$DISCORD_BOT_DIR/bot.py"
DISCORD_VENV="$DISCORD_BOT_DIR/venv"

# PIDs of background processes
WATCHER_PID=""
DISCORD_PID=""

# Function to print colored output
print_status() {
    echo -e "${BLUE}[LAUNCHER]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Function to check if required files exist
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    if [ ! -f "$NOTES_CLI" ]; then
        print_error "Notes CLI not found at: $NOTES_CLI"
        print_error "Please build it first: go build -o notes ./src"
        exit 1
    fi
    
    if [ ! -f "$DISCORD_BOT_SCRIPT" ]; then
        print_error "Discord bot script not found at: $DISCORD_BOT_SCRIPT"
        exit 1
    fi
    
    if [ ! -d "$DISCORD_VENV" ]; then
        print_error "Discord bot virtual environment not found at: $DISCORD_VENV"
        print_error "Please set it up first: cd discord-bot && python3 -m venv venv && source venv/bin/activate && pip install -r requirements.txt"
        exit 1
    fi
    
    if [ ! -f "$DISCORD_BOT_DIR/.env" ]; then
        print_warning "Discord bot .env file not found. Copy .env.example to .env and configure it."
        print_warning "Bot will not start without proper configuration."
    fi
    
    print_success "All prerequisites check passed!"
}

# Function to start the notes file watcher
start_notes_watcher() {
    print_status "Starting notes file watcher..."
    
    cd "$SCRIPT_DIR"
    "$NOTES_CLI" watch &
    WATCHER_PID=$!
    
    # Give it a moment to start
    sleep 1
    
    if kill -0 "$WATCHER_PID" 2>/dev/null; then
        print_success "Notes watcher started (PID: $WATCHER_PID)"
    else
        print_error "Failed to start notes watcher"
        exit 1
    fi
}

# Function to start the Discord bot
start_discord_bot() {
    print_status "Starting Discord bot..."
    
    cd "$DISCORD_BOT_DIR"
    source "$DISCORD_VENV/bin/activate"
    python bot.py &
    DISCORD_PID=$!
    
    # Give it a moment to start
    sleep 2
    
    if kill -0 "$DISCORD_PID" 2>/dev/null; then
        print_success "Discord bot started (PID: $DISCORD_PID)"
    else
        print_error "Failed to start Discord bot"
        print_error "Check discord-bot/discord-bot.log for details"
        cleanup
        exit 1
    fi
}

# Function to cleanup and kill background processes
cleanup() {
    print_status "Shutting down GravityNotes system..."
    
    if [ -n "$DISCORD_PID" ] && kill -0 "$DISCORD_PID" 2>/dev/null; then
        print_status "Stopping Discord bot (PID: $DISCORD_PID)..."
        kill -TERM "$DISCORD_PID" 2>/dev/null || true
        
        # Wait up to 5 seconds for graceful shutdown
        for i in {1..5}; do
            if ! kill -0 "$DISCORD_PID" 2>/dev/null; then
                break
            fi
            sleep 1
        done
        
        # Force kill if still running
        if kill -0 "$DISCORD_PID" 2>/dev/null; then
            print_warning "Force killing Discord bot..."
            kill -KILL "$DISCORD_PID" 2>/dev/null || true
        fi
        
        print_success "Discord bot stopped"
    fi
    
    if [ -n "$WATCHER_PID" ] && kill -0 "$WATCHER_PID" 2>/dev/null; then
        print_status "Stopping notes watcher (PID: $WATCHER_PID)..."
        kill -TERM "$WATCHER_PID" 2>/dev/null || true
        
        # Wait up to 5 seconds for graceful shutdown
        for i in {1..5}; do
            if ! kill -0 "$WATCHER_PID" 2>/dev/null; then
                break
            fi
            sleep 1
        done
        
        # Force kill if still running
        if kill -0 "$WATCHER_PID" 2>/dev/null; then
            print_warning "Force killing notes watcher..."
            kill -KILL "$WATCHER_PID" 2>/dev/null || true
        fi
        
        print_success "Notes watcher stopped"
    fi
    
    print_success "GravityNotes system shutdown complete"
}

# Function to handle signals (Ctrl+C, etc.)
signal_handler() {
    echo # New line after ^C
    print_status "Received shutdown signal..."
    cleanup
    exit 0
}

# Function to monitor processes and restart if they die
monitor_processes() {
    while true; do
        # Check notes watcher
        if [ -n "$WATCHER_PID" ] && ! kill -0 "$WATCHER_PID" 2>/dev/null; then
            print_error "Notes watcher died unexpectedly!"
            print_status "Attempting to restart notes watcher..."
            start_notes_watcher
        fi
        
        # Check Discord bot
        if [ -n "$DISCORD_PID" ] && ! kill -0 "$DISCORD_PID" 2>/dev/null; then
            print_error "Discord bot died unexpectedly!"
            print_status "Attempting to restart Discord bot..."
            start_discord_bot
        fi
        
        sleep 5
    done
}

# Main function
main() {
    print_status "Starting GravityNotes System Launcher"
    print_status "Press Ctrl+C to stop both services"
    echo
    
    # Set up signal handlers
    trap signal_handler SIGINT SIGTERM
    
    # Check prerequisites
    check_prerequisites
    echo
    
    # Start services
    #start_notes_watcher
    start_discord_bot
    echo
    
    print_success "Both services are running!"
    print_status "Notes watcher: PID $WATCHER_PID"
    print_status "Discord bot: PID $DISCORD_PID"
    echo
    print_status "System is ready! Send a DM to your Discord bot to test."
    print_status "Notes will automatically sync when you edit notes.md"
    echo
    print_status "Monitoring processes... (Press Ctrl+C to stop)"
    
    # Monitor and keep running
    monitor_processes
}

# Run main function
main "$@"