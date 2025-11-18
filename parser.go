package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// RDFNamespaces defines the XML namespaces used in Project Gutenberg RDF files
var RDFNamespaces = map[string]string{
	"rdf":     "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
	"rdfs":    "http://www.w3.org/2000/01/rdf-schema#",
	"dcterms": "http://purl.org/dc/terms/",
	"pgterms": "http://www.gutenberg.org/2009/pgterms/",
	"dcam":    "http://purl.org/dc/dcam/",
}

// RDFDocument represents the parsed RDF document
type RDFDocument struct {
	XMLName xml.Name `xml:"RDF"`
	Ebook   *Ebook   `xml:"ebook"`
	Agents  []Agent  `xml:"agent"`
}

// Ebook represents the main pgterms:ebook element
type Ebook struct {
	About       string         `xml:"about,attr"`
	Title       string         `xml:"title"`
	Creator     []Creator      `xml:"creator"`
	Subject     []Subject      `xml:"subject"`
	Language    []Language     `xml:"language"`
	Rights      string         `xml:"rights"`
	Issued      string         `xml:"issued"`
	Downloads   string         `xml:"downloads"`
	Format      []RDFFormat    `xml:"hasFormat"`
	Publisher   string         `xml:"publisher"`
	License     LicenseElement `xml:"license"`
	Description []string       `xml:"description"`
	MARC508     string         `xml:"marc508"`
	MARC520     string         `xml:"marc520"`
	MARC908     string         `xml:"marc908"`
	Bookshelf   []Bookshelf    `xml:"bookshelf"`
}

// Bookshelf represents a pgterms:bookshelf element
type Bookshelf struct {
	Description *BookshelfDescription `xml:"Description"`
}

// BookshelfDescription represents the nested Description in bookshelf
type BookshelfDescription struct {
	Value string `xml:"value"`
}

// LicenseElement represents a dcterms:license element with resource attribute
type LicenseElement struct {
	Resource string `xml:"resource,attr"`
}

// Creator represents a creator element
type Creator struct {
	Agent *Agent `xml:"agent"`
}

// WebpageElement represents a pgterms:webpage element with resource attribute
type WebpageElement struct {
	Resource string `xml:"resource,attr"`
}

// Agent represents a pgterms:agent element
type Agent struct {
	About     string           `xml:"about,attr"`
	Name      string           `xml:"name"`
	BirthDate string           `xml:"birthdate"`
	DeathDate string           `xml:"deathdate"`
	Alias     []string         `xml:"alias"`
	Webpage   []WebpageElement `xml:"webpage"`
}

// Subject represents a subject element with nested Description
type Subject struct {
	Description *SubjectDescription `xml:"Description"`
}

// SubjectDescription represents the nested Description in subject
type SubjectDescription struct {
	Value string `xml:"value"`
}

// Language represents a language element
type Language struct {
	Description *LanguageDescription `xml:"Description"`
	Resource    string               `xml:"resource,attr"`
	Value       string               `xml:",chardata"`
}

// LanguageDescription represents the nested Description in language
type LanguageDescription struct {
	Value string `xml:"value"`
}

// RDFFormat represents a format element (dcterms:hasFormat with pgterms:file)
type RDFFormat struct {
	File *FileElement `xml:"file"`
}

// FileElement represents a pgterms:file element
type FileElement struct {
	About  string             `xml:"about,attr"`
	Extent string             `xml:"extent"`
	Format *FormatDescription `xml:"format>Description"`
}

// FormatDescription represents the nested Description in format
type FormatDescription struct {
	Value string `xml:"value"`
}

// ParseRDFFile parses an RDF/XML file and extracts book metadata
func ParseRDFFile(filePath string) (*Book, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return ParseRDF(file)
}

// ParseRDF parses RDF/XML content from a reader
func ParseRDF(reader io.Reader) (*Book, error) {
	decoder := xml.NewDecoder(reader)
	decoder.Strict = false // Be lenient with XML parsing

	var doc RDFDocument
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("failed to decode XML: %w", err)
	}

	book := &Book{
		Authors:     []Author{},
		Subjects:    []string{},
		Formats:     []Format{},
		Bookshelves: []string{},
	}

	if doc.Ebook == nil {
		return nil, fmt.Errorf("no ebook element found")
	}

	ebook := doc.Ebook

	// Extract Gutenberg ID
	if ebook.About != "" {
		book.GutenbergID = extractGutenbergID(ebook.About)
	}

	// Extract title
	book.Title = strings.TrimSpace(ebook.Title)

	// Extract publisher
	book.Publisher = strings.TrimSpace(ebook.Publisher)

	// Extract license
	book.License = strings.TrimSpace(ebook.License.Resource)

	// Extract rights
	book.Rights = strings.TrimSpace(ebook.Rights)

	// Extract issued date
	book.IssuedDate = strings.TrimSpace(ebook.Issued)

	// Extract download count
	if ebook.Downloads != "" {
		if count, err := strconv.Atoi(strings.TrimSpace(ebook.Downloads)); err == nil {
			book.DownloadCount = count
		}
	}

	// Extract language
	for _, lang := range ebook.Language {
		if book.Language == "" {
			// Check for nested Description structure (most common)
			if lang.Description != nil && lang.Description.Value != "" {
				book.Language = strings.TrimSpace(lang.Description.Value)
			} else if lang.Resource != "" {
				// Extract language code from resource URI
				book.Language = lang.Resource
			} else if lang.Value != "" {
				// Fallback to direct value (chardata)
				book.Language = strings.TrimSpace(lang.Value)
			}
		}
	}

	// Extract creators/authors
	for _, creator := range ebook.Creator {
		if creator.Agent != nil {
			fullName := strings.TrimSpace(creator.Agent.Name)
			firstName, lastName := splitName(fullName)

			author := Author{
				Name:      fullName,
				FirstName: firstName,
				LastName:  lastName,
				AgentID:   strings.TrimSpace(creator.Agent.About),
			}

			// Extract aliases (join multiple with semicolon)
			if len(creator.Agent.Alias) > 0 {
				aliases := make([]string, 0, len(creator.Agent.Alias))
				for _, alias := range creator.Agent.Alias {
					if trimmed := strings.TrimSpace(alias); trimmed != "" {
						aliases = append(aliases, trimmed)
					}
				}
				author.Alias = strings.Join(aliases, "; ")
			}

			// Extract webpages (join multiple with semicolon)
			if len(creator.Agent.Webpage) > 0 {
				webpages := make([]string, 0, len(creator.Agent.Webpage))
				for _, webpage := range creator.Agent.Webpage {
					if trimmed := strings.TrimSpace(webpage.Resource); trimmed != "" {
						webpages = append(webpages, trimmed)
					}
				}
				author.Webpage = strings.Join(webpages, "; ")
			}

			if creator.Agent.BirthDate != "" {
				if year := extractYear(creator.Agent.BirthDate); year != nil {
					author.BirthYear = year
				}
			}
			if creator.Agent.DeathDate != "" {
				if year := extractYear(creator.Agent.DeathDate); year != nil {
					author.DeathYear = year
				}
			}
			if author.Name != "" {
				book.Authors = append(book.Authors, author)
			}
		}
	}

	// Extract descriptions (join multiple with newlines)
	if len(ebook.Description) > 0 {
		descriptions := make([]string, 0, len(ebook.Description))
		for _, desc := range ebook.Description {
			if trimmed := strings.TrimSpace(desc); trimmed != "" {
				descriptions = append(descriptions, trimmed)
			}
		}
		book.Description = strings.Join(descriptions, "\n\n")
	}

	// Extract summary (marc520)
	book.Summary = strings.TrimSpace(ebook.MARC520)

	// Extract production notes (marc508)
	book.ProductionNotes = strings.TrimSpace(ebook.MARC508)

	// Extract reading ease score (marc908)
	book.ReadingEaseScore = strings.TrimSpace(ebook.MARC908)

	// Extract subjects
	for _, subject := range ebook.Subject {
		if subject.Description != nil {
			subj := strings.TrimSpace(subject.Description.Value)
			if subj != "" {
				book.Subjects = append(book.Subjects, subj)
			}
		}
	}

	// Extract bookshelves
	for _, bookshelf := range ebook.Bookshelf {
		if bookshelf.Description != nil {
			bs := strings.TrimSpace(bookshelf.Description.Value)
			if bs != "" {
				book.Bookshelves = append(book.Bookshelves, bs)
			}
		}
	}

	// Extract formats
	for _, format := range ebook.Format {
		if format.File != nil {
			f := Format{
				FileURL: format.File.About,
			}

			// Extract file size
			if format.File.Extent != "" {
				if size, err := strconv.ParseInt(strings.TrimSpace(format.File.Extent), 10, 64); err == nil {
					f.FileSize = &size
				}
			}

			// Extract format type
			if format.File.Format != nil {
				f.Type = strings.TrimSpace(format.File.Format.Value)
			}

			if f.Type == "" {
				// Try to extract from URL
				f.Type = extractFormatFromURL(format.File.About)
			}

			if f.FileURL != "" {
				book.Formats = append(book.Formats, f)
			}
		}
	}

	return book, nil
}

// extractGutenbergID extracts the Gutenberg ID from a resource URI
func extractGutenbergID(uri string) string {
	re := regexp.MustCompile(`/(\d+)(?:/|$)`)
	matches := re.FindStringSubmatch(uri)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractYear extracts a year from a date string
func extractYear(dateStr string) *int {
	re := regexp.MustCompile(`(\d{4})`)
	matches := re.FindStringSubmatch(dateStr)
	if len(matches) > 1 {
		if year, err := strconv.Atoi(matches[1]); err == nil {
			return &year
		}
	}
	return nil
}

// findAgentByResource finds an agent description by resource URI
func findAgentByResource(doc *RDFDocument, resource string) *Agent {
	for _, agent := range doc.Agents {
		if agent.About == resource {
			return &agent
		}
	}
	return nil
}

// extractFormatFromURL extracts format type from a URL
func extractFormatFromURL(url string) string {
	url = strings.ToLower(url)
	if strings.Contains(url, "epub") {
		return "application/epub+zip"
	}
	if strings.Contains(url, "kindle") {
		return "application/x-mobipocket-ebook"
	}
	if strings.Contains(url, "html") {
		return "text/html"
	}
	if strings.Contains(url, "txt") || strings.Contains(url, "plain") {
		return "text/plain"
	}
	return ""
}

// splitName splits a full name into first name and last name.
// Handles various formats:
// - "Last, First" (most common in Gutenberg)
// - "First Last"
// - "First Middle Last"
// - Single names (organizations, etc.)
func splitName(fullName string) (firstName, lastName string) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return "", ""
	}

	// Check for "Last, First" format (comma-separated)
	if strings.Contains(fullName, ",") {
		parts := strings.SplitN(fullName, ",", 2)
		if len(parts) == 2 {
			lastName = strings.TrimSpace(parts[0])
			firstName = strings.TrimSpace(parts[1])
			return firstName, lastName
		}
	}

	// Split by spaces for "First Last" or "First Middle Last" format
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", ""
	} else if len(parts) == 1 {
		// Single name - treat as last name (could be organization)
		return "", parts[0]
	} else {
		// Multiple parts: first name is first part, last name is last part
		// Middle names are ignored for simplicity
		firstName = parts[0]
		lastName = parts[len(parts)-1]
		return firstName, lastName
	}
}
