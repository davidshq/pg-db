package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the database connection and provides methods for database operations
type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection and initializes the schema
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	// SQLite works best with a single connection or very few connections
	// due to its file-level locking model. Using too many connections causes contention.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0) // Connections don't expire

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Run migrations to add new columns if they don't exist
	if err := db.migrateSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// initSchema creates all necessary tables and indexes
func (db *DB) initSchema() error {
	schema := `
	-- Books table
	CREATE TABLE IF NOT EXISTS books (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		gutenberg_id TEXT UNIQUE NOT NULL,
		title TEXT,
		language TEXT,
		publisher TEXT,
		license TEXT,
		rights TEXT,
		issued_date TEXT,
		download_count INTEGER DEFAULT 0,
		description TEXT,
		summary TEXT,
		production_notes TEXT,
		reading_ease_score TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Authors table
	CREATE TABLE IF NOT EXISTS authors (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		first_name TEXT,
		last_name TEXT,
		agent_id TEXT,
		alias TEXT,
		webpage TEXT,
		birth_year INTEGER,
		death_year INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Unique constraint on author name and dates
	CREATE UNIQUE INDEX IF NOT EXISTS idx_authors_unique ON authors(name, birth_year, death_year);
	
	-- Indexes for author name parts
	CREATE INDEX IF NOT EXISTS idx_authors_first_name ON authors(first_name);
	CREATE INDEX IF NOT EXISTS idx_authors_last_name ON authors(last_name);

	-- Book-Author relationship table
	CREATE TABLE IF NOT EXISTS book_authors (
		book_id INTEGER NOT NULL,
		author_id INTEGER NOT NULL,
		PRIMARY KEY (book_id, author_id),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
		FOREIGN KEY (author_id) REFERENCES authors(id) ON DELETE CASCADE
	);

	-- Subjects table
	CREATE TABLE IF NOT EXISTS subjects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subject TEXT UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Book-Subject relationship table
	CREATE TABLE IF NOT EXISTS book_subjects (
		book_id INTEGER NOT NULL,
		subject_id INTEGER NOT NULL,
		PRIMARY KEY (book_id, subject_id),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
		FOREIGN KEY (subject_id) REFERENCES subjects(id) ON DELETE CASCADE
	);

	-- Bookshelves table
	CREATE TABLE IF NOT EXISTS bookshelves (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bookshelf TEXT UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Book-Bookshelf relationship table
	CREATE TABLE IF NOT EXISTS book_bookshelves (
		book_id INTEGER NOT NULL,
		bookshelf_id INTEGER NOT NULL,
		PRIMARY KEY (book_id, bookshelf_id),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
		FOREIGN KEY (bookshelf_id) REFERENCES bookshelves(id) ON DELETE CASCADE
	);

	-- Formats table
	CREATE TABLE IF NOT EXISTS formats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		book_id INTEGER NOT NULL,
		format_type TEXT NOT NULL,
		file_url TEXT,
		file_size INTEGER,
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	);

	-- Indexes for performance
	CREATE INDEX IF NOT EXISTS idx_books_gutenberg_id ON books(gutenberg_id);
	CREATE INDEX IF NOT EXISTS idx_authors_name ON authors(name);
	CREATE INDEX IF NOT EXISTS idx_book_authors_book_id ON book_authors(book_id);
	CREATE INDEX IF NOT EXISTS idx_book_authors_author_id ON book_authors(author_id);
	CREATE INDEX IF NOT EXISTS idx_book_subjects_book_id ON book_subjects(book_id);
	CREATE INDEX IF NOT EXISTS idx_book_subjects_subject_id ON book_subjects(subject_id);
	CREATE INDEX IF NOT EXISTS idx_book_bookshelves_book_id ON book_bookshelves(book_id);
	CREATE INDEX IF NOT EXISTS idx_book_bookshelves_bookshelf_id ON book_bookshelves(bookshelf_id);
	CREATE INDEX IF NOT EXISTS idx_authors_agent_id ON authors(agent_id);
	CREATE INDEX IF NOT EXISTS idx_formats_book_id ON formats(book_id);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// migrateSchema adds new columns to existing tables if they don't exist
func (db *DB) migrateSchema() error {
	migrations := []string{
		// Add new author columns if they don't exist
		`ALTER TABLE authors ADD COLUMN first_name TEXT`,
		`ALTER TABLE authors ADD COLUMN last_name TEXT`,
		`ALTER TABLE authors ADD COLUMN agent_id TEXT`,
		`ALTER TABLE authors ADD COLUMN alias TEXT`,
		`ALTER TABLE authors ADD COLUMN webpage TEXT`,
		// Add new book columns if they don't exist
		`ALTER TABLE books ADD COLUMN publisher TEXT`,
		`ALTER TABLE books ADD COLUMN license TEXT`,
		`ALTER TABLE books ADD COLUMN description TEXT`,
		`ALTER TABLE books ADD COLUMN summary TEXT`,
		`ALTER TABLE books ADD COLUMN production_notes TEXT`,
		`ALTER TABLE books ADD COLUMN reading_ease_score TEXT`,
	}

	for _, migration := range migrations {
		// SQLite doesn't support IF NOT EXISTS for ALTER TABLE ADD COLUMN,
		// so we need to check if the column exists first
		_, err := db.conn.Exec(migration)
		if err != nil {
			// Ignore "duplicate column name" errors (column already exists)
			// SQLite returns error code 1 for this
			if !strings.Contains(err.Error(), "duplicate column") {
				// For other errors, log but don't fail (might be column already exists)
				log.Printf("Migration warning (may be safe to ignore): %v", err)
			}
		}
	}

	// Create indexes if they don't exist
	indexMigrations := []string{
		`CREATE INDEX IF NOT EXISTS idx_authors_first_name ON authors(first_name)`,
		`CREATE INDEX IF NOT EXISTS idx_authors_last_name ON authors(last_name)`,
		`CREATE INDEX IF NOT EXISTS idx_authors_agent_id ON authors(agent_id)`,
	}

	for _, migration := range indexMigrations {
		if _, err := db.conn.Exec(migration); err != nil {
			log.Printf("Index creation warning: %v", err)
		}
	}

	return nil
}

// Book represents a book record
type Book struct {
	GutenbergID      string
	Title            string
	Language         string
	Publisher        string
	License          string
	Rights           string
	IssuedDate       string
	DownloadCount    int
	Description      string
	Summary          string
	ProductionNotes  string
	ReadingEaseScore string
	Authors          []Author
	Subjects         []string
	Bookshelves      []string
	Formats          []Format
}

// Author represents an author record
type Author struct {
	Name      string
	FirstName string
	LastName  string
	AgentID   string
	Alias     string
	Webpage   string
	BirthYear *int
	DeathYear *int
}

// Format represents a file format for a book
type Format struct {
	Type     string
	FileURL  string
	FileSize *int64
}

// BookExists checks if a book with the given Gutenberg ID already exists
func (db *DB) BookExists(gutenbergID string) (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM books WHERE gutenberg_id = ?", gutenbergID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// InsertBook inserts a book and all related data in a transaction
func (db *DB) InsertBook(book *Book) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert or update book (preserve created_at for existing books)
	_, err = tx.Exec(`
		INSERT INTO books (gutenberg_id, title, language, publisher, license, rights, issued_date, download_count, description, summary, production_notes, reading_ease_score, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(gutenberg_id) DO UPDATE SET
			title = excluded.title,
			language = excluded.language,
			publisher = excluded.publisher,
			license = excluded.license,
			rights = excluded.rights,
			issued_date = excluded.issued_date,
			download_count = excluded.download_count,
			description = excluded.description,
			summary = excluded.summary,
			production_notes = excluded.production_notes,
			reading_ease_score = excluded.reading_ease_score
	`, book.GutenbergID, book.Title, book.Language, book.Publisher, book.License, book.Rights, book.IssuedDate, book.DownloadCount, book.Description, book.Summary, book.ProductionNotes, book.ReadingEaseScore, time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert book: %w", err)
	}

	// Get the book ID (works for both INSERT and REPLACE)
	var bookID int64
	err = tx.QueryRow("SELECT id FROM books WHERE gutenberg_id = ?", book.GutenbergID).Scan(&bookID)
	if err != nil {
		return fmt.Errorf("failed to get book ID: %w", err)
	}

	// Insert authors
	for _, author := range book.Authors {
		var authorID int64
		// Check if author exists - use COALESCE for NULL-safe comparison
		var existingID sql.NullInt64
		err := tx.QueryRow(`
			SELECT id FROM authors 
			WHERE name = ? AND 
			      COALESCE(birth_year, -1) = COALESCE(?, -1) AND
			      COALESCE(death_year, -1) = COALESCE(?, -1)
		`, author.Name, author.BirthYear, author.DeathYear).Scan(&existingID)

		if err == nil && existingID.Valid {
			// Author exists, use existing ID
			authorID = existingID.Int64

			// Update author fields if they're not already set
			_, err = tx.Exec(`
				UPDATE authors 
				SET first_name = COALESCE(NULLIF(?, ''), first_name),
				    last_name = COALESCE(NULLIF(?, ''), last_name),
				    agent_id = COALESCE(NULLIF(?, ''), agent_id),
				    alias = COALESCE(NULLIF(?, ''), alias),
				    webpage = COALESCE(NULLIF(?, ''), webpage)
				WHERE id = ?
			`, author.FirstName, author.LastName, author.AgentID, author.Alias, author.Webpage, authorID)
			if err != nil {
				return fmt.Errorf("failed to update author: %w", err)
			}
		} else if err == sql.ErrNoRows {
			// Insert new author
			result, err := tx.Exec(`
				INSERT INTO authors (name, first_name, last_name, agent_id, alias, webpage, birth_year, death_year, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, author.Name, author.FirstName, author.LastName, author.AgentID, author.Alias, author.Webpage, author.BirthYear, author.DeathYear, time.Now())
			if err != nil {
				return fmt.Errorf("failed to insert author: %w", err)
			}
			authorID, err = result.LastInsertId()
			if err != nil {
				return fmt.Errorf("failed to get author ID: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to query author: %w", err)
		}

		_, err = tx.Exec(`
			INSERT OR IGNORE INTO book_authors (book_id, author_id)
			VALUES (?, ?)
		`, bookID, authorID)
		if err != nil {
			return fmt.Errorf("failed to link author: %w", err)
		}
	}

	// Insert subjects
	for _, subject := range book.Subjects {
		var subjectID int64
		// Try to get existing subject ID
		err := tx.QueryRow("SELECT id FROM subjects WHERE subject = ?", subject).Scan(&subjectID)
		if err == sql.ErrNoRows {
			// Insert new subject
			result, err := tx.Exec(`
				INSERT INTO subjects (subject, created_at)
				VALUES (?, ?)
			`, subject, time.Now())
			if err != nil {
				return fmt.Errorf("failed to insert subject: %w", err)
			}
			subjectID, err = result.LastInsertId()
			if err != nil {
				return fmt.Errorf("failed to get subject ID: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to query subject: %w", err)
		}

		_, err = tx.Exec(`
			INSERT OR IGNORE INTO book_subjects (book_id, subject_id)
			VALUES (?, ?)
		`, bookID, subjectID)
		if err != nil {
			return fmt.Errorf("failed to link subject: %w", err)
		}
	}

	// Delete existing formats for this book (to avoid duplicates on re-import)
	// Only delete if we have new formats to insert, otherwise preserve existing formats
	if len(book.Formats) > 0 {
		_, err = tx.Exec("DELETE FROM formats WHERE book_id = ?", bookID)
		if err != nil {
			return fmt.Errorf("failed to delete existing formats: %w", err)
		}
	}

	// Insert bookshelves
	for _, bookshelf := range book.Bookshelves {
		var bookshelfID int64
		// Try to get existing bookshelf ID
		err := tx.QueryRow("SELECT id FROM bookshelves WHERE bookshelf = ?", bookshelf).Scan(&bookshelfID)
		if err == sql.ErrNoRows {
			// Insert new bookshelf
			result, err := tx.Exec(`
				INSERT INTO bookshelves (bookshelf, created_at)
				VALUES (?, ?)
			`, bookshelf, time.Now())
			if err != nil {
				return fmt.Errorf("failed to insert bookshelf: %w", err)
			}
			bookshelfID, err = result.LastInsertId()
			if err != nil {
				return fmt.Errorf("failed to get bookshelf ID: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to query bookshelf: %w", err)
		}

		_, err = tx.Exec(`
			INSERT OR IGNORE INTO book_bookshelves (book_id, bookshelf_id)
			VALUES (?, ?)
		`, bookID, bookshelfID)
		if err != nil {
			return fmt.Errorf("failed to link bookshelf: %w", err)
		}
	}

	// Insert formats
	for _, format := range book.Formats {
		_, err := tx.Exec(`
			INSERT INTO formats (book_id, format_type, file_url, file_size)
			VALUES (?, ?, ?, ?)
		`, bookID, format.Type, format.FileURL, format.FileSize)
		if err != nil {
			return fmt.Errorf("failed to insert format: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// BatchInsertBooks inserts multiple books in batches
func (db *DB) BatchInsertBooks(books []*Book, batchSize int) error {
	for i := 0; i < len(books); i += batchSize {
		end := i + batchSize
		if end > len(books) {
			end = len(books)
		}

		batch := books[i:end]
		for _, book := range batch {
			if err := db.InsertBook(book); err != nil {
				log.Printf("Error inserting book %s: %v", book.GutenbergID, err)
				// Continue with next book instead of failing entire batch
			}
		}
	}

	return nil
}
