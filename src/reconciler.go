package main

import (
	"fmt"
	"log"
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

func (r *Reconciler) ReconcileFromSpecificFile() error {
	// Read the file content
	content, err := r.fileManager.ReadMarkdownFile()
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", r.fileManager.notesPath, err)
	}

	// Parse blocks from the file
	parsedFileBlocks := ParseBlocksFromMarkdown(content)

	// Get current block hashes associated with this file
	currentlyAssociatedHashes, err := r.db.GetFileBlockHashes(r.fileManager.notesPath)
	if err != nil {
		return fmt.Errorf("failed to get current file blocks: %w", err)
	}

	// Process blocks from file
	newAssociatedHashes := make(map[string]bool)
	for _, parsedBlock := range parsedFileBlocks {
		if parsedBlock.IsEmpty() {
			continue
		}

		newAssociatedHashes[parsedBlock.ContentHash] = true

		// Check if identical block already exists in database
		preexistingBlock, err := r.db.GetBlockByHash(parsedBlock.ContentHash)
		if err != nil {
			return fmt.Errorf("failed to get block by hash: %w", err)
		}

		if preexistingBlock == nil {
			// if not, we add it
			if err := r.db.CreateBlock(parsedBlock); err != nil {
				return fmt.Errorf("failed to create new block: %w", err)
			}
			log.Printf("Created new block with hash: %s", parsedBlock.ContentHash)
		}

		// Add file-block association - ignores duplicates automatically
		if err := r.db.AddFileBlockAssociation(r.fileManager.notesPath, parsedBlock.ContentHash); err != nil {
			return fmt.Errorf("failed to add file-block association: %w", err)
		}
	}

	// Remove blocks that are no longer in the file
	// This will delete them entirely from the database (global deletion)
	for _, hash := range currentlyAssociatedHashes {
		if !newAssociatedHashes[hash] {
			// Block was deleted from this file - delete it entirely from database
			if err := r.db.DeleteBlockByHash(hash); err != nil {
				return fmt.Errorf("failed to delete block: %w", err)
			}
			log.Printf("Deleted block with hash: %s (removed from %s)", hash, r.fileManager.notesPath)
		}
	}

	return nil
}

func (r *Reconciler) RegenerateSpecificFile() error {
	// Get block hashes for this file
	hashes, err := r.db.GetFileBlockHashes(r.fileManager.notesPath)
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
	if err := r.fileManager.WriteMarkdownFile(content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("Regenerated file %s with %d blocks", r.fileManager.notesPath, len(blocks))
	return nil
}
