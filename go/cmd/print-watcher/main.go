package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/registry"
)

const (
	registryName = "MkdEpubPrintWatcher"
	singletonPort = "127.0.0.1:18293"
)

func checkSingleton() (net.Listener, error) {
	ln, err := net.Listen("tcp", singletonPort)
	if err != nil {
		return nil, fmt.Errorf("another instance is already running: %w", err)
	}
	return ln, nil
}

func registerStartup() {
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("Startup registration: failed to get executable path: %v", err)
		return
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		log.Printf("Startup registration: failed to open registry key: %v", err)
		return
	}
	defer key.Close()

	err = key.SetStringValue(registryName, fmt.Sprintf(`"%s"`, exePath))
	if err != nil {
		log.Printf("Startup registration: failed to set registry value: %v", err)
	} else {
		log.Println("Startup registration: successfully registered in HKCU Startup.")
	}
}

func initLogging() (*os.File, error) {
	tempDir := os.Getenv("TEMP")
	if tempDir == "" {
		tempDir = `C:\Windows\Temp`
	}
	logDir := filepath.Join(tempDir, "mkd-epub-print-watcher")
	_ = os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "watcher.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	log.Println("--- WATCHER STARTED ---")
	return f, nil
}

func generateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := 0; i < n; i++ {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

func tryProcessSpool(dir, exeName string) {
	spoolFile := filepath.Join(dir, "spool.pdf")
	
	// 1. Check if spool.pdf exists
	info, err := os.Stat(spoolFile)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		return
	}

	// 2. Wait until the file has content (size > 0)
	if info.Size() == 0 {
		return
	}

	// 3. Try to open the file with write access to check if it's unlocked by the spooler
	f, err := os.OpenFile(spoolFile, os.O_WRONLY, 0)
	if err != nil {
		// File is still locked by the spooler (printing in progress)
		return
	}
	f.Close() // File is unlocked!

	log.Printf("Detected completed print job in %s", dir)

	// 4. Generate a unique name and move/rename the spool file
	timestamp := time.Now().Format("20060102_150405")
	randStr := generateRandomString(6)
	jobFilename := fmt.Sprintf("job_%s_%s.pdf", timestamp, randStr)
	jobPath := filepath.Join(dir, jobFilename)

	err = os.Rename(spoolFile, jobPath)
	if err != nil {
		log.Printf("Error renaming spool file: %v", err)
		return
	}
	log.Printf("Renamed spool.pdf to %s", jobFilename)

	// 5. Run the printer GUI
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("Error getting watcher executable path: %v", err)
		return
	}
	binDir := filepath.Dir(exePath)
	printerExe := filepath.Join(binDir, exeName)

	log.Printf("Launching printer executable: %s %s", printerExe, jobPath)
	cmd := exec.Command(printerExe, jobPath)
	// Start asynchronously so the watcher doesn't block the next jobs
	err = cmd.Start()
	if err != nil {
		log.Printf("Error starting printer executable %s: %v", printerExe, err)
	}
}

func main() {
	// 1. Check if another instance is running
	listener, err := checkSingleton()
	if err != nil {
		// Silently exit if already running
		os.Exit(0)
	}
	defer func() {
		if listener != nil {
			listener.Close()
		}
	}()

	// 2. Init logging
	logFile, err := initLogging()
	if err == nil && logFile != nil {
		defer logFile.Close()
	}

	// 3. Register in HKCU run key for startup
	registerStartup()

	// 4. Ensure directories exist
	mkdSpoolDir := `C:\Windows\Temp\markitdown-spool`
	epubSpoolDir := `C:\Windows\Temp\epub-spool`
	
	_ = os.MkdirAll(mkdSpoolDir, 0777)
	_ = os.MkdirAll(epubSpoolDir, 0777)

	log.Printf("Watching directories:\n- %s\n- %s", mkdSpoolDir, epubSpoolDir)

	// 5. Watch loop
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		tryProcessSpool(mkdSpoolDir, "markitdown-printer.exe")
		tryProcessSpool(epubSpoolDir, "epub-printer.exe")
	}
}
