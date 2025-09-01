package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type FileManager struct {
	notesPath string
	dbPath    string
}

func NewFileManager(basePath string) *FileManager {
	return &FileManager{
		notesPath: filepath.Join(basePath, "notes.md"),
		dbPath:    filepath.Join(basePath, "notes.db"),
	}
}

func (fm *FileManager) GetNotesPath() string {
	return fm.notesPath
}

func (fm *FileManager) GetDBPath() string {
	return fm.dbPath
}

func (fm *FileManager) ReadMarkdownFile() (string, error) {
	if !fm.markdownFileExists() {
		return "", nil
	}

	content, err := os.ReadFile(fm.notesPath)
	if err != nil {
		return "", fmt.Errorf("failed to read markdown file: %w", err)
	}

	return string(content), nil
}

func (fm *FileManager) WriteMarkdownFile(content string) error {
	err := os.WriteFile(fm.notesPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}

	return nil
}

func (fm *FileManager) markdownFileExists() bool {
	_, err := os.Stat(fm.notesPath)
	return !os.IsNotExist(err)
}

func (fm *FileManager) DatabaseExists() bool {
	_, err := os.Stat(fm.dbPath)
	return !os.IsNotExist(err)
}

func (fm *FileManager) EnsureDirectoryExists() error {
	dir := filepath.Dir(fm.notesPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

func (fm *FileManager) CreateEmptyMarkdownFile() error {
	if fm.markdownFileExists() {
		return nil
	}

	if err := fm.EnsureDirectoryExists(); err != nil {
		return err
	}

	return fm.WriteMarkdownFile("")
}

func (fm *FileManager) GetFileModTime() (int64, error) {
	if !fm.markdownFileExists() {
		return 0, nil
	}

	info, err := os.Stat(fm.notesPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file mod time: %w", err)
	}

	return info.ModTime().Unix(), nil
}

func (fm *FileManager) BackupMarkdownFile() error {
	if !fm.markdownFileExists() {
		return nil
	}

	backupPath := fm.notesPath + ".backup"
	content, err := fm.ReadMarkdownFile()
	if err != nil {
		return fmt.Errorf("failed to read file for backup: %w", err)
	}

	err = os.WriteFile(backupPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

func (fm *FileManager) RestoreFromBackup() error {
	backupPath := fm.notesPath + ".backup"
	
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist")
	}

	content, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	return fm.WriteMarkdownFile(string(content))
}

func (fm *FileManager) DeleteBackup() error {
	backupPath := fm.notesPath + ".backup"
	
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(backupPath)
}

func (fm *FileManager) ReadExternalMarkdownFile(filePath string) (string, error) {
	// If not absolute, make it absolute from current working directory
	if !filepath.IsAbs(filePath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		filePath = filepath.Join(cwd, filePath)
	}
	
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", filePath)
	}
	
	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read external markdown file: %w", err)
	}
	
	return string(content), nil
}