# Discord Notes Bot

A Discord bot that automatically captures messages from a specific channel, deletes them, and adds them to the GravityNotes system.

## Features

- **Channel Monitoring**: Watches a specific Discord channel for new messages
- **Automatic Capture**: Captures message content with author and timestamp
- **Message Deletion**: Automatically deletes captured messages from Discord
- **Notes Integration**: Adds captured messages to GravityNotes using the CLI
- **Logging**: Comprehensive logging for debugging and monitoring
- **Error Handling**: Robust error handling for Discord API and CLI integration

## Setup

### 1. Prerequisites

- Python 3.8 or higher
- GravityNotes CLI built and available (see parent directory)
- Discord bot token and permissions

### 2. Create Discord Bot

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Click "New Application" and give it a name
3. Go to the "Bot" section and click "Add Bot"
4. Copy the bot token for later use
5. Enable "Message Content Intent" under "Privileged Gateway Intents"

### 3. Bot Permissions

The bot needs the following permissions in your Discord server:
- `View Channels`
- `Send Messages`
- `Manage Messages` (to delete messages)
- `Read Message History`

### 4. Install Dependencies

```bash
cd discord-bot
source venv/bin/activate  # On Windows: venv\\Scripts\\activate
pip install -r requirements.txt
```

### 5. Configuration

1. Copy the environment template:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` and fill in your values:
   ```env
   DISCORD_TOKEN=your_actual_bot_token_here
   CHANNEL_ID=123456789012345678
   NOTES_CLI_PATH=../notes
   ```

To get the Channel ID:
- Enable Developer Mode in Discord (User Settings → Advanced → Developer Mode)
- Right-click on the target channel and select "Copy ID"

### 6. Running the Bot

```bash
# Activate virtual environment
source venv/bin/activate

# Run the bot
python bot.py
```

The bot will start monitoring the specified channel and log activity to both console and `discord-bot.log`.

## Usage

Once running, the bot will:

1. **Monitor Messages**: Watch the configured channel for new messages
2. **Capture Content**: Save message content with format: `[Discord - Username - Timestamp] message content`
3. **Delete Original**: Remove the message from Discord after successful capture
4. **Add to Notes**: Execute `../notes add "captured content"` to store in GravityNotes

### Bot Commands

- `!ping` - Test bot responsiveness
- `!status` - Display bot status and configuration (only works in monitored channel)

## File Structure

```
discord-bot/
├── venv/              # Python virtual environment
├── bot.py             # Main bot implementation
├── requirements.txt   # Python dependencies
├── .env.example       # Configuration template
├── .env              # Your actual configuration (create this)
├── discord-bot.log   # Bot log file (created when running)
└── README.md         # This file
```

## Troubleshooting

### Common Issues

1. **"DISCORD_TOKEN environment variable is required"**
   - Ensure `.env` file exists and contains your bot token

2. **"Could not access channel with ID: X"**
   - Verify the channel ID is correct
   - Ensure the bot has been added to your server
   - Check that the bot has permission to view the channel

3. **"Bot doesn't have permission to delete messages"**
   - Ensure the bot has "Manage Messages" permission in the target channel

4. **"Notes CLI not found"**
   - Verify the `NOTES_CLI_PATH` in `.env` points to the correct location
   - Ensure the GravityNotes CLI has been built (`go build -o notes ./src`)

### Logging

The bot logs to both console and `discord-bot.log`. Check the log file for detailed error information:

```bash
tail -f discord-bot.log
```

## Development

To modify the bot behavior:

1. Edit `bot.py` for core functionality
2. The `add_note_to_system()` function handles CLI integration
3. The `on_message()` event handles message processing
4. Add new commands using the `@bot.command()` decorator

## Security Notes

- Keep your `.env` file secure and never commit it to version control
- The bot token provides full access to your bot - treat it like a password
- Consider running the bot on a dedicated server or VPS for 24/7 operation