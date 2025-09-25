package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var (
	db               *Database
	dbPath           string
	multiFileWatcher *MultiFileWatcher // New multi-file watcher
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	var err error
	basePath := os.Getenv("NOTES_PATH")
	if basePath == "" {
		basePath, err = os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get current directory: %v", err)
		}
	}

	dbPath = filepath.Join(basePath, "notes.db")

	if command != "init" {
		if !fileExists(dbPath) {
			fmt.Printf("Error: No notes repository found in %s. Run 'notes init' first.\n", dbPath)
			os.Exit(1)
		}

		db, err = NewDatabase(dbPath)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()
	}

	switch command {
	case "init":
		handleInit()
	case "add":
		handleAdd()
	case "grep":
		handleGrep()
	case "watch":
		handleWatch()
	case "unwatch":
		handleUnwatch()
	case "watcher":
		handleWatcher()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: notes <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  init                    Initialize new repository")
	fmt.Println("  add \"content\"            Add new note block")
	fmt.Println("  grep \"term1\" \"term2\"      Search across all blocks (union of keywords)")
	fmt.Println("  grep \"term\" \"-excluded\"   Use -prefix to exclude keywords")
	fmt.Println("  watcher                 Start the file watcher daemon")
	fmt.Println("  watch <file>            Add file to watch list")
	fmt.Println("  unwatch <file>          Remove file from watch list")
}

func handleInit() {
	if fileExists(dbPath) {
		fmt.Printf("Repository already exists at %s\n", filepath.Dir(dbPath))
		return
	}

	database, err := NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	fmt.Printf("Initialized empty notes repository at %s\n", filepath.Dir(dbPath))
}

func handleAdd() {
	if len(os.Args) < 3 {
		fmt.Println("Error: add command requires content argument")
		fmt.Println("Usage: notes add \"content\"")
		os.Exit(1)
	}

	content := os.Args[2]
	if content == "" {
		fmt.Println("Error: content cannot be empty")
		os.Exit(1)
	}

	newBlock := NewBlock(content)

	if err := db.CreateBlock(newBlock); err != nil {
		log.Fatalf("Failed to add note: %v", err)
	}

	fmt.Println("Note added successfully")
}

func handleGrep() {
	if len(os.Args) < 3 {
		fmt.Println("Error: grep command requires search term(s)")
		fmt.Println("Usage: notes grep \"term1\" \"term2\" -\"excluded\"")
		os.Exit(1)
	}

	// Parse all arguments after "notes grep"
	args := os.Args[2:]
	var includeKeywords []string
	var excludeKeywords []string

	for _, arg := range args {
		if arg == "" {
			continue
		}
		if arg[0] == '-' {
			// Remove the ! prefix for exclude keywords
			if len(arg) > 1 {
				excludeKeywords = append(excludeKeywords, arg[1:])
			}
		} else {
			includeKeywords = append(includeKeywords, arg)
		}
	}

	if len(includeKeywords) == 0 && len(excludeKeywords) == 0 {
		fmt.Println("Error: at least one search term is required")
		os.Exit(1)
	}

	blocks, err := db.SearchBlocks(includeKeywords, excludeKeywords)
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}

	if len(blocks) == 0 {
		fmt.Println("No blocks found matching the specified criteria")
		return
	}

	for i, block := range blocks {
		fmt.Println(block.Content)
		if i < len(blocks)-1 {
			fmt.Println()
		}
	}
}

func handleWatch() {
	if len(os.Args) < 3 {
		fmt.Println("Error: watch command requires a file path")
		fmt.Println("Usage: notes watch <file>")
		os.Exit(1)
	}

	filePath := os.Args[2]

	// Resolve to absolute path for consistency
	absPath, err := ResolveAbsolutePath(filePath)
	if err != nil {
		log.Fatalf("Failed to resolve file path: %v", err)
	}

	// Check if file exists
	if !fileExists(absPath) {
		log.Fatalf("File does not exist: %s", absPath)
	}

	// Add file to watched files in database
	if err := db.AddWatchedFile(absPath); err != nil {
		log.Fatalf("Failed to add file to watch list: %v", err)
	}

	fmt.Printf("Added %s to watch list\n", absPath)
	fmt.Println("Start the watcher daemon with: notes watcher")
}

func handleUnwatch() {
	if len(os.Args) < 3 {
		fmt.Println("Error: unwatch command requires a file path")
		fmt.Println("Usage: notes unwatch <file>")
		os.Exit(1)
	}

	filePath := os.Args[2]

	// Resolve to absolute path for consistency
	absPath, err := ResolveAbsolutePath(filePath)
	if err != nil {
		log.Fatalf("Failed to resolve file path: %v", err)
	}

	// Check if file is in watch list
	isWatched, err := db.IsFileWatched(absPath)
	if err != nil {
		log.Fatalf("Failed to check if file is watched: %v", err)
	}

	if !isWatched {
		fmt.Printf("File %s is not in the watch list\n", absPath)
		return
	}

	// Remove file from database
	if err := db.RemoveWatchedFile(absPath); err != nil {
		log.Fatalf("Failed to remove file from watch list: %v", err)
	}

	fmt.Printf("Removed %s from watch list\n", absPath)
	fmt.Println("The watcher daemon will pick up these changes automatically")
}

func handleWatcher() {
	// Initialize multi-file watcher
	var err error
	multiFileWatcher, err = NewMultiFileWatcher(db)
	if err != nil {
		log.Fatalf("Failed to create multi-file watcher: %v", err)
	}

	fmt.Println("Starting file watcher daemon...")

	// Start the watcher
	if err := multiFileWatcher.Start(); err != nil {
		log.Fatalf("Failed to start multi-file watcher: %v", err)
	}

	fmt.Println("File watcher daemon started. Monitoring for database changes...")
	fmt.Printf("Press Ctrl+C to stop the daemon.\n\n")

	// Set up periodic database sync
	syncTicker := time.NewTicker(5 * time.Second)
	defer syncTicker.Stop()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Main daemon loop
	for {
		select {
		case <-syncTicker.C:
			// Periodically sync with database
			if err := multiFileWatcher.SyncWithDatabase(); err != nil {
				log.Printf("Error syncing with database: %v", err)
			}

		case sig := <-sigCh:
			fmt.Printf("\nReceived %s signal. Shutting down gracefully...\n", sig)

			// Stop the multi-file watcher
			if err := multiFileWatcher.Stop(); err != nil {
				log.Printf("Error stopping watcher: %v", err)
			}

			fmt.Println("File watcher daemon stopped.")
			return
		}
	}
}
