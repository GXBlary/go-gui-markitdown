package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
	"github.com/zakahan/docx2md"
)

var resolvedPandocPath string

// initPandoc resolves the path of Pandoc (either locally next to the executable, or in the system PATH)
func initPandoc() error {
	// 1. Look in the same directory as the current running executable
	exePath, err := os.Executable()
	if err == nil {
		localDir := filepath.Dir(exePath)
		// Try both windows name and unix name
		names := []string{"pandoc.exe", "pandoc"}
		for _, name := range names {
			localPath := filepath.Join(localDir, name)
			if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
				resolvedPandocPath = localPath
				return nil
			}
		}
	}

	// 2. Look in the system PATH
	systemPath, err := exec.LookPath("pandoc")
	if err == nil {
		resolvedPandocPath = systemPath
		return nil
	}

	return fmt.Errorf("pandoc n'a pas été trouvé (ni dans le dossier de l'application, ni dans le PATH)")
}

// convertWithPandoc runs the resolved Pandoc tool on the specified file
func convertWithPandoc(filePath string) (string, error) {
	if resolvedPandocPath == "" {
		return "", fmt.Errorf("pandoc n'est pas disponible sur ce système")
	}

	// Run: pandoc -t markdown <filePath>
	cmd := exec.Command(resolvedPandocPath, filePath, "-t", "markdown")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("pandoc err: %v (details: %s)", err, stderr.String())
	}
	return stdout.String(), nil
}

// convertFile routes the file to the appropriate converter based on extension
func convertFile(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".docx":
		// Try Pandoc first (higher fidelity for tables/headings), fallback to native docx2md
		if md, err := convertWithPandoc(filePath); err == nil {
			return md, nil
		}
		return convertDocx(filePath)
	case ".xlsx":
		return convertXlsx(filePath)
	case ".pdf":
		return convertPdf(filePath)
	case ".html", ".htm":
		// Try Pandoc first, fallback to native html-to-markdown
		if md, err := convertWithPandoc(filePath); err == nil {
			return md, nil
		}
		return convertHtml(filePath)
	case ".txt", ".md":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case ".pptx", ".rtf", ".epub", ".odt", ".tex", ".wiki":
		// Formats only supported via Pandoc
		return convertWithPandoc(filePath)
	default:
		return "", fmt.Errorf("format non supporté: %s", ext)
	}
}

// convertPdf extracts text from a PDF file
func convertPdf(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	_, err = buf.ReadFrom(b)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// convertDocx extracts text and styles from Word (.docx) to Markdown
func convertDocx(filePath string) (string, error) {
	tempDir, err := os.MkdirTemp("", "docx2md-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	_, mdString, err := docx2md.DocxConvert(filePath, tempDir)
	if err != nil {
		return "", err
	}

	return mdString, nil
}

// convertXlsx reads Excel cells and formats them into Markdown tables
func convertXlsx(filePath string) (string, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	sheets := f.GetSheetList()
	for _, sheetName := range sheets {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			continue
		}
		if len(rows) == 0 {
			continue
		}

		buf.WriteString(fmt.Sprintf("## %s\n\n", sheetName))

		// Find maximum columns to ensure grid alignment
		maxCols := 0
		for _, row := range rows {
			if len(row) > maxCols {
				maxCols = len(row)
			}
		}

		if maxCols == 0 {
			continue
		}

		// Write header row
		buf.WriteString("|")
		for c := 0; c < maxCols; c++ {
			val := ""
			if c < len(rows[0]) {
				val = strings.TrimSpace(rows[0][c])
			}
			buf.WriteString(fmt.Sprintf(" %s |", escapeMarkdownTable(val)))
		}
		buf.WriteString("\n")

		// Write separator row
		buf.WriteString("|")
		for c := 0; c < maxCols; c++ {
			buf.WriteString(" --- |")
		}
		buf.WriteString("\n")

		// Write data rows
		for r := 1; r < len(rows); r++ {
			buf.WriteString("|")
			for c := 0; c < maxCols; c++ {
				val := ""
				if c < len(rows[r]) {
					val = strings.TrimSpace(rows[r][c])
				}
				buf.WriteString(fmt.Sprintf(" %s |", escapeMarkdownTable(val)))
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	return buf.String(), nil
}

// convertHtml converts HTML to Markdown
func convertHtml(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	markdown, err := htmltomarkdown.ConvertString(string(data))
	if err != nil {
		return "", err
	}

	return markdown, nil
}

// escapeMarkdownTable escapes the '|' character and removes line breaks to preserve table layout
func escapeMarkdownTable(val string) string {
	val = strings.ReplaceAll(val, "|", "\\|")
	val = strings.ReplaceAll(val, "\n", " ")
	val = strings.ReplaceAll(val, "\r", "")
	return val
}

// collectFiles returns a flat slice of files by expanding directories recursively
func collectFiles(paths []string) ([]string, error) {
	var allFiles []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			err := filepath.Walk(p, func(path string, fileInfo os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !fileInfo.IsDir() {
					allFiles = append(allFiles, path)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			allFiles = append(allFiles, p)
		}
	}
	return allFiles, nil
}
