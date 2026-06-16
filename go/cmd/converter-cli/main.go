package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mkd-epub-exporters/pkg/converter"
)

func main() {
	// Parse CLI arguments
	outputDir := flag.String("out", "", "Output directory for converted files (required)")
	flag.Parse()

	files := flag.Args()

	if len(files) == 0 {
		fmt.Println("Usage: markitdown -out <output_directory> <file1> [file2] ...")
		os.Exit(1)
	}

	if *outputDir == "" {
		fmt.Println("Error: The output directory (-out) is required.")
		os.Exit(1)
	}

	// Try to initialize Pandoc (if installed)
	if err := converter.InitPandoc(); err != nil {
		fmt.Printf("Note: Pandoc not detected (%v). Formats like PPTX, RTF, EPUB, etc. will not be supported.\n", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Expand directory arguments recursively
	allFiles, err := converter.CollectFiles(files)
	if err != nil {
		fmt.Printf("Error reading files: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting conversion of %d file(s)...\n", len(allFiles))
	successCount := 0
	errorCount := 0

	for i, fPath := range allFiles {
		fmt.Printf("[%d/%d] Converting %s...\n", i+1, len(allFiles), filepath.Base(fPath))
		mdContent, err := converter.ConvertFile(fPath, *outputDir, false)
		if err != nil {
			fmt.Printf("  -> Failed: %v\n", err)
			errorCount++
			continue
		}

		// Avoid name collisions in output folder
		outName := strings.TrimSuffix(filepath.Base(fPath), filepath.Ext(fPath)) + ".md"
		outPath := filepath.Join(*outputDir, outName)

		counter := 1
		for {
			if _, err := os.Stat(outPath); os.IsNotExist(err) {
				break
			}
			stem := strings.TrimSuffix(filepath.Base(fPath), filepath.Ext(fPath))
			outPath = filepath.Join(*outputDir, fmt.Sprintf("%s_%d.md", stem, counter))
			counter++
		}

		err = os.WriteFile(outPath, []byte(mdContent), 0644)
		if err != nil {
			fmt.Printf("  -> Failed to write: %v\n", err)
			errorCount++
			continue
		}

		fmt.Printf("  -> Success: %s\n", filepath.Base(outPath))
		successCount++
	}

	fmt.Printf("\nConversion completed.\nSuccess: %d\nFailed/Ignored: %d\n", successCount, errorCount)
}
