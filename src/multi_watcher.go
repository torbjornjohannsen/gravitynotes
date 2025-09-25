package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type MultiFileWatcher struct {
	watcher             *fsnotify.Watcher
	db                  *Database
	respondToFileChange map[string]bool
	stopCh              chan bool
	mu                  sync.RWMutex
	IsRunning           bool // Made public
	debounceTimers      map[string]*time.Timer
	reconcilers         map[string]*Reconciler
}

func NewMultiFileWatcher(db *Database) (*MultiFileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &MultiFileWatcher{
		watcher:             watcher,
		db:                  db,
		respondToFileChange: make(map[string]bool),
		stopCh:              make(chan bool),
		debounceTimers:      make(map[string]*time.Timer),
		reconcilers:         make(map[string]*Reconciler),
	}, nil
}

func (mfw *MultiFileWatcher) AddFile(filePath string) error {

	// Resolve to absolute path
	absPath, err := ResolveAbsolutePath(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Check if file exists
	if !fileExists(absPath) {
		return fmt.Errorf("file does not exist: %s", absPath)
	}

	// Add to database as watched file
	if err := mfw.db.AddWatchedFile(absPath); err != nil {
		return fmt.Errorf("failed to add watched file to database: %w", err)
	}

	// Add to fsnotify watcher
	if err := mfw.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to add file to watcher: %w", err)
	}

	newFileManager := NewFileManager(absPath)
	newReconciler := NewReconciler(mfw.db, newFileManager)

	mfw.reconcilers[absPath] = newReconciler
	mfw.respondToFileChange[absPath] = true

	log.Printf("Started watching file: %s", absPath)

	// Perform initial reconciliation
	if err := mfw.reconcilers[absPath].ReconcileFromSpecificFile(); err != nil {
		log.Printf("Failed initial reconciliation for %s: %v", absPath, err)
	}

	return nil
}

func (mfw *MultiFileWatcher) RemoveFile(filePath string) error {
	mfw.mu.Lock()
	defer mfw.mu.Unlock()

	// Resolve to absolute path
	absPath, err := ResolveAbsolutePath(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Remove from fsnotify watcher
	if err := mfw.watcher.Remove(absPath); err != nil {
		log.Printf("Warning: failed to remove file from watcher: %v", err)
	}

	// Remove from database (this will cascade delete file_blocks)
	if err := mfw.db.RemoveWatchedFile(absPath); err != nil {
		return fmt.Errorf("failed to remove watched file from database: %w", err)
	}

	delete(mfw.respondToFileChange, absPath)
	delete(mfw.reconcilers, absPath)

	// Clean up debounce timer if exists
	if timer, exists := mfw.debounceTimers[absPath]; exists {
		timer.Stop()
		delete(mfw.debounceTimers, absPath)
	}

	log.Printf("Stopped watching file: %s", absPath)
	return nil
}

func (mfw *MultiFileWatcher) Start() error {

	if mfw.IsRunning {
		return fmt.Errorf("multi-file watcher is already running")
	}

	mfw.mu.Lock()
	defer mfw.mu.Unlock()

	// Load existing watched files from database
	watchedFiles, err := mfw.db.GetWatchedFiles()
	if err != nil {
		return fmt.Errorf("failed to get watched files: %w", err)
	}

	// Add each watched file
	for _, filePath := range watchedFiles {
		mfw.AddFile(filePath)
	}

	mfw.IsRunning = true
	go mfw.watchLoop()
	return nil
}

func (mfw *MultiFileWatcher) Stop() error {
	mfw.mu.Lock()
	defer mfw.mu.Unlock()

	if !mfw.IsRunning {
		return nil
	}

	mfw.stopCh <- true
	mfw.IsRunning = false

	// Stop all debounce timers
	for _, timer := range mfw.debounceTimers {
		timer.Stop()
	}
	mfw.debounceTimers = make(map[string]*time.Timer)

	if err := mfw.watcher.Close(); err != nil {
		return fmt.Errorf("failed to close file watcher: %w", err)
	}

	log.Println("Multi-file watcher stopped")
	return nil
}

func (mfw *MultiFileWatcher) watchLoop() {
	for {
		select {
		case event, ok := <-mfw.watcher.Events:
			if !ok {
				log.Println("File watcher events channel closed")
				return
			}

			if mfw.shouldProcessEvent(event) {
				mfw.debounceEvent(event.Name)
			}

		case err, ok := <-mfw.watcher.Errors:
			if !ok {
				log.Println("File watcher errors channel closed")
				return
			}
			log.Printf("File watcher error: %v", err)

		case <-mfw.stopCh:
			log.Println("Multi-file watcher stop signal received")
			return
		}
	}
}

func (mfw *MultiFileWatcher) shouldProcessEvent(event fsnotify.Event) bool {
	mfw.mu.Lock()
	defer mfw.mu.Unlock()

	absPath, err := ResolveAbsolutePath(event.Name)

	if err != nil {
		log.Println("Error resolving absolute path of event: %v ", err)
		return false
	}

	if !mfw.respondToFileChange[absPath] {
		// don't ignore the next
		mfw.respondToFileChange[absPath] = true
		return false
	}

	// Handle file deletion
	if event.Op&fsnotify.Remove == fsnotify.Remove {
		log.Printf("Watched file deleted: %s", absPath)
		go func() {
			if err := mfw.RemoveFile(absPath); err != nil {
				log.Printf("Error removing deleted file: %v", err)
			}
		}()
		return false
	}

	// Process write and create events
	if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
		log.Printf("File change detected: %s", absPath)
		return true
	}

	return false
}

func (mfw *MultiFileWatcher) debounceEvent(filePath string) {
	mfw.mu.Lock()
	defer mfw.mu.Unlock()

	// Stop existing timer for this file
	if timer, exists := mfw.debounceTimers[filePath]; exists {
		timer.Stop()
	}

	// Create new timer
	mfw.debounceTimers[filePath] = time.AfterFunc(200*time.Millisecond, func() {
		if err := mfw.reconcilers[filePath].ReconcileFromSpecificFile(); err != nil {
			log.Printf("Reconciliation failed for %s: %v", filePath, err)
		} else {
			log.Printf("Reconciliation completed for %s", filePath)
		}

		if err := mfw.reconcilers[filePath].RegenerateSpecificFile(); err != nil {
			log.Printf("Regeneration failed for %s: %v", filePath, err)
		} else {
			log.Printf("Regenerated %s successfully", filePath)
		}

		mfw.mu.Lock()
		// make sure we don't run an infinite loop
		// - by ignoring the write event we have caused by regenerating
		mfw.respondToFileChange[filePath] = false
		delete(mfw.debounceTimers, filePath)
		mfw.mu.Unlock()
	})
}

func (mfw *MultiFileWatcher) SyncWithDatabase() error {
	mfw.mu.Lock()
	defer mfw.mu.Unlock()

	watchedFiles, err := mfw.db.GetWatchedFiles()
	if err != nil {
		return fmt.Errorf("failed to get watched files from database: %w", err)
	}

	// Convert to set for easy lookup
	dbFileSet := make(map[string]bool)
	for _, file := range watchedFiles {
		dbFileSet[file] = true
	}

	// Add files from database that we're not currently watching
	for _, file := range watchedFiles {
		_, ok := mfw.reconcilers[file]
		if !ok {
			mfw.AddFile(file)
		}
	}

	// Remove files that are no longer in the database
	for file := range mfw.respondToFileChange {
		if !dbFileSet[file] {
			// Remove from fsnotify watcher
			if err := mfw.watcher.Remove(file); err != nil {
				log.Printf("Warning: failed to remove file from watcher: %s: %v", file, err)
			}

			delete(mfw.respondToFileChange, file)
			delete(mfw.reconcilers, file)

			// Clean up debounce timer if exists
			if timer, exists := mfw.debounceTimers[file]; exists {
				timer.Stop()
				delete(mfw.debounceTimers, file)
			}

			log.Printf("Stopped watching file: %s", file)
		}
	}

	return nil
}
