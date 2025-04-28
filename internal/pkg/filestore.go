package pkg

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

const UploadDir = "./uploads"

func init() {
	// Ensure upload directory exists
	if err := os.MkdirAll(UploadDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create upload directory: %v", err))
	}
}

// SaveFile saves an uploaded file to disk and returns the path
func SaveFile(file *multipart.FileHeader, taskID uuid.UUID) (string, error) {
	// Create task directory if it doesn't exist
	taskDir := filepath.Join(UploadDir, taskID.String())
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create task directory: %w", err)
	}

	// Get the filename
	filename := filepath.Base(file.Filename)

	// Create the file path
	filepath := filepath.Join(taskDir, filename)

	// Open the file
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("error opening uploaded file: %w", err)
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("error creating destination file: %w", err)
	}
	defer dst.Close()

	// Copy contents
	if _, err = io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("error copying file contents: %w", err)
	}

	return filepath, nil
}

// GetFilePath returns the full path to a task file
func GetFilePath(taskID uuid.UUID, filename string) string {
	return filepath.Join(UploadDir, taskID.String(), filename)
}

// DeleteTaskFiles removes all files associated with a task
func DeleteTaskFiles(taskID uuid.UUID) error {
	taskDir := filepath.Join(UploadDir, taskID.String())
	return os.RemoveAll(taskDir)
}
