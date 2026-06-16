package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"mkd-epub-exporters/pkg/converter"
)

var (
	wtsapi32          = syscall.NewLazyDLL("wtsapi32.dll")
	wtsQueryUserToken = wtsapi32.NewProc("WTSQueryUserToken")

	advapi32            = syscall.NewLazyDLL("advapi32.dll")
	createProcessAsUser = advapi32.NewProc("CreateProcessAsUserW")

	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	processIdToSessionId         = kernel32.NewProc("ProcessIdToSessionId")
	wtsGetActiveConsoleSessionId = kernel32.NewProc("WTSGetActiveConsoleSessionId")

	userenv                 = syscall.NewLazyDLL("userenv.dll")
	createEnvironmentBlock  = userenv.NewProc("CreateEnvironmentBlock")
	destroyEnvironmentBlock = userenv.NewProc("DestroyEnvironmentBlock")

	user32                   = syscall.NewLazyDLL("user32.dll")
	allowSetForegroundWindow = user32.NewProc("AllowSetForegroundWindow")
)

func getSessionID() (uint32, error) {
	var sessionId uint32
	pid := os.Getpid()
	r1, _, err := processIdToSessionId.Call(uintptr(pid), uintptr(unsafe.Pointer(&sessionId)))
	if r1 == 0 {
		return 0, err
	}
	return sessionId, nil
}

func runInUserSession() (bool, error) {
	log.Println("runInUserSession: Start")
	mySessionId, err := getSessionID()
	if err != nil {
		return false, fmt.Errorf("failed to get current session ID: %w", err)
	}
	log.Printf("runInUserSession: Current Session ID = %d", mySessionId)

	// If we are not in Session 0, we don't need to relaunch
	if mySessionId != 0 {
		return false, nil
	}

	// 1. Get active console session ID (user's desktop session)
	activeSessionId, _, _ := wtsGetActiveConsoleSessionId.Call()
	log.Printf("runInUserSession: Active Console Session ID = %d", activeSessionId)
	if activeSessionId == 0 {
		return false, fmt.Errorf("no active console session found")
	}

	// 2. Query user token for the active session
	var userToken syscall.Handle
	r1, _, err := wtsQueryUserToken.Call(activeSessionId, uintptr(unsafe.Pointer(&userToken)))
	log.Printf("runInUserSession: WTSQueryUserToken result = %d, err = %v", r1, err)
	if r1 == 0 {
		return false, fmt.Errorf("WTSQueryUserToken failed: %w", err)
	}
	defer syscall.CloseHandle(userToken)

	// 3. Prepare CreateProcessAsUser arguments
	exePath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("failed to get executable path: %w", err)
	}
	log.Printf("runInUserSession: Executable path = %s", exePath)

	// Build the command line with all original arguments
	var cmdStr string
	if len(os.Args) >= 2 {
		var args []string
		for _, arg := range os.Args[1:] {
			escaped := strings.ReplaceAll(arg, `"`, `\"`)
			args = append(args, fmt.Sprintf(`"%s"`, escaped))
		}
		cmdStr = fmt.Sprintf(`"%s" %s`, exePath, strings.Join(args, " "))
	} else {
		cmdStr = fmt.Sprintf(`"%s"`, exePath)
	}
	log.Printf("runInUserSession: Command line string = %s", cmdStr)

	cmdLineUTF16, err := syscall.UTF16FromString(cmdStr)
	if err != nil {
		return false, fmt.Errorf("failed to convert command line to UTF16: %w", err)
	}
	cmdLinePtr := &cmdLineUTF16[0]

	desktopUTF16, err := syscall.UTF16FromString("winsta0\\default")
	if err != nil {
		return false, fmt.Errorf("failed to convert desktop to UTF16: %w", err)
	}

	var envBlock uintptr
	r1, _, err = createEnvironmentBlock.Call(
		uintptr(unsafe.Pointer(&envBlock)),
		uintptr(userToken),
		1, // bInherit = TRUE
	)
	log.Printf("runInUserSession: CreateEnvironmentBlock result = %d, err = %v", r1, err)
	if r1 == 0 {
		return false, fmt.Errorf("CreateEnvironmentBlock failed: %w", err)
	}
	defer destroyEnvironmentBlock.Call(envBlock)

	var si syscall.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Desktop = &desktopUTF16[0]

	var pi syscall.ProcessInformation

	log.Println("runInUserSession: Calling CreateProcessAsUserW...")
	r1, _, err = createProcessAsUser.Call(
		uintptr(userToken),
		0,
		uintptr(unsafe.Pointer(cmdLinePtr)),
		0,
		0,
		1, // inherit handles = TRUE
		0x00000400, // creation flags = CREATE_UNICODE_ENVIRONMENT
		envBlock,
		0, // current directory = NULL
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	log.Printf("runInUserSession: CreateProcessAsUserW result = %d, err = %v", r1, err)
	if r1 == 0 {
		return false, fmt.Errorf("CreateProcessAsUserW failed: %w", err)
	}

	// Allow the child process to set the foreground window
	allowSetForegroundWindow.Call(uintptr(pi.ProcessId))

	// Close spawned process handles
	syscall.CloseHandle(pi.Process)
	syscall.CloseHandle(pi.Thread)

	log.Println("runInUserSession: Finished successfully")
	return true, nil
}

func initLogging() (*os.File, error) {
	tempDir := os.Getenv("TEMP")
	if tempDir == "" {
		tempDir = `C:\Windows\Temp` // Fallback for Session 0 if TEMP is not set
	}
	logDir := filepath.Join(tempDir, "markitdown-spool")
	_ = os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "debug.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	log.SetOutput(f)
	log.Println("--- NEW INVOCATION ---")
	return f, nil
}

func sanitizeFilename(name string) string {
	badChars := `\/:*?"<>|`
	for _, c := range badChars {
		name = strings.ReplaceAll(name, string(c), "-")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "document"
	}
	return name
}

func convertFileWithProgress(filePath string, onProgress func(currentPage, totalPages int)) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".pdf" {
		return converter.ConvertPdf(filePath, onProgress)
	}
	if onProgress != nil {
		onProgress(0, 1)
	}
	res, err := converter.ConvertFile(filePath, "", false)
	if onProgress != nil {
		onProgress(1, 1)
	}
	return res, err
}

func convertWithProgress(pdfPath string) (string, error) {
	var dlg *walk.Dialog
	var pb *walk.ProgressBar
	var lbl *walk.Label
	var resultText string
	var errResult error

	err := Dialog{
		AssignTo: &dlg,
		Title:    "Conversion en cours",
		MinSize:  Size{Width: 350, Height: 110},
		Layout:   VBox{Margins: Margins{Left: 15, Top: 15, Right: 15, Bottom: 15}},
		Children: []Widget{
			Label{
				AssignTo: &lbl,
				Text:     "Initialisation de la conversion...",
			},
			ProgressBar{
				AssignTo: &pb,
			},
		},
	}.Create(nil)

	if err != nil {
		return "", err
	}

	dlg.Starting().Attach(func() {
		go func() {
			resultText, errResult = convertFileWithProgress(pdfPath, func(curr, total int) {
				dlg.Synchronize(func() {
					if dlg.IsDisposed() {
						return
					}
					if total > 0 {
						pb.SetRange(0, total)
						pb.SetValue(curr)
						lbl.SetText(fmt.Sprintf("Extraction de la page %d / %d...", curr, total))
					} else {
						lbl.SetText("Conversion du document en cours...")
					}
				})
			})

			dlg.Synchronize(func() {
				if dlg.IsDisposed() {
					return
				}
				dlg.Close(walk.DlgCmdOK)
			})
		}()
	})

	dlg.Run()

	return resultText, errResult
}

func main() {
	// Lock the OS thread for GUI operations
	runtime.LockOSThread()

	// Initialize logging
	logFile, err := initLogging()
	if err == nil && logFile != nil {
		defer logFile.Close()
	}

	log.Printf("Args: %v", os.Args)
	sessionId, err := getSessionID()
	log.Printf("Session ID: %d, err: %v", sessionId, err)

	// If running in Session 0 (Print Spooler context), relaunch in the active user's interactive session
	relaunched, err := runInUserSession()
	if err != nil {
		log.Printf("Session relaunch error: %v", err)
	}
	if relaunched {
		log.Println("Relaunched in user session successfully. Exiting parent process.")
		os.Exit(0)
	}

	// 1. Check command line arguments
	var pdfPath string
	var shouldCleanupFile bool

	log.Println("main: Checking command line arguments...")
	if len(os.Args) < 2 {
		log.Println("main: No arguments, showing interactive prompt.")
		msgText := "Voulez-vous convertir un document existant (PDF, PowerPoint, Word, Excel, HTML, Texte) en Markdown ?\n\n" +
			"- Cliquez sur 'Oui' pour sélectionner un document à convertir.\n" +
			"- Cliquez sur 'Non' pour installer/enregistrer l'imprimante virtuelle.\n" +
			"- Cliquez sur 'Annuler' pour quitter."
		ret := walk.MsgBox(nil, "Print to Markdown", msgText, walk.MsgBoxYesNoCancel|walk.MsgBoxIconQuestion)
		if ret == walk.DlgCmdYes {
			dlg := &walk.FileDialog{
				Title:  "Sélectionnez le document à convertir en Markdown",
				Filter: "Tous les fichiers supportés (*.pdf;*.docx;*.pptx;*.xlsx;*.html;*.htm;*.txt)|*.pdf;*.docx;*.pptx;*.xlsx;*.html;*.htm;*.txt|Tous les fichiers (*.*)|*.*",
			}
			ok, err := dlg.ShowOpen(nil)
			if err != nil || !ok {
				os.Exit(0)
			}
			pdfPath = dlg.FilePath
			shouldCleanupFile = false
		} else if ret == walk.DlgCmdNo {
			exePath, err := os.Executable()
			if err == nil {
				installerPath := filepath.Join(filepath.Dir(exePath), "install.exe")
				if _, err := os.Stat(installerPath); err == nil {
					cmd := exec.Command(installerPath)
					cmd.Dir = filepath.Dir(exePath)
					_ = cmd.Start()
				} else {
					walk.MsgBox(nil, "Installateur introuvable",
						fmt.Sprintf("Impossible de trouver l'installateur à l'adresse :\n%s\n\nVeuillez exécuter l'installateur manuellement.", installerPath),
						walk.MsgBoxIconError)
				}
			}
			os.Exit(0)
		} else {
			os.Exit(0)
		}
	} else {
		pdfPath = os.Args[1]
		// Only clean up the file if it resides in the temporary spool directories
		shouldCleanupFile = strings.Contains(strings.ToLower(pdfPath), "spool") || strings.Contains(strings.ToLower(pdfPath), "temp")
	}

	log.Printf("main: Input document path = %s, shouldCleanup = %v", pdfPath, shouldCleanupFile)

	// 2. Verify file exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		log.Printf("main: Error - Input file does not exist: %v", err)
		walk.MsgBox(nil, "Error",
			fmt.Sprintf("The input file does not exist:\n%s", pdfPath),
			walk.MsgBoxIconError)
		os.Exit(1)
	}
	log.Println("main: Input file exists. Converting...")

	// 3. Extract text from document
	mdContent, err := convertWithProgress(pdfPath)
	if err != nil {
		log.Printf("main: Error converting file: %v", err)
		walk.MsgBox(nil, "Conversion Error",
			fmt.Sprintf("Could not extract text from the document:\n%v", err),
			walk.MsgBoxIconError)
		if shouldCleanupFile {
			os.Remove(pdfPath)
		}
		os.Exit(1)
	}
	log.Printf("main: Conversion succeeded. Character count = %d.", len(mdContent))

	// Clean up the temporary file immediately if it was a spool file
	if shouldCleanupFile {
		err = os.Remove(pdfPath)
		log.Printf("main: Temp file cleanup result: %v", err)
	}

	// 4. Open Save File Dialog
	defaultName := "document"
	if pdfTitle := extractPdfTitle(pdfPath); pdfTitle != "" {
		defaultName = sanitizeFilename(pdfTitle)
	} else if len(os.Args) >= 3 && os.Args[2] != "" {
		defaultName = sanitizeFilename(os.Args[2])
	} else {
		spoolName := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
		if !strings.HasPrefix(strings.ToLower(spoolName), "spool") &&
			!strings.HasPrefix(strings.ToLower(spoolName), "print") &&
			!strings.HasPrefix(strings.ToLower(spoolName), "job_") &&
			spoolName != "" {
			defaultName = sanitizeFilename(spoolName)
		}
	}

	dlg := &walk.FileDialog{
		Title:          "Save Document as Markdown",
		Filter:         "Markdown File (*.md)|*.md|All Files (*.*)|*.*",
		InitialDirPath: filepath.Join(os.Getenv("USERPROFILE"), "Documents"),
		FilePath:       defaultName + ".md",
	}

	log.Printf("main: FileDialog configuration: InitialDirPath = %s, FilePath = %s", dlg.InitialDirPath, dlg.FilePath)
	ok, err := dlg.ShowSave(nil)
	log.Printf("main: ShowSave result = %v, err = %v", ok, err)
	if err != nil {
		log.Printf("main: Error showing FileDialog: %v", err)
		walk.MsgBox(nil, "Error", fmt.Sprintf("Error opening the save file dialog: %v", err), walk.MsgBoxIconError)
	} else if ok {
		log.Printf("main: Writing markdown to target = %s", dlg.FilePath)
		err = os.WriteFile(dlg.FilePath, []byte(mdContent), 0644)
		if err != nil {
			log.Printf("main: Error writing output file: %v", err)
			walk.MsgBox(nil, "Save Error", fmt.Sprintf("Could not save the Markdown file:\n%v", err), walk.MsgBoxIconError)
		} else {
			log.Println("main: File saved successfully!")
			walk.MsgBox(nil, "Print Completed", fmt.Sprintf("The file was saved successfully:\n%s", dlg.FilePath), walk.MsgBoxIconInformation)
		}
	} else {
		log.Println("main: Save dialog cancelled by user.")
	}
}

func extractPdfTitle(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return ""
	}
	size := info.Size()
	limit := int64(10 * 1024 * 1024) // 10MB limit
	if size < limit {
		limit = size
	}
	b := make([]byte, limit)
	n, _ := f.Read(b)
	content := string(b[:n])

	idx := strings.Index(content, "/Title")
	if idx == -1 {
		return ""
	}

	sub := content[idx:]
	openIdx := strings.Index(sub, "(")
	closeIdx := strings.Index(sub, ")")
	if openIdx != -1 && closeIdx != -1 && closeIdx > openIdx {
		title := sub[openIdx+1 : closeIdx]
		title = strings.ReplaceAll(title, `\\`, `\`)
		title = strings.ReplaceAll(title, `\(`, `(`,)
		title = strings.ReplaceAll(title, `\)`, `)`)
		title = strings.TrimSpace(title)
		if title != "" {
			return title
		}
	}

	openHex := strings.Index(sub, "<")
	closeHex := strings.Index(sub, ">")
	if openHex != -1 && closeHex != -1 && closeHex > openHex {
		hexStr := strings.TrimSpace(sub[openHex+1 : closeHex])
		decoded, err := decodePdfHex(hexStr)
		if err == nil && decoded != "" {
			return decoded
		}
	}

	return ""
}

func decodePdfHex(hexStr string) (string, error) {
	var bytes []byte
	for i := 0; i < len(hexStr)-1; i += 2 {
		var b byte
		_, err := fmt.Sscanf(hexStr[i:i+2], "%02x", &b)
		if err != nil {
			return "", err
		}
		bytes = append(bytes, b)
	}

	if len(bytes) >= 2 && bytes[0] == 0xFE && bytes[1] == 0xFF {
		u16s := make([]uint16, (len(bytes)-2)/2)
		for i := 0; i < len(u16s); i++ {
			u16s[i] = uint16(bytes[2+2*i])<<8 | uint16(bytes[2+2*i+1])
		}
		return string(utf16.Decode(u16s)), nil
	}

	return string(bytes), nil
}
