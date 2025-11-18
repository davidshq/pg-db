package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// VerifyDB verifies that the database was created and populated correctly
func VerifyDB(dbPath string) {
	if dbPath == "" {
		fmt.Println("Error: database path is required")
		os.Exit(1)
	}

	fmt.Printf("Verifying database: %s\n\n", dbPath)

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Check if database exists and has tables
	tables := []string{"books", "authors", "subjects", "book_authors", "book_subjects", "formats"}

	fmt.Println("Checking tables:")
	for _, table := range tables {
		var count int
		err := conn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			fmt.Printf("  ❌ %s: ERROR - %v\n", table, err)
		} else {
			fmt.Printf("  ✅ %s: %d records\n", table, count)
		}
	}

	// Get some statistics
	fmt.Println("\nDatabase Statistics:")

	var totalBooks int
	conn.QueryRow("SELECT COUNT(*) FROM books").Scan(&totalBooks)
	fmt.Printf("  Total books: %d\n", totalBooks)

	var totalAuthors int
	conn.QueryRow("SELECT COUNT(*) FROM authors").Scan(&totalAuthors)
	fmt.Printf("  Total authors: %d\n", totalAuthors)

	var totalSubjects int
	conn.QueryRow("SELECT COUNT(*) FROM subjects").Scan(&totalSubjects)
	fmt.Printf("  Total subjects: %d\n", totalSubjects)

	var totalFormats int
	conn.QueryRow("SELECT COUNT(*) FROM formats").Scan(&totalFormats)
	fmt.Printf("  Total formats: %d\n", totalFormats)

	// Sample some books
	fmt.Println("\nSample books (first 5):")
	rows, err := conn.Query(`
		SELECT b.gutenberg_id, b.title, 
		       GROUP_CONCAT(a.name, ', ') as authors
		FROM books b
		LEFT JOIN book_authors ba ON b.id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.id
		GROUP BY b.id
		LIMIT 5
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title, authors sql.NullString
			rows.Scan(&id, &title, &authors)
			fmt.Printf("  ID: %s | Title: %s | Authors: %s\n",
				id.String, title.String, authors.String)
		}
	}

	fmt.Println("\nDatabase verification complete!")
}
