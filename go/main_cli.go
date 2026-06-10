//go:build !windows

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Parse CLI arguments
	outputDir := flag.String("out", "", "Dossier de sortie pour les fichiers convertis (obligatoire)")
	flag.Parse()

	files := flag.Args()

	if len(files) == 0 {
		fmt.Println("Usage: markitdown -out <dossier_sortie> <fichier1> [fichier2] ...")
		os.Exit(1)
	}

	if *outputDir == "" {
		fmt.Println("Erreur: Le dossier de sortie (-out) est obligatoire.")
		os.Exit(1)
	}

	// Try to initialize Pandoc (if installed on macOS/Linux)
	if err := initPandoc(); err != nil {
		fmt.Printf("Note: Pandoc non détecté (%v). Les formats PPTX, RTF, EPUB, etc. ne seront pas supportés.\n", err)
	} else {
		fmt.Printf("Pandoc détecté à l'adresse : %s\n", resolvedPandocPath)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("Erreur lors de la création du dossier de sortie: %v\n", err)
		os.Exit(1)
	}

	// Expand directory arguments recursively
	allFiles, err := collectFiles(files)
	if err != nil {
		fmt.Printf("Erreur lors de la lecture des fichiers: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Début de la conversion de %d fichier(s)...\n", len(allFiles))
	successCount := 0
	errorCount := 0

	for i, fPath := range allFiles {
		fmt.Printf("[%d/%d] Conversion de %s...\n", i+1, len(allFiles), filepath.Base(fPath))
		mdContent, err := convertFile(fPath)
		if err != nil {
			fmt.Printf("  -> Échec: %v\n", err)
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
			fmt.Printf("  -> Échec d'écriture: %v\n", err)
			errorCount++
			continue
		}

		fmt.Printf("  -> Succès: %s\n", filepath.Base(outPath))
		successCount++
	}

	fmt.Printf("\nConversion terminée.\nSuccès: %d\nÉchecs/Ignorés: %d\n", successCount, errorCount)
}
