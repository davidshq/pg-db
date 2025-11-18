package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

// ImportStats tracks import statistics
type ImportStats struct {
	TotalFiles int
	Processed  int
	Successful int
	Failed     int
	Skipped    int
	Errors     []string
	mu         sync.Mutex
}

// NewImportStats creates a new ImportStats instance
func NewImportStats(totalFiles int) *ImportStats {
	return &ImportStats{
		TotalFiles: totalFiles,
		Errors:     make([]string, 0),
	}
}

// RecordSuccess records a successful import
func (s *ImportStats) RecordSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Processed++
	s.Successful++
}

// RecordFailure records a failed import
func (s *ImportStats) RecordFailure(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Processed++
	s.Failed++
	if err != nil {
		s.Errors = append(s.Errors, err.Error())
		if len(s.Errors) > 100 {
			s.Errors = s.Errors[len(s.Errors)-100:] // Keep only last 100 errors
		}
	}
}

// RecordSkipped records a skipped import
func (s *ImportStats) RecordSkipped() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Processed++
	s.Skipped++
}

// Importer handles the import process
type Importer struct {
	db        *DB
	batchSize int
	workers   int
	resume    bool
	stats     *ImportStats
}

// NewImporter creates a new Importer instance
func NewImporter(db *DB, batchSize, workers int, resume bool) *Importer {
	return &Importer{
		db:        db,
		batchSize: batchSize,
		workers:   workers,
		resume:    resume,
	}
}

// Import processes RDF files and imports them into the database
func (imp *Importer) Import(rdfFiles []string) error {
	imp.stats = NewImportStats(len(rdfFiles))

	// Create progress bar
	bar := progressbar.Default(int64(len(rdfFiles)), "Importing books")

	// Create worker pool
	fileChan := make(chan string, imp.workers)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < imp.workers; i++ {
		wg.Add(1)
		go imp.worker(fileChan, bar, &wg)
	}

	// Send files to workers
	for _, file := range rdfFiles {
		fileChan <- file
	}
	close(fileChan)

	// Wait for all workers to complete
	wg.Wait()
	bar.Finish()

	// Print summary
	imp.printSummary()

	return nil
}

// worker processes files from the channel
func (imp *Importer) worker(fileChan <-chan string, bar *progressbar.ProgressBar, wg *sync.WaitGroup) {
	defer wg.Done()

	batch := make([]*Book, 0, imp.batchSize)

	for filePath := range fileChan {
		// Parse RDF file
		book, err := ParseRDFFile(filePath)

		// Check if we should skip this file (after parsing to avoid double parse)
		if imp.resume && err == nil && book != nil && book.GutenbergID != "" {
			exists, checkErr := imp.db.BookExists(book.GutenbergID)
			if checkErr == nil && exists {
				imp.stats.RecordSkipped()
				bar.Add(1)
				continue
			}
		}
		if err != nil {
			imp.stats.RecordFailure(fmt.Errorf("failed to parse %s: %w", filePath, err))
			bar.Add(1)
			continue
		}

		// Validate book has at least a Gutenberg ID
		if book.GutenbergID == "" {
			imp.stats.RecordFailure(fmt.Errorf("no Gutenberg ID found in %s", filePath))
			bar.Add(1)
			continue
		}

		batch = append(batch, book)

		// Insert batch when it reaches the batch size
		if len(batch) >= imp.batchSize {
			imp.insertBatch(batch)
			batch = batch[:0] // Reset batch
		}

		bar.Add(1)
	}

	// Insert remaining books in batch
	if len(batch) > 0 {
		imp.insertBatch(batch)
	}
}

// insertBatch inserts a batch of books
func (imp *Importer) insertBatch(batch []*Book) {
	for _, book := range batch {
		if err := imp.db.InsertBook(book); err != nil {
			imp.stats.RecordFailure(fmt.Errorf("failed to insert book %s: %w", book.GutenbergID, err))
		} else {
			imp.stats.RecordSuccess()
		}
	}
}

// printSummary prints import statistics
func (imp *Importer) printSummary() {
	fmt.Printf("\n\nImport Summary:\n")
	fmt.Printf("===============\n")
	fmt.Printf("Total files:     %d\n", imp.stats.TotalFiles)
	fmt.Printf("Processed:       %d\n", imp.stats.Processed)
	fmt.Printf("Successful:      %d\n", imp.stats.Successful)
	fmt.Printf("Failed:          %d\n", imp.stats.Failed)
	fmt.Printf("Skipped:         %d\n", imp.stats.Skipped)
	if imp.stats.Processed > 0 {
		fmt.Printf("Success rate:    %.2f%%\n", float64(imp.stats.Successful)/float64(imp.stats.Processed)*100)
	} else {
		fmt.Printf("Success rate:    N/A (no files processed)\n")
	}

	if len(imp.stats.Errors) > 0 {
		fmt.Printf("\nRecent errors (%d shown):\n", len(imp.stats.Errors))
		for i, err := range imp.stats.Errors {
			if i >= 10 {
				fmt.Printf("... and %d more errors\n", len(imp.stats.Errors)-10)
				break
			}
			fmt.Printf("  - %s\n", err)
		}
	}
}

// ImportWithProgress is an alternative import function with detailed progress
func (imp *Importer) ImportWithProgress(rdfFiles []string) error {
	startTime := time.Now()
	imp.stats = NewImportStats(len(rdfFiles))

	// Create progress bar with more details
	bar := progressbar.NewOptions(
		len(rdfFiles),
		progressbar.OptionSetDescription("Importing books"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionOnCompletion(func() {
			fmt.Print("\n")
		}),
	)

	// Process files
	for _, filePath := range rdfFiles {
		// Check if we should skip this file
		if imp.resume {
			book, err := ParseRDFFile(filePath)
			if err == nil && book.GutenbergID != "" {
				exists, err := imp.db.BookExists(book.GutenbergID)
				if err == nil && exists {
					imp.stats.RecordSkipped()
					bar.Add(1)
					continue
				}
			}
		}

		// Parse and insert
		book, err := ParseRDFFile(filePath)
		if err != nil {
			imp.stats.RecordFailure(fmt.Errorf("failed to parse %s: %w", filePath, err))
			bar.Add(1)
			continue
		}

		if book.GutenbergID == "" {
			imp.stats.RecordFailure(fmt.Errorf("no Gutenberg ID found in %s", filePath))
			bar.Add(1)
			continue
		}

		if err := imp.db.InsertBook(book); err != nil {
			imp.stats.RecordFailure(fmt.Errorf("failed to insert book %s: %w", book.GutenbergID, err))
		} else {
			imp.stats.RecordSuccess()
		}

		bar.Add(1)
	}

	bar.Finish()

	elapsed := time.Since(startTime)
	fmt.Printf("\nImport completed in %s\n", elapsed)
	imp.printSummary()

	return nil
}
