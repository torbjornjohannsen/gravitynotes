package main

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
)

type Block struct {
	ID          int       `json:"id"`
	Content     string    `json:"content"`
	ContentHash string    `json:"content_hash"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewBlock(content string) *Block {
	now := time.Now()
	trimmedContent := strings.TrimSpace(content)
	
	return &Block{
		Content:     trimmedContent,
		ContentHash: generateContentHash(trimmedContent),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func generateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

func (b *Block) UpdateContent(content string) {
	b.Content = strings.TrimSpace(content)
	b.ContentHash = generateContentHash(b.Content)
	b.UpdatedAt = time.Now()
}

func (b *Block) IsEmpty() bool {
	return strings.TrimSpace(b.Content) == ""
}

func ParseBlocksFromMarkdown(content string) []*Block {
	if strings.TrimSpace(content) == "" {
		return []*Block{}
	}

	sections := strings.Split(content, "\n\n")
	var blocks []*Block

	for _, section := range sections {
		trimmed := strings.TrimSpace(section)
		if trimmed == "" {
			continue
		}

		normalizedSection := normalizeWhitespace(trimmed)
		if normalizedSection != "" {
			block := NewBlock(normalizedSection)
			blocks = append(blocks, block)
		}
	}

	return blocks
}

func normalizeWhitespace(content string) string {
	lines := strings.Split(content, "\n")
	var normalizedLines []string

	for _, line := range lines {
		normalizedLines = append(normalizedLines, strings.TrimRight(line, " \t"))
	}

	result := strings.Join(normalizedLines, "\n")
	return strings.TrimSpace(result)
}

func BlocksToMarkdown(blocks []*Block) string {
	if len(blocks) == 0 {
		return ""
	}

	var sections []string
	for _, block := range blocks {
		if !block.IsEmpty() {
			sections = append(sections, block.Content)
		}
	}

	return strings.Join(sections, "\n\n")
}

func FindBlocksByContentHash(blocks []*Block, targetHash string) *Block {
	for _, block := range blocks {
		if block.ContentHash == targetHash {
			return block
		}
	}
	return nil
}

func FilterBlocksByContent(blocks []*Block, searchTerm string) []*Block {
	var matches []*Block
	searchLower := strings.ToLower(searchTerm)
	
	for _, block := range blocks {
		contentLower := strings.ToLower(block.Content)
		if strings.Contains(contentLower, searchLower) {
			matches = append(matches, block)
		}
	}
	
	return matches
}