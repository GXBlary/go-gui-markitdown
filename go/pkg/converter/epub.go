package converter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-shiori/go-epub"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

// splitMarkdownIntoPages splits markdown content by standalone Markdown horizontal rules/page breaks.
func splitMarkdownIntoPages(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	lines := strings.Split(normalized, "\n")
	var pages []string
	var currentChunk []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if the line is exactly "---", "***", or "___" (Markdown horizontal rules/page breaks)
		isSeparator := false
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			isSeparator = true
		}

		if isSeparator {
			pageContent := strings.Join(currentChunk, "\n")
			if strings.TrimSpace(pageContent) != "" {
				pages = append(pages, pageContent)
			}
			currentChunk = nil
		} else {
			currentChunk = append(currentChunk, line)
		}
	}

	pageContent := strings.Join(currentChunk, "\n")
	if strings.TrimSpace(pageContent) != "" {
		pages = append(pages, pageContent)
	}

	return pages
}

// extractFirstHeading finds the first markdown heading (e.g. # Heading) in the text to use as section title.
func extractFirstHeading(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// Count the number of leading '#'
			count := 0
			for _, r := range trimmed {
				if r == '#' {
					count++
				} else {
					break
				}
			}
			// Must have at least one '#' and be followed by a space
			if count > 0 && len(trimmed) > count && trimmed[count] == ' ' {
				heading := strings.TrimSpace(trimmed[count:])
				if heading != "" {
					return heading
				}
			}
		}
	}
	return ""
}

// cleanHeading removes common Markdown formatting characters from a heading for TOC presentation.
func cleanHeading(heading string) string {
	heading = strings.ReplaceAll(heading, "**", "")
	heading = strings.ReplaceAll(heading, "*", "")
	heading = strings.ReplaceAll(heading, "`", "")
	heading = strings.ReplaceAll(heading, "_", "")
	return strings.TrimSpace(heading)
}

// ConvertToEpub compiles the extracted text (Markdown/plain text) into a valid EPUB document
func ConvertToEpub(text string, title string, outputPath string) error {
	// Create a new EPUB
	e, err := epub.NewEpub(title)
	if err != nil {
		return fmt.Errorf("failed to create epub instance: %w", err)
	}
	e.SetAuthor("MarkItDown Converter")

	// Split the text into pages on horizontal rules (---)
	pages := splitMarkdownIntoPages(text)
	if len(pages) == 0 {
		pages = []string{text}
	}

	// Convert Markdown/plain text to HTML with hard line wraps and XHTML formatting enabled
	md := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)

	for i, pageText := range pages {
		var htmlBuf bytes.Buffer
		if err := md.Convert([]byte(pageText), &htmlBuf); err != nil {
			return fmt.Errorf("failed to convert page %d to html: %w", i+1, err)
		}

		// Choose a title for this section based on the first heading, or fallback to Page X
		sectionTitle := extractFirstHeading(pageText)
		if sectionTitle != "" {
			sectionTitle = cleanHeading(sectionTitle)
		} else {
			sectionTitle = fmt.Sprintf("Page %d", i+1)
		}

		// Add section to EPUB with automatic filename generation
		_, err = e.AddSection(htmlBuf.String(), sectionTitle, "", "")
		if err != nil {
			return fmt.Errorf("failed to add page %d to epub: %w", i+1, err)
		}
	}

	// Write to file
	if err := e.Write(outputPath); err != nil {
		return fmt.Errorf("failed to write epub file: %w", err)
	}

	return nil
}
