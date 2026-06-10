//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// FileListModel provides data to the ListBox widget
type FileListModel struct {
	walk.ListModelBase
	items []string
}

func (m *FileListModel) ItemCount() int {
	return len(m.items)
}

func (m *FileListModel) Value(index int) interface{} {
	return m.items[index]
}

func (m *FileListModel) ResetItems(items []string) {
	m.items = items
	m.PublishItemsReset()
}

// AppUI holds references to UI components and state
type AppUI struct {
	mainWindow     *walk.MainWindow
	listBox        *walk.ListBox
	outputDirLabel *walk.Label
	statusLabel    *walk.Label
	convertButton  *walk.PushButton

	selectedPaths []string
	outputDir     string
	model         *FileListModel
}

func main() {
	// Lock the OS thread for Windows GUI loop
	runtime.LockOSThread()

	// Locate the external Pandoc binary
	if err := initPandoc(); err != nil {
		log.Printf("Pandoc initialization warning: %v", err)
	}

	app := &AppUI{
		model: &FileListModel{},
	}

	var openFilesBtn *walk.PushButton
	var openDirBtn *walk.PushButton
	var clearBtn *walk.PushButton
	var outDirBtn *walk.PushButton

	err := MainWindow{
		AssignTo: &app.mainWindow,
		Title:    "MarkItDown - Converter",
		MinSize:  Size{Width: 700, Height: 500},
		Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
		OnDropFiles: func(files []string) {
			// Handle drag-and-drop of files and folders
			for _, f := range files {
				if !contains(app.selectedPaths, f) {
					app.selectedPaths = append(app.selectedPaths, f)
				}
			}
			app.model.ResetItems(app.selectedPaths)
		},
		Children: []Widget{
			// Top buttons frame
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					PushButton{
						AssignTo: &openFilesBtn,
						Text:     "Add File(s)",
						OnClicked: func() {
							app.addFiles()
						},
					},
					PushButton{
						AssignTo: &openDirBtn,
						Text:     "Add Folder",
						OnClicked: func() {
							app.addDirectory()
						},
					},
					HSpacer{},
					PushButton{
						AssignTo: &clearBtn,
						Text:     "Clear list",
						OnClicked: func() {
							app.clearList()
						},
					},
				},
			},

			// ListBox
			ListBox{
				AssignTo: &app.listBox,
				Model:    app.model,
			},

			// Output Dir selection frame
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					PushButton{
						AssignTo: &outDirBtn,
						Text:     "Output folder",
						OnClicked: func() {
							app.selectOutputDir()
						},
					},
					Label{
						AssignTo: &app.outputDirLabel,
						Text:     "(No folder selected)",
					},
				},
			},

			// Bottom Status and Convert Button frame
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					Label{
						AssignTo: &app.statusLabel,
						Text:     "Ready",
					},
					HSpacer{},
					PushButton{
						AssignTo: &app.convertButton,
						Text:     "Convert",
						OnClicked: func() {
							app.startConversion()
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

// contains checks if a string is present in a slice
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// addFiles shows a file selector to add files to the list
func (app *AppUI) addFiles() {
	dlg := &walk.FileDialog{
		Title:  "Select files",
		Filter: "All supported files (*.docx;*.xlsx;*.pdf;*.html;*.htm;*.txt;*.md;*.pptx;*.rtf;*.epub;*.odt;*.tex;*.wiki)|*.docx;*.xlsx;*.pdf;*.html;*.htm;*.txt;*.md;*.pptx;*.rtf;*.epub;*.odt;*.tex;*.wiki|All files (*.*)|*.*",
	}
	if ok, err := dlg.ShowOpenMultiple(app.mainWindow); err != nil {
		return
	} else if ok {
		for _, f := range dlg.FilePaths {
			if !contains(app.selectedPaths, f) {
				app.selectedPaths = append(app.selectedPaths, f)
			}
		}
		app.model.ResetItems(app.selectedPaths)
	}
}

// addDirectory shows a directory selector to add folders to the list
func (app *AppUI) addDirectory() {
	dlg := &walk.FileDialog{
		Title: "Select a folder",
	}
	if ok, err := dlg.ShowBrowseFolder(app.mainWindow); err != nil {
		return
	} else if ok {
		d := dlg.FilePath
		if d != "" && !contains(app.selectedPaths, d) {
			app.selectedPaths = append(app.selectedPaths, d)
			app.model.ResetItems(app.selectedPaths)
		}
	}
}

// clearList empties the list of files to convert
func (app *AppUI) clearList() {
	app.selectedPaths = nil
	app.model.ResetItems(nil)
}

// selectOutputDir displays a dialog to select the destination folder
func (app *AppUI) selectOutputDir() {
	dlg := &walk.FileDialog{
		Title: "Select output folder",
	}
	if ok, err := dlg.ShowBrowseFolder(app.mainWindow); err != nil {
		return
	} else if ok {
		app.outputDir = dlg.FilePath
		app.outputDirLabel.SetText(app.outputDir)
	}
}

// startConversion starts the conversion process in a background goroutine
func (app *AppUI) startConversion() {
	if len(app.selectedPaths) == 0 {
		walk.MsgBox(app.mainWindow, "Warning", "Please select at least one file or folder.", walk.MsgBoxIconWarning)
		return
	}
	if app.outputDir == "" {
		walk.MsgBox(app.mainWindow, "Warning", "Please select an output folder.", walk.MsgBoxIconWarning)
		return
	}

	app.convertButton.SetEnabled(false)
	app.statusLabel.SetText("Conversion in progress...")

	go app.processFiles()
}

// processFiles collects files, runs converters and writes markdown outputs
func (app *AppUI) processFiles() {
	// Collect all files recursively
	allFiles, err := collectFiles(app.selectedPaths)
	if err != nil {
		app.mainWindow.Synchronize(func() {
			app.statusLabel.SetText("Error reading files.")
			app.convertButton.SetEnabled(true)
			walk.MsgBox(app.mainWindow, "Error", fmt.Sprintf("Unable to list files: %v", err), walk.MsgBoxIconError)
		})
		return
	}

	if len(allFiles) == 0 {
		app.mainWindow.Synchronize(func() {
			app.statusLabel.SetText("Ready")
			app.convertButton.SetEnabled(true)
			walk.MsgBox(app.mainWindow, "Information", "No files found in selection.", walk.MsgBoxIconInformation)
		})
		return
	}

	successCount := 0
	errorCount := 0
	var successPaths []string

	for i, filePath := range allFiles {
		// Update status (thread-safe UI update)
		idx := i
		fPath := filePath
		app.mainWindow.Synchronize(func() {
			app.statusLabel.SetText(fmt.Sprintf("Converting %d/%d: %s", idx+1, len(allFiles), filepath.Base(fPath)))
		})

		// Perform conversion
		mdContent, err := convertFile(fPath)
		if err != nil {
			errorCount++
			continue
		}

		// Avoid name collisions in output folder
		outName := strings.TrimSuffix(filepath.Base(fPath), filepath.Ext(fPath)) + ".md"
		outPath := filepath.Join(app.outputDir, outName)

		counter := 1
		for {
			if _, err := os.Stat(outPath); os.IsNotExist(err) {
				break
			}
			stem := strings.TrimSuffix(filepath.Base(fPath), filepath.Ext(fPath))
			outPath = filepath.Join(app.outputDir, fmt.Sprintf("%s_%d.md", stem, counter))
			counter++
		}

		// Write conversion result to file
		err = os.WriteFile(outPath, []byte(mdContent), 0644)
		if err != nil {
			errorCount++
			continue
		}

		successCount++
		successPaths = append(successPaths, outPath)
	}

	// Show results popup on UI thread when done
	app.mainWindow.Synchronize(func() {
		app.statusLabel.SetText(fmt.Sprintf("Completed. Success: %d, Failed/Ignored: %d", successCount, errorCount))
		app.convertButton.SetEnabled(true)
		app.showResults(successPaths, errorCount)
	})
}

// openFile opens a file using Windows default application
func openFile(path string) {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.Start()
}

// showResults displays a dialog with the results of the conversion
func (app *AppUI) showResults(successPaths []string, errorCount int) {
	if len(successPaths) == 0 {
		walk.MsgBox(app.mainWindow, "Completed", fmt.Sprintf("No files were converted.\nFailed/Ignored: %d", errorCount), walk.MsgBoxIconInformation)
		return
	}

	var dlg *walk.Dialog
	var listbox *walk.ListBox

	model := &FileListModel{items: successPaths}

	err := Dialog{
		AssignTo: &dlg,
		Title:    "Conversion Results",
		MinSize:  Size{Width: 600, Height: 400},
		Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}},
		Children: []Widget{
			Label{
				Text: fmt.Sprintf("Conversion completed (Success: %d, Failed: %d).\nDouble-click a file below to open it:", len(successPaths), errorCount),
			},
			ListBox{
				AssignTo: &listbox,
				Model:    model,
				OnItemActivated: func() {
					idx := listbox.CurrentIndex()
					if idx >= 0 && idx < len(successPaths) {
						path := successPaths[idx]
						openFile(path)
					}
				},
			},
			PushButton{
				Text: "Close",
				OnClicked: func() {
					dlg.Accept()
				},
			},
		},
	}.Create(app.mainWindow)

	if err == nil {
		dlg.Run()
	}
}
