# Setup Instructions for Building and Running

## Prerequisites

- **Go 1.19 or later**


## Quick Start

### Build the Application

```bash
go build -o pg-importer.exe .
```

Or on Unix/Linux:
```bash
go build -o pg-importer .
```

### Run the Application

```bash
# Windows
.\pg-importer.exe --zip rdf-files.tar.zip --db pg.db

# Unix/Linux
./pg-importer --zip rdf-files.tar.zip --db pg.db
```

## Installation Steps

1. **Install Go** (if not already installed):
   - Download from: https://golang.org/dl/
   - Follow installation instructions for your platform

2. **Clone or download this repository**

3. **Install dependencies**:
   ```bash
   go mod download
   ```

4. **Build the application**:
   ```bash
   go build -o pg-importer.exe .
   ```

5. **Run the importer**:
   ```bash
   .\pg-importer.exe --zip rdf-files.tar.zip --db pg.db
   ```

## Verify Installation

Test that everything works:

```bash
# Test with a small batch
.\pg-importer.exe --zip rdf-files.tar.zip --db test.db --workers 2 --batch-size 10

# Verify the database was created and populated
go run verify_db.go test.db
```


## Troubleshooting

### Build Errors

If you encounter build errors:
1. Ensure Go is properly installed: `go version`
2. Update dependencies: `go mod tidy`
3. Clean build: `go clean -cache && go build`

### Runtime Errors

- **"zip file not found"**: Ensure the RDF zip file path is correct
- **"database is locked"**: Another process is using the database, wait for it to finish
- **Import seems slow**: Adjust `--workers` and `--batch-size` flags for your system

## Performance Tips

- Use `--workers` to control parallelism (default: 4)
- Use `--batch-size` to control transaction size (default: 1000)
- For large imports, consider using `--resume` to continue interrupted imports
- The application processes ~2000+ files/second on modern hardware
