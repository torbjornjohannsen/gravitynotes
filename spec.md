# Note-Taking App Specification

## Overview

A note-taking application implementing Karpathy's "append and review" system with automatic block promotion based on recency. The core concept is "topic gravity" - notes that are edited automatically float to the top, reflecting current attention and importance.

## Phase 1: CLI + Local Storage

### Core Concepts

**Blocks**: Individual notes represented internally with:
- Content (markdown text)
- Created/last-edited timestamp
- Content hash (for change detection)

**Block Promotion**: When a block is edited, it automatically moves to the top of the document by updating its timestamp to the current time.

**Single Source of Truth**: SQLite database is authoritative. Markdown file is a generated view.

### Architecture

```
┌─────────────────┐    ┌──────────────┐    ┌─────────────────┐
│   Markdown      │◄──►│    SQLite    │◄──►│   CLI Commands  │
│   File (.md)    │    │   Database   │    │                 │
└─────────────────┘    └──────────────┘    └─────────────────┘
         ▲                      ▲                      ▲
         │                      │                      │
         └──────── File Watcher ┴──────────────────────┘
```

### File Format

**Block Delimiter**: One or more consecutive empty lines
**Content**: Markdown text with whitespace trimmed for hashing
**Ordering**: Always timestamp descending (newest first)

**Example:**
```markdown
# Latest thought
This was just added and appears at top

Some older note
With multiple lines

Another block
From yesterday
```

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

**On markdown file change:**

1. Parse file into blocks (split by empty line delimiter, trim whitespace)
2. Get `last_reconciliation_time` from database metadata
3. For each parsed block:
   - If `content_hash` exists in DB: Keep existing timestamp (unchanged)
   - If `content_hash` doesn't exist: Create new block with current timestamp
4. For each DB block not found in parsed file:
   - If `created_at > last_reconciliation_time`: Preserve (CLI-added since last save)
   - Else: Delete (user removed from file)
5. Update `last_reconciliation_time` to current time
6. Regenerate markdown file in timestamp order

### CLI Interface

**Basic Commands:**
```bash
notes add "New note content"     # Add new block (prepended)
notes grep "search term"         # Search across all blocks  
notes export                     # Force regenerate .md from DB
notes init                       # Initialize new repository
```

**File Operations:**
- Watches `notes.md` for changes
- Stores `notes.db` in same directory
- Auto-reconciliation on file modification

### Edge Cases & Behavior

**Duplicate Content**: Blocks with identical content (after trimming) are treated as the same block. If user creates duplicate, it gets deduplicated with current timestamp.

**Empty Blocks**: Not allowed - empty content after trimming is ignored during parsing.

**Large Blocks**: No size limits. Large blocks that span multiple topics are a feature - they indicate sustained attention.

**Editing Behavior**: Any content change (even minor) results in new timestamp and promotion to top.

**CLI During Editing**: If CLI adds block while user has unsaved changes, both the CLI block and user's new content are preserved during reconciliation.

**Whitespace**: 
- Multiple consecutive empty lines treated as single delimiter
- Leading/trailing whitespace stripped from block content
- Regenerated file uses exactly one empty line between blocks

### Manual Reordering

Manual reordering via markdown editing is **not preserved**. File regeneration always enforces timestamp-based ordering. This is intentional - the system automates organization, manual intervention defeats the purpose.

### Toggle Feature (Future)

Placeholder for toggle functionality to allow editing without timestamp promotion. Implementation deferred to avoid complexity.

## Future Extensions (Phase 2+)

### Networking & Sync
- Go backend API with session-based authentication  
- Email/password + 2FA (TOTP)
- SendGrid integration for email delivery
- CLI sync commands to push/pull from server

### Web Interface
- Read-only web view for mobile
- Write-only interface for quick note capture
- CodeMirror-based editor for desktop editing

### Integrations
- Discord bot for message capture
- P2P sync via WebRTC (local network optimization)
- API endpoints for third-party integrations

### Deployment
- Single Go binary deployment
- Caddy for TLS termination
- SQLite for persistence (sufficient for single-user)
- Systemd service management

## Technology Stack

**Phase 1:**
- Go for CLI application
- SQLite for local storage
- File system watching for change detection

**Future Phases:**
- Go backend API
- CodeMirror 6 for web editing
- Caddy for reverse proxy/TLS
- Optional: WebRTC for P2P sync

## Design Principles

1. **Simplicity First**: Solve core use case before adding features
2. **Database Authoritative**: SQLite is source of truth, markdown is view
3. **Automatic Organization**: Minimize manual categorization/filing
4. **Content-Based Identity**: Blocks identified by content hash, not position
5. **Graceful Degradation**: Handle edge cases with sensible fallbacks

## Non-Goals

- Collaborative editing (single user at a time)
- Complex formatting beyond markdown
- Version history/git integration
- Mobile native apps
- Real-time synchronization
- Manual organization features (folders, tags, etc.)