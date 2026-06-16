package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	MB_OK                = 0x00000000
	MB_OKCANCEL          = 0x00000001
	MB_ICONHAND          = 0x00000010 // Error icon
	MB_ICONINFORMATION   = 0x00000040 // Info icon
	IDOK                 = 1
)

func MessageBox(title, text string, style uintptr) int {
	ret, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))),
		style,
	)
	return int(ret)
}

func main() {
	title := "Print to EPUB Installer"

	// 1. Confirmation Dialog
	promptText := "This installer will configure the virtual printer 'Print to EPUB' on your computer.\n\nDo you want to proceed?"
	if MessageBox(title, promptText, MB_OKCANCEL|MB_ICONINFORMATION) != IDOK {
		os.Exit(0)
	}

	exePath, err := os.Executable()
	if err != nil {
		MessageBox(title, fmt.Sprintf("Failed to get installer executable path: %v", err), MB_OK|MB_ICONHAND)
		os.Exit(1)
	}
	dir := filepath.Dir(exePath)
	scriptPath := filepath.Join(dir, "install_printer.ps1")

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		MessageBox(title, fmt.Sprintf("Configuration script not found at:\n%s\n\nPlease make sure the installation files are fully extracted.", scriptPath), MB_OK|MB_ICONHAND)
		os.Exit(1)
	}

	// 2. Unblock the PowerShell script in case it has web zone markers
	unblockCmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", fmt.Sprintf("Unblock-File -Path '%s' -ErrorAction SilentlyContinue", scriptPath))
	_ = unblockCmd.Run()

	// 3. Execute the PowerShell script in the background and capture output
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		errorMsg := fmt.Sprintf("Installation failed.\n\nError: %v\n\nDetails:\n%s", err, string(output))
		MessageBox(title, errorMsg, MB_OK|MB_ICONHAND)
		os.Exit(1)
	}

	// 4. Success Dialog
	successText := "Installation completed successfully!\n\nThe virtual printer 'Print to EPUB' is now ready for use."
	MessageBox(title, successText, MB_OK|MB_ICONINFORMATION)
}
