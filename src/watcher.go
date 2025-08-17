package main

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	reconciler   *Reconciler
	fileManager  *FileManager
	watcher      *fsnotify.Watcher
	stopCh       chan bool
	isWatching   bool
	isWriting    bool
}

func NewFileWatcher(reconciler *Reconciler, fileManager *FileManager) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &FileWatcher{
		reconciler:  reconciler,
		fileManager: fileManager,
		watcher:     watcher,
		stopCh:      make(chan bool),
		isWatching:  false,
		isWriting:   false,
	}, nil
}

func (fw *FileWatcher) Start() error {
	if fw.isWatching {
		return fmt.Errorf("file watcher is already running")
	}

	notesDir := filepath.Dir(fw.fileManager.GetNotesPath())
	if err := fw.watcher.Add(notesDir); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", notesDir, err)
	}

	fw.isWatching = true
	log.Printf("Started watching directory: %s", notesDir)

	go fw.watchLoop()
	return nil
}

func (fw *FileWatcher) Stop() error {
	if !fw.isWatching {
		return nil
	}

	fw.stopCh <- true
	fw.isWatching = false
	
	if err := fw.watcher.Close(); err != nil {
		return fmt.Errorf("failed to close file watcher: %w", err)
	}

	log.Println("File watcher stopped")
	return nil
}

func (fw *FileWatcher) watchLoop() {
	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
	}

	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				log.Println("File watcher events channel closed")
				return
			}

			if fw.shouldProcessEvent(event) {
				debounceTimer.Reset(200 * time.Millisecond)
			}

		case <-debounceTimer.C:
			if !fw.isWriting {
				fw.isWriting = true
				if err := fw.reconciler.ReconcileFromFile(); err != nil {
					log.Printf("Reconciliation failed: %v", err)
				} else {
					log.Println("File change detected - reconciliation completed")
				}
				fw.isWriting = false
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				log.Println("File watcher errors channel closed")
				return
			}
			log.Printf("File watcher error: %v", err)

		case <-fw.stopCh:
			log.Println("File watcher stop signal received")
			return
		}
	}
}

func (fw *FileWatcher) shouldProcessEvent(event fsnotify.Event) bool {
	notesPath := fw.fileManager.GetNotesPath()
	
	if event.Name != notesPath || fw.isWriting {
		return false
	}

	if event.Op&fsnotify.Write == fsnotify.Write {
		log.Printf("File write detected: %s", event.Name)
		return true
	}

	if event.Op&fsnotify.Create == fsnotify.Create {
		log.Printf("File creation detected: %s", event.Name)
		return true
	}

	return false
}

func (fw *FileWatcher) IsWatching() bool {
	return fw.isWatching
}