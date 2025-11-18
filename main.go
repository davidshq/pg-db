package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	// Parse command-line flags
	dbPath := flag.String("db", "pg.db", "Path to SQLite database file")
	zipPath := flag.String("zip", "rdf-files.tar.zip", "Path to RDF zip file")
	batchSize := flag.Int("batch-size", 1000, "Number of records per batch")
	workers := flag.Int("workers", 4, "Number of concurrent workers")
	resume := flag.Bool("resume", false, "Skip already imported books")
	flag.Parse()

	// Validate inputs
	if *zipPath == "" {
		log.Fatal("Error: zip file path is required")
	}

	if _, err := os.Stat(*zipPath); os.IsNotExist(err) {
		log.Fatalf("Error: zip file not found: %s", *zipPath)
	}

	if *batchSize <= 0 {
		log.Fatal("Error: batch-size must be greater than 0")
	}

	if *workers <= 0 {
		log.Fatal("Error: workers must be greater than 0")
	}

	// Initialize database
	fmt.Printf("Initializing database: %s\n", *dbPath)
	db, err := NewDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Extract RDF files
	fmt.Printf("Extracting RDF files from: %s\n", *zipPath)
	rdfFiles, cleanup, err := ExtractRDFFiles(*zipPath)
	if err != nil {
		log.Fatalf("Failed to extract RDF files: %v", err)
	}
	defer cleanup()

	fmt.Printf("Found %d RDF files\n", len(rdfFiles))

	if len(rdfFiles) == 0 {
		log.Fatal("No RDF files found in archive")
	}

	// Create importer
	importer := NewImporter(db, *batchSize, *workers, *resume)

	// Import files
	fmt.Printf("Starting import with %d workers, batch size %d\n", *workers, *batchSize)
	if *resume {
		fmt.Println("Resume mode: skipping already imported books")
	}

	// Use the concurrent import method
	err = importer.Import(rdfFiles)
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	fmt.Println("\nImport completed successfully!")
}
