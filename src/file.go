package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type FileManager struct {
	notesPath string
}

func NewFileManager(filename string) *FileManager {
	return &FileManager{
		notesPath: filename,
	}
}

func (fm *FileManager) GetNotesPath() string {
	return fm.notesPath
}

func (fm *FileManager) ReadMarkdownFile() (string, error) {
	return fm.ReadFile(fm.notesPath)
}

func (fm *FileManager) ReadFile(filePath string) (string, error) {
	if !fileExists(filePath) {
		return "", nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return string(content), nil
}

func (fm *FileManager) WriteMarkdownFile(content string) error {
	return fm.WriteFile(fm.notesPath, content)
}

func (fm *FileManager) WriteFile(filePath, content string) error {
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

func (fm *FileManager) markdownFileExists() bool {
	return fileExists(fm.notesPath)
}

func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

func ResolveAbsolutePath(filePath string) (string, error) {
	if filepath.IsAbs(filePath) {
		return filePath, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	return filepath.Join(cwd, filePath), nil
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
