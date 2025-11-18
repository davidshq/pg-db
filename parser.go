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
	About     string      `xml:"about,attr"`
	Title     string      `xml:"title"`
	Creator   []Creator   `xml:"creator"`
	Subject   []Subject   `xml:"subject"`
	Language  []Language  `xml:"language"`
	Rights    string      `xml:"rights"`
	Issued    string      `xml:"issued"`
	Downloads string      `xml:"downloads"`
	Format    []RDFFormat `xml:"hasFormat"`
	Publisher string      `xml:"publisher"`
}

// Creator represents a creator element
type Creator struct {
	Agent *Agent `xml:"agent"`
}

// Agent represents a pgterms:agent element
type Agent struct {
	About     string   `xml:"about,attr"`
	Name      string   `xml:"name"`
	BirthDate string   `xml:"birthdate"`
	DeathDate string   `xml:"deathdate"`
	Alias     []string `xml:"alias"`
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
	Resource string `xml:"resource,attr"`
	Value    string `xml:",chardata"`
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
		Authors:  []Author{},
		Subjects: []string{},
		Formats:  []Format{},
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
			if lang.Resource != "" {
				// Extract language code from resource URI (e.g., "http://purl.org/dc/terms/LCSH" -> "LCSH")
				book.Language = lang.Resource
			} else {
				book.Language = strings.TrimSpace(lang.Value)
			}
		}
	}

	// Extract creators/authors
	for _, creator := range ebook.Creator {
		if creator.Agent != nil {
			author := Author{
				Name: strings.TrimSpace(creator.Agent.Name),
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

	// Extract subjects
	for _, subject := range ebook.Subject {
		if subject.Description != nil {
			subj := strings.TrimSpace(subject.Description.Value)
			if subj != "" {
				book.Subjects = append(book.Subjects, subj)
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
