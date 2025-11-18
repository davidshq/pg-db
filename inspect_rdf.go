package main

import (
	"fmt"
	"os"
	"strings"
)

// InspectRDF inspects RDF files from the archive and displays their structure
func InspectRDF() {
	rdfFiles, cleanup, err := ExtractRDFFiles("rdf-files.tar.zip")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	if len(rdfFiles) == 0 {
		fmt.Println("No RDF files found")
		os.Exit(1)
	}

	// Read first RDF file
	content, err := os.ReadFile(rdfFiles[0])
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Show more content and search for subject/format
	text := string(content)

	// Find subject and format sections
	subjectIdx := strings.Index(text, "subject")
	formatIdx := strings.Index(text, "format")

	fmt.Printf("Sample RDF file (%s):\n", rdfFiles[0])
	fmt.Println(strings.Repeat("=", 80))

	// Show context around subject if found
	if subjectIdx > 0 {
		start := subjectIdx - 200
		if start < 0 {
			start = 0
		}
		end := subjectIdx + 500
		if end > len(text) {
			end = len(text)
		}
		fmt.Println("SUBJECT SECTION:")
		fmt.Println(text[start:end])
		fmt.Println(strings.Repeat("-", 80))
	}

	// Show context around format if found
	if formatIdx > 0 {
		start := formatIdx - 200
		if start < 0 {
			start = 0
		}
		end := formatIdx + 500
		if end > len(text) {
			end = len(text)
		}
		fmt.Println("FORMAT SECTION:")
		fmt.Println(text[start:end])
		fmt.Println(strings.Repeat("-", 80))
	}

	// Show first 5000 chars to see full structure
	if len(text) > 5000 {
		fmt.Println("FIRST 5000 CHARS:")
		fmt.Println(text[:5000])
	} else {
		fmt.Println(text)
	}
	fmt.Println(strings.Repeat("=", 80))

	// Try parsing it
	book, err := ParseRDFFile(rdfFiles[0])
	if err != nil {
		fmt.Printf("\nParse error: %v\n", err)
	} else {
		fmt.Printf("\nParsed book:\n")
		fmt.Printf("  ID: %s\n", book.GutenbergID)
		fmt.Printf("  Title: %s\n", book.Title)
		fmt.Printf("  Authors: %d\n", len(book.Authors))
		fmt.Printf("  Subjects: %d\n", len(book.Subjects))
		fmt.Printf("  Formats: %d\n", len(book.Formats))
	}
}
