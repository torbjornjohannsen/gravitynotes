# GravityNotes

A note-taking application implementing Karpathy's "append and review" system with automatic block promotion based on recency. The core concept is **"topic gravity"** - notes that are edited automatically float to the top, reflecting current attention and importance.

## Overview

GravityNotes is a CLI-based note management system with optional Discord integration. It uses a SQLite database as the single source of truth, with a markdown file as a generated view. The system automatically organizes content by recency - whenever you edit a note block, it moves to the top.

## Architecture

```
┌─────────────────┐    ┌──────────────┐    ┌─────────────────┐
│   Markdown      │◄──►│    SQLite    │◄──►│   CLI Commands  │
│   File (.md)    │    │   Database   │    │                 │
└─────────────────┘    └──────────────┘    └─────────────────┘
         ▲                      ▲                      ▲
         │                      │                      │
         └──────── File Watcher ┴──────────────────────┘
```

## Features

### Core Note System
- **Automatic Organization**: Notes float to top when edited (topic gravity)
- **Content-based Identity**: Blocks identified by content hash, not position
- **Database Authority**: SQLite is source of truth, markdown is view
- **Real-time Sync**: File watcher automatically reconciles changes
- **Deduplication**: Identical content blocks are automatically merged

### CLI Interface
- `notes init` - Initialize new repository
- `notes add "content"` - Add new note block
- `notes grep "term"` - Search across all blocks
- `notes export` - Force regenerate markdown from database
- `notes watch` - Start file watcher (development)

### Discord Integration
- **Message Capture**: Automatically grabs messages from designated channel
- **Auto-deletion**: Removes captured messages from Discord

## Quick Start

### 1. Set Up Core System

```bash
# Build the CLI
go build -o notes ./src

# Initialize repository
./notes init

# Add your first note
./notes add "My first thought"
```

### 2. Optional: Discord Bot

```bash
# Set up Discord bot
cd discord-bot
cp .env.example .env
# Edit .env with your Discord token and channel ID

# Install dependencies
source venv/bin/activate
pip install -r requirements.txt

# Run bot
python bot.py
```

### 3. Optional: Easy server startup 
```bash 
./start-notes-system.sh 
```

## Project Structure

```
GravityNotes/
├── src/                    # Go source code
│   ├── cli.go             # Main CLI application
│   ├── database.go        # SQLite operations
│   ├── blocks.go          # Block management & hashing
│   ├── file.go            # Markdown file operations
│   ├── reconciler.go      # Sync between file and database
│   └── watcher.go         # File change monitoring
├── discord-bot/           # Discord integration
│   ├── bot.py            # Discord bot implementation
│   ├── requirements.txt  # Python dependencies
│   ├── venv/             # Python virtual environment
│   └── README.md         # Discord bot documentation
├── go.mod                # Go module definition
├── notes                 # Compiled CLI binary
├── notes.db             # SQLite database (created on init)
├── notes.md             # Generated markdown view
└── spec.md              # Original specification
```

## Core Concepts

### Block Promotion
When a block is edited, it automatically moves to the top of the document by updating its timestamp. This creates a natural organization where recently-touched content rises to the surface.

### Content Hashing
Each note block is identified by a SHA256 hash of its trimmed content. This enables:
- Automatic deduplication of identical content
- Reliable tracking across edits and file changes
- Efficient reconciliation between file and database

### Single Source of Truth
The SQLite database is authoritative. The markdown file is regenerated from the database whenever changes occur, ensuring consistency and preventing data loss.

## File Format

**Block Delimiter**: One or more consecutive empty lines  
**Content**: Markdown text with whitespace trimmed for hashing  
**Ordering**: Always timestamp descending (newest first)

Example `notes.md`:
```markdown
# Latest thought
This was just added and appears at top

Some older note
With multiple lines

Another block
From yesterday
```

## Technical Details

### Database Schema
```sql
CREATE TABLE blocks (
    id INTEGER PRIMARY KEY,
    content TEXT NOT NULL,
    content_hash TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

### Reconciliation Logic
1. Parse markdown file into blocks (split by empty lines, trim whitespace)
2. Compare with database using content hashes
3. Add new blocks with current timestamp
4. Preserve CLI-added blocks created since last reconciliation
5. Remove blocks deleted from file (unless recently added via CLI)
6. Regenerate markdown in timestamp order

## Technology Stack

- **Backend**: Go with SQLite for persistence
- **File Watching**: fsnotify for change detection
- **Discord Bot**: Python with discord.py
- **Content Hashing**: SHA256 for block identity

## Design Philosophy

1. **Simplicity First**: Solve core use case before adding features
2. **Database Authoritative**: SQLite is source of truth, markdown is view
3. **Automatic Organization**: Minimize manual categorization/filing
4. **Content-Based Identity**: Blocks identified by content hash, not position
5. **Graceful Degradation**: Handle edge cases with sensible fallbacks

## License

MIT License - Feel free to use and modify for your own note-taking needs.