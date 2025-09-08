package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type MultiFileWatcher struct {
	reconciler     *Reconciler
	fileManager    *FileManager
	watcher        *fsnotify.Watcher
	watchedFiles   map[string]bool
	stopCh         chan bool
	mu             sync.RWMutex
	IsRunning      bool // Made public
	debounceTimers map[string]*time.Timer
}

func NewMultiFileWatcher(reconciler *Reconciler, fileManager *FileManager) (*MultiFileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &MultiFileWatcher{
		reconciler:     reconciler,
		fileManager:    fileManager,
		watcher:        watcher,
		watchedFiles:   make(map[string]bool),
		stopCh:         make(chan bool),
		debounceTimers: make(map[string]*time.Timer),
	}, nil
}

func (mfw *MultiFileWatcher) AddFile(filePath string) error {
	mfw.mu.Lock()
	defer mfw.mu.Unlock()

	// Resolve to absolute path
	absPath, err := mfw.fileManager.ResolveAbsolutePath(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Check if file exists
	if !mfw.fileManager.fileExists(absPath) {
		return fmt.Errorf("file does not exist: %s", absPath)
	}

	// Add to database as watched file
	if err := mfw.reconciler.db.AddWatchedFile(absPath); err != nil {
		return fmt.Errorf("failed to add watched file to database: %w", err)
	}

	// Add to fsnotify watcher
	if err := mfw.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to add file to watcher: %w", err)
	}

	mfw.watchedFiles[absPath] = true
	log.Printf("Started watching file: %s", absPath)

	// Perform initial reconciliation
	if err := mfw.reconciler.ReconcileFromSpecificFile(absPath); err != nil {
		log.Printf("Failed initial reconciliation for %s: %v", absPath, err)
	}

	return nil
}

func (mfw *MultiFileWatcher) RemoveFile(filePath string) error {
	mfw.mu.Lock()
	defer mfw.mu.Unlock()

	// Resolve to absolute path
	absPath, err := mfw.fileManager.ResolveAbsolutePath(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Remove from fsnotify watcher
	if err := mfw.watcher.Remove(absPath); err != nil {
		log.Printf("Warning: failed to remove file from watcher: %v", err)
	}

	// Remove from database (this will cascade delete file_blocks)
	if err := mfw.reconciler.db.RemoveWatchedFile(absPath); err != nil {
		return fmt.Errorf("failed to remove watched file from database: %w", err)
	}

	delete(mfw.watchedFiles, absPath)

	// Clean up debounce timer if exists
	if timer, exists := mfw.debounceTimers[absPath]; exists {
		timer.Stop()
		delete(mfw.debounceTimers, absPath)
	}

	log.Printf("Stopped watching file: %s", absPath)
	return nil
}

func (mfw *MultiFileWatcher) Start() error {
	mfw.mu.Lock()
	defer mfw.mu.Unlock()

	if mfw.IsRunning {
		return fmt.Errorf("multi-file watcher is already running")
	}

	// Load existing watched files from database
	watchedFiles, err := mfw.reconciler.db.GetWatchedFiles()
	if err != nil {
		return fmt.Errorf("failed to get watched files: %w", err)
	}

	// Add each watched file to fsnotify
	for _, filePath := range watchedFiles {
		if mfw.fileManager.fileExists(filePath) {
			if err := mfw.watcher.Add(filePath); err != nil {
				log.Printf("Warning: failed to watch existing file %s: %v", filePath, err)
				continue
			}
			mfw.watchedFiles[filePath] = true
			log.Printf("Resumed watching file: %s", filePath)
		} else {
			// File was deleted, remove from database
			log.Printf("Watched file %s no longer exists, removing from database", filePath)
			if err := mfw.reconciler.db.RemoveWatchedFile(filePath); err != nil {
				log.Printf("Warning: failed to remove deleted file from database: %v", err)
			}
		}
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
	mfw.mu.RLock()
	defer mfw.mu.RUnlock()

	// Check if we're watching this file
	if !mfw.watchedFiles[event.Name] {
		return false
	}

	// Handle file deletion
	if event.Op&fsnotify.Remove == fsnotify.Remove {
		log.Printf("Watched file deleted: %s", event.Name)
		go func() {
			if err := mfw.RemoveFile(event.Name); err != nil {
				log.Printf("Error removing deleted file: %v", err)
			}
		}()
		return false
	}

	// Process write and create events
	if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
		log.Printf("File change detected: %s", event.Name)
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
		if err := mfw.reconciler.ReconcileFromSpecificFile(filePath); err != nil {
			log.Printf("Reconciliation failed for %s: %v", filePath, err)
		} else {
			log.Printf("Reconciliation completed for %s", filePath)
		}

		mfw.mu.Lock()
		delete(mfw.debounceTimers, filePath)
		mfw.mu.Unlock()
	})
}

func (mfw *MultiFileWatcher) SyncWithDatabase() error {
	mfw.mu.Lock()
	defer mfw.mu.Unlock()

	// Get files that should be watched according to database
	dbFiles, err := mfw.reconciler.db.GetWatchedFiles()
	if err != nil {
		return fmt.Errorf("failed to get watched files from database: %w", err)
	}

	// Convert to set for easy lookup
	dbFileSet := make(map[string]bool)
	for _, file := range dbFiles {
		dbFileSet[file] = true
	}

	// Add files from database that we're not currently watching
	for _, file := range dbFiles {
		if !mfw.watchedFiles[file] && mfw.fileManager.fileExists(file) {
			// Add to fsnotify watcher
			if err := mfw.watcher.Add(file); err != nil {
				log.Printf("Warning: failed to add file to watcher: %s: %v", file, err)
				continue
			}

			mfw.watchedFiles[file] = true
			log.Printf("Started watching file: %s", file)

			// Perform initial reconciliation
			if err := mfw.reconciler.ReconcileFromSpecificFile(file); err != nil {
				log.Printf("Warning: initial reconciliation failed for %s: %v", file, err)
			}
		} else if !mfw.watchedFiles[file] && !mfw.fileManager.fileExists(file) {
			// File in database but doesn't exist - remove from database
			log.Printf("File %s no longer exists, removing from database", file)
			if err := mfw.reconciler.db.RemoveWatchedFile(file); err != nil {
				log.Printf("Warning: failed to remove non-existent file from database: %v", err)
			}
		}
	}

	// Remove files that are no longer in the database
	for file := range mfw.watchedFiles {
		if !dbFileSet[file] {
			// Remove from fsnotify watcher
			if err := mfw.watcher.Remove(file); err != nil {
				log.Printf("Warning: failed to remove file from watcher: %s: %v", file, err)
			}

			delete(mfw.watchedFiles, file)

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
