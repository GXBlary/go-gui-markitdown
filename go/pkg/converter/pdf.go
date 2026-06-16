package converter

import (
	"math"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

var win1252Map = [32]rune{
	0x20AC, 0xFFFD, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, // 80-87
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0xFFFD, 0x017D, 0xFFFD, // 88-8F
	0xFFFD, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, // 90-97
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0xFFFD, 0x017E, 0x0178, // 98-9F
}

type Row struct {
	Y        float64
	Elements []pdf.Text
}

type StyledRun struct {
	Text   string
	Bold   bool
	Italic bool
}

// ConvertPdf extracts text from a PDF file preserving layout and typography.
func ConvertPdf(filePath string, onProgress func(currentPage, totalPages int)) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var docBuilder strings.Builder
	numPages := r.NumPage()

	if onProgress != nil {
		onProgress(0, numPages)
	}

	for pageNum := 1; pageNum <= numPages; pageNum++ {
		p := r.Page(pageNum)
		content := p.Content()
		texts := content.Text

		if len(texts) == 0 {
			continue
		}

		// 1. Group text elements into rows by Y coordinate
		var rows []Row
		yThreshold := 5.0

		for _, t := range texts {
			found := false
			for i, row := range rows {
				if math.Abs(row.Y-t.Y) < yThreshold {
					rows[i].Elements = append(rows[i].Elements, t)
					found = true
					break
				}
			}
			if !found {
				rows = append(rows, Row{
					Y:        t.Y,
					Elements: []pdf.Text{t},
				})
			}
		}

		// Sort rows by Y descending (top to bottom)
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Y > rows[j].Y
		})

		// 2. Find the body font size (statistical mode rounded to 0.5pt)
		sizeCounts := make(map[float64]int)
		for _, t := range texts {
			roundedSize := math.Round(t.FontSize*2) / 2
			sizeCounts[roundedSize]++
		}
		bodySize := 10.0
		maxCount := 0
		for sz, count := range sizeCounts {
			if count > maxCount {
				maxCount = count
				bodySize = sz
			}
		}

		// 3. Process each row
		for _, row := range rows {
			// Sort elements horizontally (left to right)
			sort.SliceStable(row.Elements, func(i, j int) bool {
				return row.Elements[i].X < row.Elements[j].X
			})

			// Determine dominant font size for this row
			rowSizeCounts := make(map[float64]int)
			for _, t := range row.Elements {
				rounded := math.Round(t.FontSize*2) / 2
				rowSizeCounts[rounded]++
			}
			domSize := bodySize
			maxRowSizeCount := 0
			for sz, count := range rowSizeCounts {
				if count > maxRowSizeCount {
					maxRowSizeCount = count
					domSize = sz
				}
			}

			// Group elements into styled runs
			var runs []StyledRun
			var currentRun strings.Builder
			var currentBold, currentItalic bool
			hasStarted := false

			for i, t := range row.Elements {
				fontLower := strings.ToLower(t.Font)
				isBold := strings.Contains(fontLower, "bold")
				isItalic := strings.Contains(fontLower, "italic") || strings.Contains(fontLower, "oblique")

				// Gap detection for space insertion
				if i > 0 && t.S != " " {
					prev := row.Elements[i-1]
					if t.X > prev.X+0.1 && prev.S != " " {
						gap := t.X - (prev.X + prev.W)
						spaceWidth := t.FontSize * 0.2
						if gap > spaceWidth {
							if currentRun.Len() > 0 {
								runs = append(runs, StyledRun{
									Text:   currentRun.String(),
									Bold:   currentBold,
									Italic: currentItalic,
								})
								currentRun.Reset()
							}
							runs = append(runs, StyledRun{Text: " "})
						}
					}
				}

				if !hasStarted {
					currentBold = isBold
					currentItalic = isItalic
					hasStarted = true
				} else if isBold != currentBold || isItalic != currentItalic {
					if currentRun.Len() > 0 {
						runs = append(runs, StyledRun{
							Text:   currentRun.String(),
							Bold:   currentBold,
							Italic: currentItalic,
						})
						currentRun.Reset()
					}
					currentBold = isBold
					currentItalic = isItalic
				}

				currentRun.WriteString(t.S)
			}
			if currentRun.Len() > 0 {
				runs = append(runs, StyledRun{
					Text:   currentRun.String(),
					Bold:   currentBold,
					Italic: currentItalic,
				})
			}

			// Format text according to styles
			var rowBuilder strings.Builder
			for _, run := range runs {
				text := run.Text
				if text == " " {
					rowBuilder.WriteString(" ")
					continue
				}

				if run.Bold && run.Italic {
					rowBuilder.WriteString("***" + text + "***")
				} else if run.Bold {
					rowBuilder.WriteString("**" + text + "**")
				} else if run.Italic {
					rowBuilder.WriteString("*" + text + "*")
				} else {
					rowBuilder.WriteString(text)
				}
			}

			line := strings.TrimSpace(rowBuilder.String())
			if len(line) == 0 {
				continue
			}

			// Auto-header markdown formatting
			isAlreadyFormatted := strings.HasPrefix(line, "#") ||
				strings.HasPrefix(line, "-") ||
				strings.HasPrefix(line, "*") ||
				strings.HasPrefix(line, "|") ||
				strings.HasPrefix(line, ">")

			if !isAlreadyFormatted && domSize >= bodySize+2.0 {
				diff := domSize - bodySize
				if diff >= 8.0 {
					line = "# " + line
				} else if diff >= 4.0 {
					line = "## " + line
				} else {
					line = "### " + line
				}
			}

			docBuilder.WriteString(line + "\n")
		}

		// Page separator
		if pageNum < numPages {
			docBuilder.WriteString("\n---\n\n")
		}

		if onProgress != nil {
			onProgress(pageNum, numPages)
		}
	}

	return docBuilder.String(), nil
}

func DecodeText(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	// Check for UTF-16LE BOM (FF FE)
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		return decodeUTF16(b[2:], false)
	}

	// Check for UTF-16BE BOM (FE FF)
	if len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {
		return decodeUTF16(b[2:], true)
	}

	// Check for UTF-8 BOM (EF BB BF)
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		b = b[3:]
	}

	// Check if it's valid UTF-8
	if utf8.Valid(b) {
		return string(b)
	}

	// Fallback to Windows-1252 (ANSI)
	return decodeWindows1252(b)
}

func decodeUTF16(b []byte, bigEndian bool) string {
	n := len(b) / 2
	u16s := make([]uint16, n)
	for i := 0; i < n; i++ {
		if bigEndian {
			u16s[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		} else {
			u16s[i] = uint16(b[2*i+1])<<8 | uint16(b[2*i])
		}
	}
	return string(utf16.Decode(u16s))
}

func decodeWindows1252(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		if c >= 0x80 && c <= 0x9F {
			runes[i] = win1252Map[c-0x80]
		} else {
			runes[i] = rune(c)
		}
	}
	return string(runes)
}
