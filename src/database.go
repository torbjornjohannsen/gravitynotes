package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return database, nil
}

func (d *Database) createTables() error {
	blocksTable := `
	CREATE TABLE IF NOT EXISTS blocks (
		id INTEGER PRIMARY KEY,
		content TEXT NOT NULL,
		content_hash TEXT UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	metadataTable := `
	CREATE TABLE IF NOT EXISTS metadata (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);`

	if _, err := d.db.Exec(blocksTable); err != nil {
		return fmt.Errorf("failed to create blocks table: %w", err)
	}

	if _, err := d.db.Exec(metadataTable); err != nil {
		return fmt.Errorf("failed to create metadata table: %w", err)
	}

	return nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) CreateBlock(block *Block) error {
	query := `INSERT INTO blocks (content, content_hash, created_at, updated_at) 
			  VALUES (?, ?, ?, ?)`
	
	result, err := d.db.Exec(query, block.Content, block.ContentHash, 
		block.CreatedAt, block.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert block: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	block.ID = int(id)
	return nil
}

func (d *Database) GetBlockByHash(hash string) (*Block, error) {
	query := `SELECT id, content, content_hash, created_at, updated_at 
			  FROM blocks WHERE content_hash = ?`
	
	row := d.db.QueryRow(query, hash)
	
	var block Block
	err := row.Scan(&block.ID, &block.Content, &block.ContentHash, 
		&block.CreatedAt, &block.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan block: %w", err)
	}

	return &block, nil
}

func (d *Database) GetAllBlocks() ([]*Block, error) {
	query := `SELECT id, content, content_hash, created_at, updated_at 
			  FROM blocks ORDER BY updated_at DESC`
	
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query blocks: %w", err)
	}
	defer rows.Close()

	var blocks []*Block
	for rows.Next() {
		var block Block
		err := rows.Scan(&block.ID, &block.Content, &block.ContentHash,
			&block.CreatedAt, &block.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan block: %w", err)
		}
		blocks = append(blocks, &block)
	}

	return blocks, nil
}

func (d *Database) DeleteBlock(id int) error {
	query := `DELETE FROM blocks WHERE id = ?`
	_, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete block: %w", err)
	}
	return nil
}

func (d *Database) UpdateBlockTimestamp(hash string, timestamp time.Time) error {
	query := `UPDATE blocks SET updated_at = ? WHERE content_hash = ?`
	_, err := d.db.Exec(query, timestamp, hash)
	if err != nil {
		return fmt.Errorf("failed to update block timestamp: %w", err)
	}
	return nil
}

func (d *Database) GetMetadata(key string) (string, error) {
	query := `SELECT value FROM metadata WHERE key = ?`
	row := d.db.QueryRow(query, key)
	
	var value string
	err := row.Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("failed to get metadata: %w", err)
	}
	
	return value, nil
}

func (d *Database) SetMetadata(key, value string) error {
	query := `INSERT OR REPLACE INTO metadata (key, value) VALUES (?, ?)`
	_, err := d.db.Exec(query, key, value)
	if err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}
	return nil
}

func (d *Database) SearchBlocks(searchTerm string) ([]*Block, error) {
	query := `SELECT id, content, content_hash, created_at, updated_at 
			  FROM blocks WHERE content LIKE ? ORDER BY updated_at DESC`
	
	rows, err := d.db.Query(query, "%"+searchTerm+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to search blocks: %w", err)
	}
	defer rows.Close()

	var blocks []*Block
	for rows.Next() {
		var block Block
		err := rows.Scan(&block.ID, &block.Content, &block.ContentHash,
			&block.CreatedAt, &block.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan block: %w", err)
		}
		blocks = append(blocks, &block)
	}

	return blocks, nil
}

func (d *Database) GetBlocksCreatedAfter(timestamp time.Time) ([]*Block, error) {
	query := `SELECT id, content, content_hash, created_at, updated_at 
			  FROM blocks WHERE created_at > ? ORDER BY updated_at DESC`
	
	rows, err := d.db.Query(query, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to query blocks: %w", err)
	}
	defer rows.Close()

	var blocks []*Block
	for rows.Next() {
		var block Block
		err := rows.Scan(&block.ID, &block.Content, &block.ContentHash,
			&block.CreatedAt, &block.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan block: %w", err)
		}
		blocks = append(blocks, &block)
	}

	return blocks, nil
}

func (d *Database) DeleteBlocksByTag(tag string) (int, error) {
	query := `DELETE FROM blocks WHERE content LIKE ?`
	result, err := d.db.Exec(query, "%"+tag+"%")
	if err != nil {
		return 0, fmt.Errorf("failed to delete blocks with tag '%s': %w", tag, err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows count: %w", err)
	}
	
	return int(rowsAffected), nil
}