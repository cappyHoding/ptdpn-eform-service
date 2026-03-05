// Package storage manages local file storage for uploaded files.
//
// DIRECTORY STRUCTURE:
//
//	/var/app/storage/
//	├── ktp/{year}/{month}/{application_id}.jpg
//	├── selfie/{year}/{month}/{application_id}.jpg
//	├── collateral/{year}/{month}/{uuid}_{original_filename}
//	└── contracts/{year}/{month}/{application_id}.pdf
//	                          {application_id}_signed.pdf
//
// WHY LOCAL STORAGE?
// BPR Perdana stores files on-premise for security and regulatory compliance.
// All PII (KTP images, selfies) stays within the bank's infrastructure.
// The paths stored in the database are absolute paths on the server.
package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// FileType represents the category of file being stored.
// Each category has its own subdirectory.
type FileType string

const (
	FileTypeKTP        FileType = "ktp"
	FileTypeSelfie     FileType = "selfie"
	FileTypeCollateral FileType = "collateral"
	FileTypeContract   FileType = "contracts"
)

// Manager handles all file I/O operations.
type Manager struct {
	basePath string
}

// New creates a new storage Manager with the given base path.
// Verifies the base path exists and is writable at startup.
func New(basePath string) (*Manager, error) {
	// Ensure all subdirectories exist on startup
	for _, subDir := range []FileType{FileTypeKTP, FileTypeSelfie, FileTypeCollateral, FileTypeContract} {
		dir := filepath.Join(basePath, string(subDir))
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create storage directory %s: %w", dir, err)
		}
	}

	// Test that we can write to the base path
	testFile := filepath.Join(basePath, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return nil, fmt.Errorf("storage path %s is not writable: %w", basePath, err)
	}
	f.Close()
	os.Remove(testFile)

	return &Manager{basePath: basePath}, nil
}

// SaveFile saves a file from a reader to the structured directory.
// Returns the absolute path where the file was saved.
//
// Parameters:
//   - fileType: category (ktp, selfie, collateral, contracts)
//   - filename: the target filename (e.g. "app-uuid.jpg")
//   - src:      the file content reader (from multipart form or generated content)
func (m *Manager) SaveFile(fileType FileType, filename string, src io.Reader) (string, error) {
	// Build the dated directory path: /base/{type}/{year}/{month}/
	now := time.Now()
	dir := filepath.Join(
		m.basePath,
		string(fileType),
		now.Format("2006"), // year: e.g. "2025"
		now.Format("01"),   // month: e.g. "07"
	)

	// Create directory if it doesn't exist (new month, new directory)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Full path for the file
	filePath := filepath.Join(dir, filename)

	// Create the file — O_EXCL ensures we don't accidentally overwrite
	// If file exists (same application_id same month), use O_TRUNC to overwrite
	dst, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer dst.Close()

	// Copy content from source to destination
	if _, err := io.Copy(dst, src); err != nil {
		// Clean up partial file on failure
		os.Remove(filePath)
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return filePath, nil
}

// DeleteFile removes a file by its absolute path.
// Used when an application is fully deleted (edge case — soft deletes don't touch files).
func (m *Manager) DeleteFile(absolutePath string) error {
	// Security check: ensure the path is within our base directory
	// Prevents directory traversal attacks
	rel, err := filepath.Rel(m.basePath, absolutePath)
	if err != nil || len(rel) > 0 && rel[0] == '.' {
		return fmt.Errorf("invalid file path: path must be within storage directory")
	}

	if err := os.Remove(absolutePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file %s: %w", absolutePath, err)
	}
	return nil
}

// BuildKTPPath builds the expected path for a KTP image.
// Use this to construct the path BEFORE saving (for validation, etc.)
func (m *Manager) BuildKTPPath(applicationID string) string {
	now := time.Now()
	return filepath.Join(
		m.basePath, string(FileTypeKTP),
		now.Format("2006"), now.Format("01"),
		applicationID+".jpg",
	)
}

// BuildSelfiePath builds the expected path for a selfie image.
func (m *Manager) BuildSelfiePath(applicationID string) string {
	now := time.Now()
	return filepath.Join(
		m.basePath, string(FileTypeSelfie),
		now.Format("2006"), now.Format("01"),
		applicationID+".jpg",
	)
}

// BuildContractPath builds the expected path for a contract PDF.
func (m *Manager) BuildContractPath(applicationID string) string {
	now := time.Now()
	return filepath.Join(
		m.basePath, string(FileTypeContract),
		now.Format("2006"), now.Format("01"),
		applicationID+".pdf",
	)
}

// BuildSignedContractPath builds the expected path for a signed contract PDF.
func (m *Manager) BuildSignedContractPath(applicationID string) string {
	now := time.Now()
	return filepath.Join(
		m.basePath, string(FileTypeContract),
		now.Format("2006"), now.Format("01"),
		applicationID+"_signed.pdf",
	)
}
