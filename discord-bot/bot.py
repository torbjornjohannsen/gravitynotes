#!/usr/bin/env python3
"""
Discord bot that monitors a specific channel, captures messages, deletes them,
and adds them to the GravityNotes system.
"""

import os
import sys
import asyncio
import logging
import subprocess
from pathlib import Path
from datetime import datetime

import discord
from discord.ext import commands
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('discord-bot.log'),
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

# Bot configuration
DISCORD_TOKEN = os.getenv('DISCORD_TOKEN')
CHANNEL_ID = int(os.getenv('CHANNEL_ID', 0))
NOTES_CLI_PATH = os.getenv('NOTES_CLI_PATH', '../notes')

if not DISCORD_TOKEN:
    logger.error("DISCORD_TOKEN environment variable is required")
    sys.exit(1)

if not CHANNEL_ID:
    logger.error("CHANNEL_ID environment variable is required")
    sys.exit(1)

# Configure bot intents
intents = discord.Intents.default()
intents.message_content = True
intents.messages = True

# Create bot instance
bot = commands.Bot(command_prefix='!', intents=intents)

def add_note_to_system(content: str, author: str) -> bool:
    """
    Add a note to the GravityNotes system using the CLI.
    
    Args:
        content: The message content to add
        author: The author of the message
        
    Returns:
        bool: True if successful, False otherwise
    """
    try:
        # Get the absolute path to the notes CLI
        cli_path = Path(__file__).parent / NOTES_CLI_PATH
        cli_path = cli_path.resolve()
        
        if not cli_path.exists():
            logger.error(f"Notes CLI not found at: {cli_path}")
            return False
        
        # Execute the notes add command
        result = subprocess.run(
            [str(cli_path), 'add', content],
            capture_output=True,
            text=True,
            timeout=30,
            cwd=cli_path.parent
        )
        
        if result.returncode == 0:
            logger.info(f"Successfully added note from {author}: {content[:100]}...")
            return True
        else:
            logger.error(f"Failed to add note. Exit code: {result.returncode}")
            logger.error(f"Stderr: {result.stderr}")
            return False
            
    except subprocess.TimeoutExpired:
        logger.error("Notes CLI command timed out")
        return False
    except Exception as e:
        logger.error(f"Error adding note to system: {e}")
        return False

@bot.event
async def on_ready():
    """Called when the bot has successfully connected to Discord."""
    logger.info(f"Bot logged in as {bot.user.name} (ID: {bot.user.id})")
    
    # Verify the target channel exists and is accessible
    channel = bot.get_channel(CHANNEL_ID)
    if channel:
        logger.info(f"Monitoring channel: #{channel.name} (ID: {CHANNEL_ID})")
    else:
        logger.error(f"Could not access channel with ID: {CHANNEL_ID}")
        logger.error("Make sure the bot has permission to view the channel")

@bot.event
async def on_message(message):
    """
    Handle incoming messages. Capture and delete messages from the target channel.
    """
    # Ignore messages from the bot itself
    if message.author == bot.user:
        return
    
    # Only process messages from the target channel
    if message.channel.id != CHANNEL_ID:
        return
    
    # Skip empty messages
    if not message.content.strip():
        logger.info(f"Skipping empty message from {message.author.display_name}")
        return
    
    logger.info(f"Processing message from {message.author.display_name}: {message.content[:100]}...")
    
    try:
        # Add the message to the notes system
        success = add_note_to_system(message.content, message.author.display_name)
        
        if success:
            # Delete the original message
            await message.delete()
            logger.info(f"Message captured and deleted successfully")
        else:
            logger.error("Failed to add note - message not deleted")
            
    except discord.Forbidden:
        logger.error("Bot doesn't have permission to delete messages in this channel")
    except discord.NotFound:
        logger.warning("Message was already deleted")
    except Exception as e:
        logger.error(f"Error processing message: {e}")

@bot.event
async def on_error(event, *args, **kwargs):
    """Handle bot errors."""
    logger.error(f"Bot error in event {event}: {args}, {kwargs}")

@bot.command(name='ping')
async def ping_command(ctx):
    """Simple ping command to test bot responsiveness."""
    await ctx.send('Pong! Bot is running.')

@bot.command(name='status')
async def status_command(ctx):
    """Display bot status and configuration."""
    if ctx.channel.id != CHANNEL_ID:
        await ctx.send("This command only works in the monitored channel.")
        return
    
    embed = discord.Embed(
        title="Discord Notes Bot Status",
        color=0x00ff00,
        timestamp=datetime.now()
    )
    embed.add_field(name="Monitored Channel", value=f"<#{CHANNEL_ID}>", inline=False)
    embed.add_field(name="Notes CLI", value=NOTES_CLI_PATH, inline=False)
    embed.add_field(name="Status", value="✅ Active", inline=False)
    
    await ctx.send(embed=embed)

async def main():
    """Main function to run the bot."""
    try:
        logger.info("Starting Discord bot...")
        await bot.start(DISCORD_TOKEN)
    except KeyboardInterrupt:
        logger.info("Bot shutdown requested")
    except Exception as e:
        logger.error(f"Bot crashed: {e}")
    finally:
        if not bot.is_closed():
            await bot.close()
        logger.info("Bot shutdown complete")

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("Bot interrupted by user")
    except Exception as e:
        logger.error(f"Fatal error: {e}")
        sys.exit(1)