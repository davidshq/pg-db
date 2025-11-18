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
// Files are extracted to a permanent directory and will be reused on subsequent runs.
// Returns a list of paths to extracted RDF files and a no-op cleanup function.
func ExtractRDFFiles(zipPath string) ([]string, func(), error) {
	// Create a permanent directory name based on the zip file name
	zipBaseName := filepath.Base(zipPath)
	zipNameWithoutExt := strings.TrimSuffix(zipBaseName, filepath.Ext(zipBaseName))
	extractDir := zipNameWithoutExt + "-extracted"

	// Check if files are already extracted
	if entries, err := os.ReadDir(extractDir); err == nil && len(entries) > 0 {
		// Directory exists and has files, return existing files
		var rdfFiles []string
		err := filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info != nil && !info.IsDir() && strings.HasSuffix(path, ".rdf") {
				rdfFiles = append(rdfFiles, path)
			}
			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read existing extracted files: %w", err)
		}
		if len(rdfFiles) > 0 {
			// Return existing files with a no-op cleanup function
			return rdfFiles, func() {}, nil
		}
	}

	// Create extraction directory if it doesn't exist
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create extraction directory: %w", err)
	}

	// No-op cleanup function since we want to keep the files
	cleanup := func() {}

	// Open zip file
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
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
		return nil, nil, fmt.Errorf("no tar file found in zip archive")
	}

	// Extract tar file
	tarReader, err := tarFile.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open tar file: %w", err)
	}
	defer tarReader.Close()

	// Determine if it's gzipped
	var rdfFiles []string
	if strings.HasSuffix(tarFile.Name, ".tar.gz") || strings.HasSuffix(tarFile.Name, ".tgz") {
		// Handle gzipped tar
		gzReader, err := gzip.NewReader(tarReader)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()

		rdfFiles, err = extractTar(gzReader, extractDir)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to extract tar: %w", err)
		}
	} else {
		// Handle regular tar
		rdfFiles, err = extractTar(tarReader, extractDir)
		if err != nil {
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
