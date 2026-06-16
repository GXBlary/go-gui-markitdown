package converter

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/xuri/excelize/v2"
	"github.com/zakahan/docx2md"
)

var resolvedPandocPath string

// InitPandoc resolves the path of Pandoc (either locally next to the executable, or in the system PATH)
func InitPandoc() error {
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

	return fmt.Errorf("pandoc was not found (neither in the application folder nor in the PATH)")
}

// convertWithPandoc runs the resolved Pandoc tool on the specified file, extracting media if mediaDir is specified.
func convertWithPandoc(filePath string, mediaDir string) (string, error) {
	if resolvedPandocPath == "" {
		return "", fmt.Errorf("pandoc is not available on this system")
	}

	args := []string{filePath, "-t", "markdown"}
	if mediaDir != "" {
		args = append(args, "--extract-media="+mediaDir)
	}

	cmd := exec.Command(resolvedPandocPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("pandoc err: %v (details: %s)", err, stderr.String())
	}
	return stdout.String(), nil
}

// ConvertFile routes the file to the appropriate converter based on extension
func ConvertFile(filePath string, outDir string, embedImages bool) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".docx":
		// Try Pandoc first (higher fidelity for tables/headings), fallback to native docx2md
		tempDir, err := os.MkdirTemp("", "pandoc-docx-")
		if err == nil {
			defer os.RemoveAll(tempDir)
			md, err := convertWithPandoc(filePath, tempDir)
			if err == nil {
				if embedImages {
					md = EmbedLocalImages(md, tempDir)
				} else if outDir != "" {
					CopyLocalImages(md, tempDir, outDir)
				}
				return md, nil
			}
		}
		return convertDocx(filePath, outDir, embedImages)
	case ".xlsx":
		return convertXlsx(filePath)
	case ".pdf":
		return ConvertPdf(filePath, nil)
	case ".html", ".htm":
		// Try Pandoc first, fallback to native html-to-markdown
		tempDir, err := os.MkdirTemp("", "pandoc-html-")
		if err == nil {
			defer os.RemoveAll(tempDir)
			md, err := convertWithPandoc(filePath, tempDir)
			if err == nil {
				if embedImages {
					md = EmbedLocalImages(md, tempDir)
				} else if outDir != "" {
					CopyLocalImages(md, tempDir, outDir)
				}
				return md, nil
			}
		}
		return convertHtml(filePath)
	case ".txt", ".md":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case ".pptx", ".rtf", ".epub", ".odt", ".tex", ".wiki":
		// Try MarkItDown CLI first (high fidelity for PPTX, EPUB, etc.)
		if cliPath, hasCli := GetMarkItDownCliPath(); hasCli {
			md, err := ConvertWithMarkItDown(cliPath, filePath)
			if err == nil {
				return md, nil
			}
		}
		// Fallback to Pandoc
		tempDir, err := os.MkdirTemp("", "pandoc-media-")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(tempDir)

		md, err := convertWithPandoc(filePath, tempDir)
		if err == nil {
			if ext == ".pptx" {
				md = strings.ReplaceAll(md, "\r\n", "\n")
				md = strings.ReplaceAll(md, "\r", "\n")
				re := regexp.MustCompile(`<!--\s*Slide number:\s*\d+\s*-->`)
				md = re.ReplaceAllString(md, "\n\n---\n\n")
				for strings.Contains(md, "\n\n\n") {
					md = strings.ReplaceAll(md, "\n\n\n", "\n\n")
				}
			}
			if embedImages {
				md = EmbedLocalImages(md, tempDir)
			} else if outDir != "" {
				CopyLocalImages(md, tempDir, outDir)
			}
		}
		return md, err
	default:
		return "", fmt.Errorf("unsupported format: %s", ext)
	}
}

// convertDocx extracts text and styles from Word (.docx) to Markdown
func convertDocx(filePath string, outDir string, embedImages bool) (string, error) {
	tempDir, err := os.MkdirTemp("", "docx2md-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	_, mdString, err := docx2md.DocxConvert(filePath, tempDir)
	if err != nil {
		return "", err
	}

	if embedImages {
		mdString = EmbedLocalImages(mdString, tempDir)
	} else if outDir != "" {
		CopyLocalImages(mdString, tempDir, outDir)
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

// CollectFiles returns a flat slice of files by expanding directories recursively
func CollectFiles(paths []string) ([]string, error) {
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

// GetMarkItDownCliPath resolves the path of markitdown-cli.exe
func GetMarkItDownCliPath() (string, bool) {
	exePath, err := os.Executable()
	if err != nil {
		return "", false
	}
	dir := filepath.Dir(exePath)

	paths := []string{
		filepath.Join(dir, "markitdown-cli.exe"),
		filepath.Join(dir, "resources", "markitdown-cli.exe"),
		filepath.Join(dir, "..", "resources", "markitdown-cli.exe"),
		filepath.Join(dir, "..", "mkd-epub-virtual-printers", "resources", "markitdown-cli.exe"),
	}

	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, true
		}
	}

	return "", false
}

// ConvertWithMarkItDown runs the resolved markitdown-cli.exe on the specified file
func ConvertWithMarkItDown(cliPath, filePath string) (string, error) {
	cmd := exec.Command(cliPath, filePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("markitdown-cli err: %v (details: %s)", err, stderr.String())
	}
	text := stdout.String()

	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Replace <!-- Slide number: X --> with \n\n---\n\n
	re := regexp.MustCompile(`<!--\s*Slide number:\s*\d+\s*-->`)
	text = re.ReplaceAllString(text, "\n\n---\n\n")

	// Clean up consecutive newlines (3 or more) to max 2 newlines
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return text, nil
}

// EmbedLocalImages parses the markdown content for local image references, reads the image files from baseDir, Base64-encodes them, and embeds them inline.
func EmbedLocalImages(markdown string, baseDir string) string {
	re := regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)
	return re.ReplaceAllStringFunc(markdown, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		alt := submatches[1]
		src := submatches[2]

		// Skip remote URLs or data URIs
		if regexp.MustCompile(`^(http|https|ftp|data):`).MatchString(src) {
			return match
		}

		imgPath := filepath.Join(baseDir, src)
		if info, err := os.Stat(imgPath); err == nil && !info.IsDir() {
			data, err := os.ReadFile(imgPath)
			if err != nil {
				return match
			}

			mimeType := mime.TypeByExtension(filepath.Ext(imgPath))
			if mimeType == "" {
				mimeType = http.DetectContentType(data)
			}

			b64 := base64.StdEncoding.EncodeToString(data)
			return fmt.Sprintf("![%s](data:%s;base64,%s)", alt, mimeType, b64)
		}

		return match
	})
}

// CopyLocalImages parses the markdown content for local image references, and copies those files from baseDir to outDir.
func CopyLocalImages(markdown string, baseDir string, outDir string) {
	re := regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)
	submatchesList := re.FindAllStringSubmatch(markdown, -1)
	for _, submatches := range submatchesList {
		if len(submatches) < 3 {
			continue
		}
		src := submatches[2]

		// Skip remote URLs or data URIs
		if regexp.MustCompile(`^(http|https|ftp|data):`).MatchString(src) {
			continue
		}

		srcPath := filepath.Join(baseDir, src)
		if info, err := os.Stat(srcPath); err == nil && !info.IsDir() {
			destPath := filepath.Join(outDir, src)
			os.MkdirAll(filepath.Dir(destPath), 0755)

			data, err := os.ReadFile(srcPath)
			if err == nil {
				os.WriteFile(destPath, data, 0644)
			}
		}
	}
}
