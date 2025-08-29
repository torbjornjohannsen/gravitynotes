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
NOTES_CHANNEL_ID = int(os.getenv('NOTES_CHANNEL_ID', 0))
COMMAND_CHANNEL_ID = int(os.getenv('COMMAND_CHANNEL_ID', 0))
NOTES_CLI_PATH = os.getenv('NOTES_CLI_PATH', '../notes')

if not DISCORD_TOKEN:
    logger.error("DISCORD_TOKEN environment variable is required")
    sys.exit(1)

if not NOTES_CHANNEL_ID:
    logger.error("NOTES_CHANNEL_ID environment variable is required")
    sys.exit(1)

if not COMMAND_CHANNEL_ID:
    logger.error("NOTES_CHANNEL_ID environment variable is required")
    sys.exit(1)

# Configure bot intents
intents = discord.Intents.default()
intents.message_content = True
intents.messages = True

# Create bot instance
bot = commands.Bot(command_prefix='!', intents=intents)

def execute_command_in_CLI(cmd: str, content: str) -> bool:
    """
    Execute a command using the GravityNotes CLI.
    
    Args:
        cmd: The command to run 
        content: The message content to add
        
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
            [str(cli_path), cmd, content],
            capture_output=True,
            text=True,
            timeout=30,
            cwd=cli_path.parent
        )

        if result.returncode == 0:
            logger.info(f"Successfully executed {cmd}: {content[:100]}...")
            return True
        else:
            logger.error(f"Failed to execute command. Exit code: {result.returncode}")
            logger.error(f"Stderr: {result.stderr}")
            return False
            
    except subprocess.TimeoutExpired:
        logger.error("Notes CLI command timed out")
        return False
    except Exception as e:
        logger.error(f"Error executing {cmd}: {e}")
        return False

async def process_note_message(message, channel_name: str) -> bool:
    """
    Process a message's content and add it to the notes system.
    
    Args:
        message: Discord message object
        channel_name: Name of the channel for logging
        
    Returns:
        bool: True if successfully processed and should be deleted, False otherwise
    """
    # Skip messages from the bot itself
    if message.author == bot.user:
        return False
    
    # Skip empty messages
    if not message.content.strip():
        logger.info(f"Skipping empty message from {message.author.display_name}")
        return False
    
    logger.info(f"Processing message from {message.author.display_name} in #{channel_name}: {message.content[:100]}...")
    
    # Add the message to the notes system
    success = execute_command_in_CLI('add', message.content)
    
    if success:
        logger.info(f"Message captured successfully")
        return True
    else:
        logger.error("Failed to add note")
        return False

async def sync_channel_messages(channel):
    """
    Sync all existing messages from the channel to the notes system.
    
    Args:
        channel: Discord channel object
    """
    logger.info(f"Starting message sync for channel #{channel.name}")
    
    try:
        # Fetch all messages from the channel, oldest first
        messages = []
        async for message in channel.history(limit=None, oldest_first=True):
            messages.append(message)
        
        if not messages:
            logger.info("No messages found in channel")
            return
        
        logger.info(f"Found {len(messages)} messages to sync")
        
        processed_count = 0
        deleted_count = 0
        
        for i, message in enumerate(messages, 1):
            logger.info(f"Processing message {i}/{len(messages)}")
            
            # Process the message content
            should_delete = await process_note_message(message, channel.name)
            
            if should_delete:
                try:
                    await message.delete()
                    deleted_count += 1
                    logger.info(f"Message {i} deleted successfully")
                except discord.Forbidden:
                    logger.error(f"No permission to delete message {i}")
                except discord.NotFound:
                    logger.warning(f"Message {i} was already deleted")
                except Exception as e:
                    logger.error(f"Error deleting message {i}: {e}")
                
                processed_count += 1
            
            # Add a small delay to avoid hitting rate limits
            await asyncio.sleep(0.1)
        
        logger.info(f"Sync complete: {processed_count} messages processed, {deleted_count} messages deleted")
        
    except discord.Forbidden:
        logger.error("Bot doesn't have permission to read message history in this channel")
    except Exception as e:
        logger.error(f"Error during message sync: {e}")

async def handle_command(message): 
    # Skip messages from the bot itself
    if message.author == bot.user:
        return False
    
    # Skip empty messages
    if not message.content.strip():
        logger.info(f"Skipping empty message from {message.author.display_name}")
        return False
    
    logger.info(f"Processing command from {message.author.display_name}:  {message.content[:100]}...")

    success = execute_command_in_CLI(message.content, '')

    return success

@bot.event
async def on_ready():
    """Called when the bot has successfully connected to Discord."""
    logger.info(f"Bot logged in as {bot.user.name} (ID: {bot.user.id})")
    
    # Verify the target channel exists and is accessible
    channel = bot.get_channel(NOTES_CHANNEL_ID)
    if channel:
        logger.info(f"Monitoring channel: #{channel.name} (ID: {NOTES_CHANNEL_ID})")
        
        # Sync all existing messages from the channel
        await sync_channel_messages(channel)
        
    else:
        logger.error(f"Could not access channel with ID: {NOTES_CHANNEL_ID}")
        logger.error("Make sure the bot has permission to view the channel")

@bot.event
async def on_message(message):
    """
    Handle incoming messages. Capture and delete messages from the target channel.
    """
    # Only process messages from the target channel
    if message.channel.id == NOTES_CHANNEL_ID:
        
        # Get channel for logging
        channel = bot.get_channel(NOTES_CHANNEL_ID)
        channel_name = channel.name if channel else str(NOTES_CHANNEL_ID)
        
        # Process the message content
        should_delete = await process_note_message(message, channel_name)
    elif message.channel.id == COMMAND_CHANNEL_ID: 
        
        # Get channel for logging
        channel = bot.get_channel(COMMAND_CHANNEL_ID)
        channel_name = channel.name if channel else str(COMMAND_CHANNEL_ID)
        
        # Process the message content
        should_delete = await handle_command(message)
    else: 
        return 

    if should_delete:
        try:
            # Delete the original message
            await message.delete()
            logger.info(f"Real-time message captured and deleted successfully")
        except discord.Forbidden:
            logger.error("Bot doesn't have permission to delete messages in this channel")
        except discord.NotFound:
            logger.warning("Message was already deleted")
        except Exception as e:
            logger.error(f"Error deleting real-time message: {e}")

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
    if ctx.channel.id != COMMAND_CHANNEL_ID:
        await ctx.send("This command only works in the monitored channel.")
        return
    
    embed = discord.Embed(
        title="Discord Notes Bot Status",
        color=0x00ff00,
        timestamp=datetime.now()
    )
    embed.add_field(name="Monitored Channel", value=f"<#{COMMAND_CHANNEL_ID}>", inline=False)
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