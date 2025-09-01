package main

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

const LastReconciliationTimeKey = "last_reconciliation_time"

type Reconciler struct {
	db          *Database
	fileManager *FileManager
}

func NewReconciler(db *Database, fileManager *FileManager) *Reconciler {
	return &Reconciler{
		db:          db,
		fileManager: fileManager,
	}
}

func (r *Reconciler) ReconcileFromFile() error {
	content, err := r.fileManager.ReadMarkdownFile()
	if err != nil {
		return fmt.Errorf("failed to read markdown file: %w", err)
	}

	fileBlocks := ParseBlocksFromMarkdown(content)

	lastReconciliationTime, err := r.getLastReconciliationTime()
	if err != nil {
		return fmt.Errorf("failed to get last reconciliation time: %w", err)
	}

	if err := r.processFileBlocks(fileBlocks); err != nil {
		return fmt.Errorf("failed to process file blocks: %w", err)
	}

	if err := r.removeDeletedBlocks(fileBlocks, lastReconciliationTime); err != nil {
		return fmt.Errorf("failed to remove deleted blocks: %w", err)
	}

	if err := r.RegenerateMarkdownFile(); err != nil {
		return fmt.Errorf("failed to regnerate markdown file: %w", err)
	}

	return r.updateLastReconciliationTime()
}

func (r *Reconciler) processFileBlocks(fileBlocks []*Block) error {
	for _, fileBlock := range fileBlocks {
		if fileBlock.IsEmpty() {
			continue
		}

		existingBlock, err := r.db.GetBlockByHash(fileBlock.ContentHash)
		if err != nil {
			return fmt.Errorf("failed to get block by hash: %w", err)
		}

		if existingBlock == nil {
			if err := r.db.CreateBlock(fileBlock); err != nil {
				return fmt.Errorf("failed to create new block: %w", err)
			}
			log.Printf("Created new block with hash: %s", fileBlock.ContentHash)
		}
	}

	return nil
}

func (r *Reconciler) removeDeletedBlocks(fileBlocks []*Block, lastReconciliationTime time.Time) error {
	dbBlocks, err := r.db.GetAllBlocks()
	if err != nil {
		return fmt.Errorf("failed to get all blocks from database: %w", err)
	}

	fileHashSet := make(map[string]bool)
	for _, fileBlock := range fileBlocks {
		fileHashSet[fileBlock.ContentHash] = true
	}

	for _, dbBlock := range dbBlocks {
		if !fileHashSet[dbBlock.ContentHash] {
			if dbBlock.CreatedAt.After(lastReconciliationTime) {
				log.Printf("Preserving CLI-added block (ID: %d) created after last reconciliation", dbBlock.ID)
				continue
			}

			if err := r.db.DeleteBlock(dbBlock.ID); err != nil {
				return fmt.Errorf("failed to delete block %d: %w", dbBlock.ID, err)
			}
			log.Printf("Deleted block with hash: %s", dbBlock.ContentHash)
		}
	}

	return nil
}

func (r *Reconciler) RegenerateMarkdownFile() error {
	blocks, err := r.db.GetAllBlocks()
	if err != nil {
		return fmt.Errorf("failed to get blocks from database: %w", err)
	}

	content := BlocksToMarkdown(blocks)

	if err := r.fileManager.WriteMarkdownFile(content); err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}

	log.Printf("Regenerated markdown file with %d blocks", len(blocks))
	return nil
}

func (r *Reconciler) getLastReconciliationTime() (time.Time, error) {
	timeStr, err := r.db.GetMetadata(LastReconciliationTimeKey)
	if err != nil {
		return time.Time{}, err
	}

	if timeStr == "" {
		return time.Unix(0, 0), nil
	}

	timestamp, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse reconciliation time: %w", err)
	}

	return time.Unix(timestamp, 0), nil
}

func (r *Reconciler) updateLastReconciliationTime() error {
	now := time.Now()
	// we add 1 since unixtime is in seconds and the whole process usually takes less than 1s
	// thus we'll get a bunch of false positives, since generation of new blocks and reconciliation
	// happen on the same second
	timeStr := strconv.FormatInt(now.Unix()+1, 10)

	if err := r.db.SetMetadata(LastReconciliationTimeKey, timeStr); err != nil {
		return fmt.Errorf("failed to update reconciliation time: %w", err)
	}

	return nil
}

func (r *Reconciler) AddBlock(content string) error {
	block := NewBlock(content)
	if block.IsEmpty() {
		return fmt.Errorf("cannot add empty block")
	}

	existingBlock, err := r.db.GetBlockByHash(block.ContentHash)
	if err != nil {
		return fmt.Errorf("failed to check for existing block: %w", err)
	}

	if existingBlock != nil {
		if err := r.db.UpdateBlockTimestamp(block.ContentHash, time.Now()); err != nil {
			return fmt.Errorf("failed to update existing block timestamp: %w", err)
		}
		log.Printf("Updated timestamp for existing block with hash: %s", block.ContentHash)
	} else {
		if err := r.db.CreateBlock(block); err != nil {
			return fmt.Errorf("failed to create block: %w", err)
		}
		log.Printf("Created new block with hash: %s", block.ContentHash)
	}

	return r.RegenerateMarkdownFile()
}

func (r *Reconciler) SearchBlocks(searchTerm string) ([]*Block, error) {
	blocks, err := r.db.SearchBlocks(searchTerm)
	if err != nil {
		return nil, fmt.Errorf("failed to search blocks: %w", err)
	}

	return blocks, nil
}

func (r *Reconciler) Initialize() error {
	if err := r.fileManager.EnsureDirectoryExists(); err != nil {
		return fmt.Errorf("failed to ensure directory exists: %w", err)
	}

	if err := r.fileManager.CreateEmptyMarkdownFile(); err != nil {
		return fmt.Errorf("failed to create empty markdown file: %w", err)
	}

	if err := r.updateLastReconciliationTime(); err != nil {
		return fmt.Errorf("failed to set initial reconciliation time: %w", err)
	}

	log.Println("Repository initialized successfully")
	return nil
}

func (r *Reconciler) IngestTaggedBlocks(tag string, sourceFilePath string) error {
	// Step 1: Read the external markdown file
	content, err := r.fileManager.ReadExternalMarkdownFile(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Step 2: Parse blocks from the file using existing parser
	newBlocks := ParseBlocksFromMarkdown(content)

	// Step 3: Delete existing blocks with the tag
	deletedCount, err := r.db.DeleteBlocksByTag(tag)
	if err != nil {
		return fmt.Errorf("failed to delete blocks with tag '%s': %w", tag, err)
	}
	log.Printf("Deleted %d blocks containing '%s'", deletedCount, tag)

	// Step 4: Add each block to database
	addedCount := 0
	for _, block := range newBlocks {
		if block.IsEmpty() {
			continue
		}

		// Check for existing block with same content hash
		existingBlock, err := r.db.GetBlockByHash(block.ContentHash)
		if err != nil {
			return fmt.Errorf("failed to check existing block: %w", err)
		}

		if existingBlock == nil {
			if err := r.db.CreateBlock(block); err != nil {
				return fmt.Errorf("failed to create block: %w", err)
			}
			addedCount++
			log.Printf("Created new block with hash: %s", block.ContentHash)
		} else {
			// Update timestamp if block already exists
			if err := r.db.UpdateBlockTimestamp(block.ContentHash, time.Now()); err != nil {
				return fmt.Errorf("failed to update existing block timestamp: %w", err)
			}
			log.Printf("Updated timestamp for existing block with hash: %s", block.ContentHash)
		}
	}
	log.Printf("Added %d new blocks from %s", addedCount, sourceFilePath)

	// Step 5: Regenerate the main notes.md file
	if err := r.RegenerateMarkdownFile(); err != nil {
		return fmt.Errorf("failed to regenerate markdown: %w", err)
	}

	return nil
}
