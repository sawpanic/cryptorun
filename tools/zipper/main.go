package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <source_dir> <output_zip>\n", os.Args[0])
		os.Exit(1)
	}

	sourceDir := os.Args[1]
	outputZip := os.Args[2]

	if err := zipDirectory(sourceDir, outputZip); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully created zip: %s\n", outputZip)
}

func zipDirectory(sourceDir, outputZip string) error {
	// Create output zip file
	zipFile, err := os.Create(outputZip)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	// Create zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through source directory
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path for zip entry
		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Normalize path separators for cross-platform compatibility
		relativePath = strings.ReplaceAll(relativePath, "\\", "/")

		// Create zip entry
		zipEntry, err := zipWriter.Create(relativePath)
		if err != nil {
			return fmt.Errorf("failed to create zip entry: %w", err)
		}

		// Open source file
		sourceFile, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		defer sourceFile.Close()

		// Copy file contents to zip entry
		_, err = io.Copy(zipEntry, sourceFile)
		if err != nil {
			return fmt.Errorf("failed to copy file contents: %w", err)
		}

		return nil
	})
}
