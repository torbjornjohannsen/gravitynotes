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

func (r *Reconciler) SearchBlocks(includeKeywords, excludeKeywords []string) ([]*Block, error) {
	blocks, err := r.db.SearchBlocks(includeKeywords, excludeKeywords)
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

	// Step 3: Add only new blocks to database (append-only behavior)
	addedCount := 0
	skippedCount := 0
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
			// New block - add to top by updating timestamp to now
			block.UpdatedAt = time.Now()
			if err := r.db.CreateBlock(block); err != nil {
				return fmt.Errorf("failed to create block: %w", err)
			}
			addedCount++
			log.Printf("Created new block with hash: %s", block.ContentHash)
		} else {
			// Block already exists - skip it
			skippedCount++
		}
	}
	log.Printf("Added %d new blocks from %s (skipped %d duplicates)", addedCount, sourceFilePath, skippedCount)

	// Step 4: Regenerate the main notes.md file
	if err := r.RegenerateMarkdownFile(); err != nil {
		return fmt.Errorf("failed to regenerate markdown: %w", err)
	}

	return nil
}

func (r *Reconciler) ReconcileFromSpecificFile(filePath string) error {
	// Resolve to absolute path for consistency
	absPath, err := r.fileManager.ResolveAbsolutePath(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Read the file content
	content, err := r.fileManager.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", absPath, err)
	}

	// Parse blocks from the file
	fileBlocks := ParseBlocksFromMarkdown(content)

	// Get current block hashes associated with this file
	currentHashes, err := r.db.GetFileBlockHashes(absPath)
	if err != nil {
		return fmt.Errorf("failed to get current file blocks: %w", err)
	}

	// Create map of current hashes for quick lookup
	currentHashSet := make(map[string]bool)
	for _, hash := range currentHashes {
		currentHashSet[hash] = true
	}

	// Process blocks from file
	newHashSet := make(map[string]bool)
	for _, fileBlock := range fileBlocks {
		if fileBlock.IsEmpty() {
			continue
		}

		newHashSet[fileBlock.ContentHash] = true

		// Check if block exists in database
		existingBlock, err := r.db.GetBlockByHash(fileBlock.ContentHash)
		if err != nil {
			return fmt.Errorf("failed to get block by hash: %w", err)
		}

		if existingBlock == nil {
			// Create new block
			if err := r.db.CreateBlock(fileBlock); err != nil {
				return fmt.Errorf("failed to create new block: %w", err)
			}
			log.Printf("Created new block with hash: %s", fileBlock.ContentHash)
		} else {
			// Update timestamp for existing block
			if err := r.db.UpdateBlockTimestamp(fileBlock.ContentHash, time.Now()); err != nil {
				return fmt.Errorf("failed to update block timestamp: %w", err)
			}
		}

		// Add file-block association
		if err := r.db.AddFileBlockAssociation(absPath, fileBlock.ContentHash); err != nil {
			return fmt.Errorf("failed to add file-block association: %w", err)
		}
	}

	// Remove blocks that are no longer in the file
	// This will delete them entirely from the database (global deletion)
	for _, hash := range currentHashes {
		if !newHashSet[hash] {
			// Block was deleted from this file - delete it entirely from database
			if err := r.db.DeleteBlockByHash(hash); err != nil {
				return fmt.Errorf("failed to delete block: %w", err)
			}
			log.Printf("Deleted block with hash: %s (removed from %s)", hash, absPath)
		}
	}

	return nil
}

func (r *Reconciler) RegenerateSpecificFile(filePath string) error {
	// Resolve to absolute path for consistency
	absPath, err := r.fileManager.ResolveAbsolutePath(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Get block hashes for this file
	hashes, err := r.db.GetFileBlockHashes(absPath)
	if err != nil {
		return fmt.Errorf("failed to get file block hashes: %w", err)
	}

	// Get actual blocks for these hashes
	var blocks []*Block
	for _, hash := range hashes {
		block, err := r.db.GetBlockByHash(hash)
		if err != nil {
			return fmt.Errorf("failed to get block by hash: %w", err)
		}
		if block != nil {
			blocks = append(blocks, block)
		}
	}

	// Convert to markdown
	content := BlocksToMarkdown(blocks)

	// Write to file
	if err := r.fileManager.WriteFile(absPath, content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("Regenerated file %s with %d blocks", absPath, len(blocks))
	return nil
}
