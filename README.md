# Project Gutenberg RDF to SQLite Importer

A Go CLI application that extracts and imports Project Gutenberg RDF metadata files into a normalized SQLite database.

## Features

- Extracts RDF files from zip/tar archives
- Parses RDF/XML metadata (titles, authors, subjects, formats, etc.)
- Imports data into a normalized SQLite database
- Batch processing with configurable batch size
- Concurrent processing with worker pool
- Progress tracking with progress bar
- Resume capability (skip already imported books)
- Comprehensive error handling and statistics

## Installation

### Prerequisites

- **Go 1.24.4 or later**

### Build

```bash
go build -o pg-importer.exe .
```

Or on Unix/Linux:
```bash
go build -o pg-importer .
```

The application uses a pure Go SQLite driver, so no CGO or C compiler is needed. This makes building and cross-compiling much easier.

### Install Dependencies

```bash
go mod download
```

## Usage

### Basic Usage

```bash
# Windows
.\pg-importer.exe

# Unix/Linux
./pg-importer
```

This will:
- Extract RDF files from `rdf-files.tar.zip`
- Create/use database `pg.db`
- Import all books with default settings

### Command-Line Options

```bash
.\pg-importer.exe [options]
```

Options:

- `--db <path>` - Path to SQLite database file (default: `pg.db`)
- `--zip <path>` - Path to RDF zip file (default: `rdf-files.tar.zip`)
- `--batch-size <n>` - Number of records per batch (default: 1000)
- `--workers <n>` - Number of concurrent workers (default: 4)
- `--resume` - Skip already imported books

### Examples

Import with custom database path:

```bash
.\pg-importer.exe --db /path/to/pg.db
```

Import with custom zip file:

```bash
.\pg-importer.exe --zip /path/to/rdf-files.tar.zip
```

Resume import (skip existing books):

```bash
.\pg-importer.exe --resume
```

Custom batch size and workers:

```bash
.\pg-importer.exe --batch-size 500 --workers 8
```

### Verify Import

After importing, verify the database:

```bash
go run verify_db.go pg.db
```

Or use the default database:

```bash
go run verify_db.go
```

## Database Schema

The application creates a normalized database schema with the following tables:

### books

Core book metadata.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| gutenberg_id | TEXT | Project Gutenberg ebook ID (unique) |
| title | TEXT | Book title |
| language | TEXT | Language code |
| publisher | TEXT | Publisher information |
| license | TEXT | License information |
| rights | TEXT | Rights information |
| issued_date | TEXT | Publication/issue date |
| download_count | INTEGER | Number of downloads |
| description | TEXT | Book description |
| summary | TEXT | Book summary (MARC 520) |
| production_notes | TEXT | Production notes (MARC 508) |
| reading_ease_score | TEXT | Reading ease score (MARC 908) |
| created_at | TIMESTAMP | Record creation timestamp |

### authors

Author information.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| name | TEXT | Author name |
| first_name | TEXT | First name (nullable) |
| last_name | TEXT | Last name (nullable) |
| agent_id | TEXT | Agent ID from RDF (nullable) |
| alias | TEXT | Author aliases (nullable) |
| webpage | TEXT | Author webpage URLs (nullable) |
| birth_year | INTEGER | Birth year (nullable) |
| death_year | INTEGER | Death year (nullable) |
| created_at | TIMESTAMP | Record creation timestamp |

### book_authors

Many-to-many relationship between books and authors.

| Column | Type | Description |
|--------|------|-------------|
| book_id | INTEGER | Foreign key to books.id |
| author_id | INTEGER | Foreign key to authors.id |

### subjects

Subject/topic categories.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| subject | TEXT | Subject name (unique) |
| created_at | TIMESTAMP | Record creation timestamp |

### book_subjects

Many-to-many relationship between books and subjects.

| Column | Type | Description |
|--------|------|-------------|
| book_id | INTEGER | Foreign key to books.id |
| subject_id | INTEGER | Foreign key to subjects.id |

### bookshelves

Bookshelf/category classifications.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| bookshelf | TEXT | Bookshelf name (unique) |
| created_at | TIMESTAMP | Record creation timestamp |

### book_bookshelves

Many-to-many relationship between books and bookshelves.

| Column | Type | Description |
|--------|------|-------------|
| book_id | INTEGER | Foreign key to books.id |
| bookshelf_id | INTEGER | Foreign key to bookshelves.id |

### formats

Available file formats for each book.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| book_id | INTEGER | Foreign key to books.id |
| format_type | TEXT | MIME type (e.g., "text/plain", "application/epub+zip") |
| file_url | TEXT | URL to the file |
| file_size | INTEGER | File size in bytes (nullable) |

## Example Queries

### Find all books by a specific author

```sql
SELECT b.title, b.gutenberg_id, a.name
FROM books b
JOIN book_authors ba ON b.id = ba.book_id
JOIN authors a ON ba.author_id = a.id
WHERE a.name LIKE '%Shakespeare%'
ORDER BY b.title;
```

### Find books by subject

```sql
SELECT b.title, b.gutenberg_id, s.subject
FROM books b
JOIN book_subjects bs ON b.id = bs.book_id
JOIN subjects s ON bs.subject_id = s.id
WHERE s.subject LIKE '%Fiction%'
ORDER BY b.download_count DESC
LIMIT 10;
```

### Find most downloaded books

```sql
SELECT title, gutenberg_id, download_count
FROM books
ORDER BY download_count DESC
LIMIT 20;
```

### Find available formats for a book

```sql
SELECT b.title, f.format_type, f.file_url, f.file_size
FROM books b
JOIN formats f ON b.id = f.book_id
WHERE b.gutenberg_id = '12345';
```

### Find books by bookshelf

```sql
SELECT b.title, b.gutenberg_id, bs.bookshelf
FROM books b
JOIN book_bookshelves bbs ON b.id = bbs.book_id
JOIN bookshelves bs ON bbs.bookshelf_id = bs.id
WHERE bs.bookshelf LIKE '%Fiction%'
ORDER BY b.download_count DESC
LIMIT 10;
```

## Performance Considerations

- **Batch Size**: Larger batch sizes reduce transaction overhead but use more memory. Default (1000) is a good balance.
- **Workers**: More workers increase parallelism but also database contention. Default (4) works well for most systems.
- **WAL Mode**: The database uses Write-Ahead Logging (WAL) mode for better concurrent performance.
- **Indexes**: Foreign keys and frequently queried columns are indexed for optimal query performance.
- **Processing Speed**: The application processes approximately 2000+ RDF files per second on modern hardware.

## Error Handling

The application handles errors gracefully:

- Invalid RDF files are logged and skipped
- Database errors are logged but don't stop the import
- A summary of errors is displayed at the end
- Up to 100 recent errors are kept in memory for reporting

## Technical Details

### SQLite Driver

The application uses `modernc.org/sqlite`, a pure Go implementation of SQLite that:
- Requires no C compiler or CGO
- Enables easy cross-compilation
- Provides comparable performance to CGO-based drivers
- Results in a single, self-contained binary

### RDF Parsing

The parser handles Project Gutenberg's RDF/XML format, extracting:
- Book metadata (title, language, publisher, license, rights, issue date, download count, description, summary, production notes, reading ease score)
- Author information (name, first name, last name, agent ID, aliases, webpages, birth/death years)
- Subject classifications
- Bookshelf/category classifications
- Available file formats with URLs and sizes

### Limitations

- The parser expects RDF/XML format as used by Project Gutenberg
- Some RDF files may have variations in structure that aren't fully handled
- Very large archives may require significant disk space for temporary extraction
- The database uses a single connection (SQLite best practice) which serializes writes

## License

This project is provided as-is for processing Project Gutenberg metadata.

## Contributing

Contributions are welcome! Please ensure code follows Go best practices and includes appropriate tests.

