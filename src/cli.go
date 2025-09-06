package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var (
	db          *Database
	fileManager *FileManager
	reconciler  *Reconciler
	watcher     *FileWatcher
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

	fileManager = NewFileManager(basePath)

	if command != "init" {
		if !fileManager.DatabaseExists() {
			fmt.Printf("Error: No notes repository found in %s. Run 'notes init' first.\n", fileManager.dbPath)
			os.Exit(1)
		}

		db, err = NewDatabase(fileManager.GetDBPath())
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		reconciler = NewReconciler(db, fileManager)
	}

	switch command {
	case "init":
		handleInit()
	case "add":
		handleAdd()
	case "grep":
		handleGrep()
	case "export":
		handleExport()
	case "ingest":
		handleIngest()
	case "watch":
		handleWatch()
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
	fmt.Println("  grep \"term\" \"!exclude\"    Use !prefix to exclude keywords")
	fmt.Println("  export                  Force regenerate .md from DB")
	fmt.Println("  ingest \"tag\" filename    Replace all blocks containing tag with file content")
	fmt.Println("  watch                   Start file watcher (development)")
}

func handleInit() {
	basePath := os.Getenv("NOTES_PATH")
	if basePath == "" {
		var err error
		basePath, err = os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get current directory: %v", err)
		}
	}

	fm := NewFileManager(basePath)

	if fm.DatabaseExists() {
		fmt.Printf("Repository already exists at %s\n", filepath.Dir(fm.GetDBPath()))
		return
	}

	database, err := NewDatabase(fm.GetDBPath())
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	reconciler := NewReconciler(database, fm)

	if err := reconciler.Initialize(); err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	fmt.Printf("Initialized empty notes repository at %s\n", filepath.Dir(fm.GetDBPath()))
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

	if err := reconciler.AddBlock(content); err != nil {
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

	blocks, err := reconciler.SearchBlocks(includeKeywords, excludeKeywords)
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}

	if len(blocks) == 0 {
		fmt.Println("No blocks found matching the specified criteria")
		return
	}

	for i := len(blocks) - 1; i >= 0; i-- {
		fmt.Println(blocks[i].Content)
		if i < len(blocks)-1 {
			fmt.Println()
		}
	}
}

func handleExport() {
	if err := reconciler.RegenerateMarkdownFile(); err != nil {
		log.Fatalf("Failed to export: %v", err)
	}

	fmt.Println("Markdown file regenerated from database")
}

func handleIngest() {
	if len(os.Args) < 4 {
		fmt.Println("Error: ingest command requires tag and filename arguments")
		fmt.Println("Usage: notes ingest \"tag\" filename")
		os.Exit(1)
	}

	tag := os.Args[2]
	if tag == "" {
		fmt.Println("Error: tag cannot be empty")
		os.Exit(1)
	}

	filename := os.Args[3]
	if filename == "" {
		fmt.Println("Error: filename cannot be empty")
		os.Exit(1)
	}

	// Call reconciler to perform the ingestion
	if err := reconciler.IngestTaggedBlocks(tag, filename); err != nil {
		log.Fatalf("Failed to ingest blocks: %v", err)
	}

	fmt.Printf("Successfully replaced blocks containing '%s' with content from %s\n", tag, filename)
}

func handleWatch() {
	watcher, err := NewFileWatcher(reconciler, fileManager)
	if err != nil {
		log.Fatalf("Failed to create file watcher: %v", err)
	}

	if err := watcher.Start(); err != nil {
		log.Fatalf("Failed to start file watcher: %v", err)
	}

	fmt.Printf("Watching %s for changes. Press Ctrl+C to stop.\n",
		fileManager.GetNotesPath())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	fmt.Println("\nStopping file watcher...")

	if err := watcher.Stop(); err != nil {
		log.Printf("Error stopping watcher: %v", err)
	}
}
