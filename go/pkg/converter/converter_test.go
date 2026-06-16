package converter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbedLocalImages(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-embed-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a dummy image file
	dummyImgContent := []byte("fake image data")
	imgPath := filepath.Join(tempDir, "image.png")
	if err := os.WriteFile(imgPath, dummyImgContent, 0644); err != nil {
		t.Fatalf("failed to write dummy image: %v", err)
	}

	markdown := "Here is an image: ![alt text](image.png)"
	result := EmbedLocalImages(markdown, tempDir)

	if !strings.Contains(result, "data:image/png;base64,") {
		t.Errorf("expected base64 URI, got: %s", result)
	}
}

func TestCopyLocalImages(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-copy-src-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	destDir, err := os.MkdirTemp("", "test-copy-dest-")
	if err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Create a dummy image file in src
	dummyImgContent := []byte("fake image data")
	imgPath := filepath.Join(tempDir, "image.png")
	if err := os.WriteFile(imgPath, dummyImgContent, 0644); err != nil {
		t.Fatalf("failed to write dummy image: %v", err)
	}

	markdown := "Here is an image: ![alt text](image.png)"
	CopyLocalImages(markdown, tempDir, destDir)

	copiedImgPath := filepath.Join(destDir, "image.png")
	if _, err := os.Stat(copiedImgPath); os.IsNotExist(err) {
		t.Errorf("expected image to be copied to dest, but it doesn't exist")
	}
}

func TestConvertToEpub(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-epub-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	epubPath := filepath.Join(tempDir, "test.epub")
	markdown := "# Chapter 1\nThis is some text.\n---\n# Chapter 2\nMore text."

	err = ConvertToEpub(markdown, "Test Book", epubPath)
	if err != nil {
		t.Fatalf("failed to convert to epub: %v", err)
	}

	// Verify file size is non-zero
	info, err := os.Stat(epubPath)
	if err != nil {
		t.Fatalf("failed to stat epub file: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("generated epub file is empty")
	}
}
