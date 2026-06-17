//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows/registry"
)

// Embedded files (compiled during the build process)
//go:embed embed/markitdown.exe
var markitdownExe []byte

//go:embed embed/markitdown-cli.exe
var markitdownCliExe []byte

//go:embed embed/pandoc.exe
var pandocExe []byte

//go:embed embed/print-watcher.exe
var printWatcherExe []byte

//go:embed embed/markitdown-printer.exe
var markitdownPrinterExe []byte

//go:embed embed/epub-printer.exe
var epubPrinterExe []byte

//go:embed embed/mfilemon.dll
var mfilemonDll []byte

//go:embed embed/mfilemonUI.dll
var mfilemonUIDll []byte

//go:embed embed/LICENSE-THIRD-PARTY.md
var licenseMd []byte

type InstallerUI struct {
	mainWindow         *walk.MainWindow
	destDirLE          *walk.LineEdit
	installConverterCB *walk.CheckBox
	installPrintersCB  *walk.CheckBox
	desktopShortcutCB  *walk.CheckBox
	startMenuShortcutCB *walk.CheckBox
	installBtn         *walk.PushButton
	cancelBtn          *walk.PushButton
	progressBar        *walk.ProgressBar
	statusLabel        *walk.Label
}

func main() {
	runtime.LockOSThread()

	app := &InstallerUI{}

	var browseBtn *walk.PushButton

	err := MainWindow{
		AssignTo: &app.mainWindow,
		Title:    "MarkItDown & Virtual Printers Setup",
		MinSize:  Size{Width: 550, Height: 380},
		Layout:   VBox{Margins: Margins{Left: 15, Top: 15, Right: 15, Bottom: 15}},
		Children: []Widget{
			Label{
				Text: "Welcome to the MarkItDown & Virtual Printers Installer.\nThis wizard will install the selected components on your system.",
				Font: Font{PointSize: 10, Bold: true},
			},
			VSpacer{Size: 10},

			// Destination Directory
			GroupBox{
				Title:  "Destination Folder",
				Layout: HBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
				Children: []Widget{
					LineEdit{
						AssignTo: &app.destDirLE,
						Text:     `C:\Program Files\MarkItDown`,
					},
					PushButton{
						AssignTo: &browseBtn,
						Text:     "Browse...",
						OnClicked: func() {
							dlg := &walk.FileDialog{
								Title: "Select Installation Folder",
							}
							if ok, err := dlg.ShowBrowseFolder(app.mainWindow); err == nil && ok {
								app.destDirLE.SetText(dlg.FilePath)
							}
						},
					},
				},
			},

			// Components Selection
			GroupBox{
				Title:  "Select Components to Install",
				Layout: VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
				Children: []Widget{
					CheckBox{
						AssignTo: &app.installConverterCB,
						Text:     "MarkItDown Desktop Converter (GUI & CLI)",
						Checked:  true,
					},
					CheckBox{
						AssignTo: &app.installPrintersCB,
						Text:     "Windows Virtual Printers (Print to Markdown/EPUB)",
						Checked:  true,
					},
				},
			},

			// Shortcuts Selection
			GroupBox{
				Title:  "Shortcuts",
				Layout: HBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
				Children: []Widget{
					CheckBox{
						AssignTo: &app.desktopShortcutCB,
						Text:     "Desktop Shortcut",
						Checked:  true,
					},
					CheckBox{
						AssignTo: &app.startMenuShortcutCB,
						Text:     "Start Menu Shortcut",
						Checked:  true,
					},
				},
			},

			VSpacer{Size: 5},

			// Progress and Status
			Label{
				AssignTo: &app.statusLabel,
				Text:     "Ready to install.",
			},
			ProgressBar{
				AssignTo: &app.progressBar,
				MinValue: 0,
				MaxValue: 100,
			},

			VSpacer{Size: 5},

			// Action Buttons
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{
						AssignTo: &app.installBtn,
						Text:     "Install",
						OnClicked: func() {
							app.startInstallation()
						},
					},
					PushButton{
						AssignTo: &app.cancelBtn,
						Text:     "Cancel",
						OnClicked: func() {
							app.mainWindow.Close()
						},
					},
				},
			},
		},
	}.Create()

	if err != nil {
		log.Fatal(err)
	}

	app.mainWindow.Run()
}

func (app *InstallerUI) startInstallation() {
	destDir := app.destDirLE.Text()
	if destDir == "" {
		walk.MsgBox(app.mainWindow, "Error", "Please select a valid installation folder.", walk.MsgBoxIconError)
		return
	}

	instConv := app.installConverterCB.Checked()
	instPrint := app.installPrintersCB.Checked()

	if !instConv && !instPrint {
		walk.MsgBox(app.mainWindow, "Warning", "Please select at least one component to install.", walk.MsgBoxIconWarning)
		return
	}

	app.installBtn.SetEnabled(false)
	app.destDirLE.SetEnabled(false)
	app.installConverterCB.SetEnabled(false)
	app.installPrintersCB.SetEnabled(false)
	app.desktopShortcutCB.SetEnabled(false)
	app.startMenuShortcutCB.SetEnabled(false)
	app.cancelBtn.SetText("Exit")

	go app.runInstallProcess(destDir, instConv, instPrint)
}

func (app *InstallerUI) updateStatus(text string, progress int) {
	app.mainWindow.Synchronize(func() {
		app.statusLabel.SetText(text)
		app.progressBar.SetValue(progress)
	})
}

func (app *InstallerUI) runInstallProcess(destDir string, instConv, instPrint bool) {
	app.updateStatus("Creating installation directory...", 10)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		app.showError("Failed to create installation directory", err)
		return
	}

	// Always write third-party license
	licensePath := filepath.Join(destDir, "LICENSE-THIRD-PARTY.md")
	if err := os.WriteFile(licensePath, licenseMd, 0644); err != nil {
		app.showError("Failed to write license file", err)
		return
	}

	if instConv {
		app.updateStatus("Extracting MarkItDown Converter...", 20)
		time.Sleep(200 * time.Millisecond)

		if err := os.WriteFile(filepath.Join(destDir, "markitdown.exe"), markitdownExe, 0755); err != nil {
			app.showError("Failed to write markitdown.exe", err)
			return
		}

		if err := os.WriteFile(filepath.Join(destDir, "markitdown-cli.exe"), markitdownCliExe, 0755); err != nil {
			app.showError("Failed to write markitdown-cli.exe", err)
			return
		}

		app.updateStatus("Extracting Pandoc engine...", 35)
		time.Sleep(200 * time.Millisecond)
		if err := os.WriteFile(filepath.Join(destDir, "pandoc.exe"), pandocExe, 0755); err != nil {
			app.showError("Failed to write pandoc.exe", err)
			return
		}
	}

	if instPrint {
		app.updateStatus("Stopping background print watcher...", 50)
		_ = exec.Command("taskkill", "/F", "/IM", "print-watcher.exe").Run()
		time.Sleep(500 * time.Millisecond)

		app.updateStatus("Extracting virtual printer files...", 55)
		if err := os.WriteFile(filepath.Join(destDir, "print-watcher.exe"), printWatcherExe, 0755); err != nil {
			app.showError("Failed to write print-watcher.exe", err)
			return
		}
		if err := os.WriteFile(filepath.Join(destDir, "markitdown-printer.exe"), markitdownPrinterExe, 0755); err != nil {
			app.showError("Failed to write markitdown-printer.exe", err)
			return
		}
		if err := os.WriteFile(filepath.Join(destDir, "epub-printer.exe"), epubPrinterExe, 0755); err != nil {
			app.showError("Failed to write epub-printer.exe", err)
			return
		}
		if err := os.WriteFile(filepath.Join(destDir, "mfilemon.dll"), mfilemonDll, 0755); err != nil {
			app.showError("Failed to write mfilemon.dll", err)
			return
		}
		if err := os.WriteFile(filepath.Join(destDir, "mfilemonUI.dll"), mfilemonUIDll, 0755); err != nil {
			app.showError("Failed to write mfilemonUI.dll", err)
			return
		}

		app.updateStatus("Configuring spool folders and permissions...", 65)
		mkdSpool := `C:\Windows\Temp\markitdown-spool`
		epubSpool := `C:\Windows\Temp\epub-spool`
		_ = os.MkdirAll(mkdSpool, 0777)
		_ = os.MkdirAll(epubSpool, 0777)

		// Set full permissions using icacls
		_ = exec.Command("icacls.exe", mkdSpool, "/grant", "*S-1-5-32-545:(OI)(CI)F", "/T", "/C", "/Q").Run()
		_ = exec.Command("icacls.exe", mkdSpool, "/grant", "*S-1-5-18:(OI)(CI)F", "/T", "/C", "/Q").Run()
		_ = exec.Command("icacls.exe", epubSpool, "/grant", "*S-1-5-32-545:(OI)(CI)F", "/T", "/C", "/Q").Run()
		_ = exec.Command("icacls.exe", epubSpool, "/grant", "*S-1-5-18:(OI)(CI)F", "/T", "/C", "/Q").Run()

		app.updateStatus("Registering printer registry ports...", 75)
		if err := registerSpoolPorts(); err != nil {
			app.showError("Failed to register printer ports", err)
			return
		}

		app.updateStatus("Restarting print spooler service...", 80)
		_ = exec.Command("net", "stop", "spooler").Run()
		time.Sleep(500 * time.Millisecond)
		if err := exec.Command("net", "start", "spooler").Run(); err != nil {
			app.showError("Failed to restart Print Spooler", err)
			return
		}

		app.updateStatus("Installing virtual printers...", 85)
		if err := configurePrinters(); err != nil {
			app.showError("Failed to install printers", err)
			return
		}

		app.updateStatus("Configuring print watcher auto-startup...", 90)
		watcherPath := filepath.Join(destDir, "print-watcher.exe")
		if err := registerStartupKey(watcherPath); err != nil {
			app.showError("Failed to set startup key", err)
			return
		}

		// Start the watcher
		_ = exec.Command("cmd.exe", "/c", "start", "", watcherPath).Start()
	}

	if instConv {
		app.updateStatus("Creating shortcuts...", 95)
		targetPath := filepath.Join(destDir, "markitdown.exe")
		if app.desktopShortcutCB.Checked() {
			_ = createShortcutLink(targetPath, "MarkItDown.lnk", "Document to Markdown & EPUB Converter", false)
		}
		if app.startMenuShortcutCB.Checked() {
			_ = createShortcutLink(targetPath, "MarkItDown.lnk", "Document to Markdown & EPUB Converter", true)
		}
	}

	app.updateStatus("Installation completed successfully!", 100)
	app.mainWindow.Synchronize(func() {
		walk.MsgBox(app.mainWindow, "Success", "Installation completed successfully!", walk.MsgBoxIconInformation)
		app.mainWindow.Close()
	})
}

func (app *InstallerUI) showError(title string, err error) {
	app.mainWindow.Synchronize(func() {
		walk.MsgBox(app.mainWindow, "Error", fmt.Sprintf("%s: %v", title, err), walk.MsgBoxIconError)
		app.installBtn.SetEnabled(true)
		app.destDirLE.SetEnabled(true)
		app.installConverterCB.SetEnabled(true)
		app.installPrintersCB.SetEnabled(true)
		app.desktopShortcutCB.SetEnabled(true)
		app.startMenuShortcutCB.SetEnabled(true)
		app.cancelBtn.SetText("Cancel")
	})
}

func registerSpoolPorts() error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Ports`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if err := key.SetStringValue(`C:\Windows\Temp\markitdown-spool\spool.pdf`, ""); err != nil {
		return err
	}
	if err := key.SetStringValue(`C:\Windows\Temp\epub-spool\spool.pdf`, ""); err != nil {
		return err
	}
	return nil
}

func configurePrinters() error {
	cmdStr := `
	$DriverName = "Microsoft Print To PDF"
	if (-not (Get-PrinterDriver -Name $DriverName -ErrorAction SilentlyContinue)) {
		Enable-WindowsOptionalFeature -Online -FeatureName "Printing-PrintToPDFServices-Features" -All -NoRestart -ErrorAction SilentlyContinue
		Add-PrinterDriver -Name $DriverName -ErrorAction SilentlyContinue
	}
	if (Get-Printer -Name "Print to Markdown" -ErrorAction SilentlyContinue) {
		Remove-Printer -Name "Print to Markdown"
	}
	Add-Printer -Name "Print to Markdown" -DriverName $DriverName -PortName "C:\Windows\Temp\markitdown-spool\spool.pdf"
	if (Get-Printer -Name "Print to EPUB" -ErrorAction SilentlyContinue) {
		Remove-Printer -Name "Print to EPUB"
	}
	Add-Printer -Name "Print to EPUB" -DriverName $DriverName -PortName "C:\Windows\Temp\epub-spool\spool.pdf"
	`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", cmdStr)
	return cmd.Run()
}

func registerStartupKey(watcherPath string) error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		key, err = registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
		if err != nil {
			return err
		}
	}
	defer key.Close()
	return key.SetStringValue("MkdEpubPrintWatcher", fmt.Sprintf(`"%s"`, watcherPath))
}

func createShortcutLink(targetPath, shortcutName, desc string, startMenu bool) error {
	var shortcutFolder string
	if startMenu {
		shortcutFolder = filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`)
	} else {
		shortcutFolder = filepath.Join(os.Getenv("USERPROFILE"), `Desktop`)
	}

	lnkPath := filepath.Join(shortcutFolder, shortcutName)
	psCmd := fmt.Sprintf(`
	$WshShell = New-Object -ComObject WScript.Shell
	$Shortcut = $WshShell.CreateShortcut('%s')
	$Shortcut.TargetPath = '%s'
	$Shortcut.Description = '%s'
	$Shortcut.WorkingDirectory = '%s'
	$Shortcut.Save()
	`, lnkPath, targetPath, desc, filepath.Dir(targetPath))

	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmd)
	return cmd.Run()
}
