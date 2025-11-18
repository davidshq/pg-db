package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractRDFFiles extracts RDF files from the zip archive (which contains a tar file)
// Returns a list of paths to extracted RDF files and a cleanup function
func ExtractRDFFiles(zipPath string) ([]string, func(), error) {
	tempDir, err := os.MkdirTemp("", "pg-rdf-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	// Open zip file
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer zipReader.Close()

	// Find the tar file inside the zip
	var tarFile *zip.File
	for _, file := range zipReader.File {
		if strings.HasSuffix(file.Name, ".tar") || strings.HasSuffix(file.Name, ".tar.gz") {
			tarFile = file
			break
		}
	}

	if tarFile == nil {
		cleanup()
		return nil, nil, fmt.Errorf("no tar file found in zip archive")
	}

	// Extract tar file
	tarReader, err := tarFile.Open()
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to open tar file: %w", err)
	}
	defer tarReader.Close()

	// Determine if it's gzipped
	var rdfFiles []string
	if strings.HasSuffix(tarFile.Name, ".tar.gz") || strings.HasSuffix(tarFile.Name, ".tgz") {
		// Handle gzipped tar
		gzReader, err := gzip.NewReader(tarReader)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()

		rdfFiles, err = extractTar(gzReader, tempDir)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed to extract tar: %w", err)
		}
	} else {
		// Handle regular tar
		rdfFiles, err = extractTar(tarReader, tempDir)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed to extract tar: %w", err)
		}
	}

	return rdfFiles, cleanup, nil
}

// extractTar extracts files from a tar archive and returns paths to RDF files
func extractTar(reader io.Reader, destDir string) ([]string, error) {
	tarReader := tar.NewReader(reader)
	var rdfFiles []string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Only process RDF files
		if !strings.HasSuffix(header.Name, ".rdf") {
			continue
		}

		// Create the full path for the file, using a sanitized version of the full path
		// to avoid collisions while keeping some directory structure
		sanitizedName := strings.ReplaceAll(header.Name, "/", "_")
		sanitizedName = strings.ReplaceAll(sanitizedName, "\\", "_")
		targetPath := filepath.Join(destDir, sanitizedName)

		// Create parent directories if needed
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		// Extract the file
		outFile, err := os.Create(targetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %w", err)
		}

		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			return nil, fmt.Errorf("failed to write file: %w", err)
		}

		outFile.Close()
		rdfFiles = append(rdfFiles, targetPath)
	}

	return rdfFiles, nil
}
