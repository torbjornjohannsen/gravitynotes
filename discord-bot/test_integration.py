#!/usr/bin/env python3
"""
Simple test script to verify Discord bot integration with notes CLI.
"""

import os
import sys
import subprocess
from pathlib import Path

def test_cli_integration():
    """Test that we can call the notes CLI from Python."""
    print("Testing CLI integration...")
    
    # Get path to notes CLI
    cli_path = Path(__file__).parent / "../notes"
    cli_path = cli_path.resolve()
    
    print(f"Notes CLI path: {cli_path}")
    
    if not cli_path.exists():
        print("❌ Notes CLI not found!")
        return False
    
    try:
        # Test with a simple add command
        test_content = "[TEST] Discord bot integration test"
        result = subprocess.run(
            [str(cli_path), 'add', test_content],
            capture_output=True,
            text=True,
            timeout=30,
            cwd=cli_path.parent
        )
        
        if result.returncode == 0:
            print("✅ CLI integration test successful!")
            print(f"Output: {result.stdout.strip()}")
            return True
        else:
            print(f"❌ CLI test failed with exit code: {result.returncode}")
            print(f"Error: {result.stderr}")
            return False
            
    except Exception as e:
        print(f"❌ CLI test failed with exception: {e}")
        return False

def test_bot_imports():
    """Test that the bot script can import required modules."""
    print("Testing bot imports...")
    
    try:
        import discord
        from discord.ext import commands
        from dotenv import load_dotenv
        print("✅ All imports successful!")
        return True
    except ImportError as e:
        print(f"❌ Import failed: {e}")
        return False

def main():
    """Run integration tests."""
    print("Discord Bot Integration Tests")
    print("=" * 40)
    
    success = True
    
    # Test imports
    if not test_bot_imports():
        success = False
    
    print()
    
    # Test CLI integration
    if not test_cli_integration():
        success = False
    
    print()
    
    if success:
        print("🎉 All tests passed! Discord bot is ready for configuration.")
        print("\nNext steps:")
        print("1. Copy .env.example to .env")
        print("2. Fill in your Discord bot token and channel ID")
        print("3. Run: python bot.py")
    else:
        print("❌ Some tests failed. Please fix the issues above.")
        sys.exit(1)

if __name__ == "__main__":
    main()