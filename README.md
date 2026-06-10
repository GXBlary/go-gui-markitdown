# Go-GUI-MarkItDown

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/GXBlary/go-gui-markitdown)](https://goreportcard.com/report/github.com/GXBlary/go-gui-markitdown)

A lightweight, high-performance port of Microsoft's **MarkItDown** document-to-markdown converter to **pure Go**. It features a native Windows desktop GUI with drag-and-drop support, along with a cross-platform command-line interface (CLI) for macOS and Linux.

---

## Features

*   **Dual Mode:**
    *   **Windows GUI:** Native Win32 desktop application (built with Walk) with lists, progress tracking, and file dialogs.
    *   **Cross-platform CLI:** A clean command-line interface for macOS, Linux, and Unix systems.
*   **Drag-and-Drop:** Drag files or folders directly from Windows Explorer into the GUI to queue them for conversion.
*   **Highly Extensible File Support:**
    *   **Native Go Engine:** Converts PDF (`.pdf`), Word (`.docx`), Excel (`.xlsx`), HTML (`.html`, `.htm`), and text (`.txt`, `.md`) files out-of-the-box.
    *   **Pandoc Integration:** Automatically detects and calls Pandoc to handle advanced document types like PowerPoint (`.pptx`), Rich Text (`.rtf`), EPUB (`.epub`), OpenDocument (`.odt`), LaTeX (`.tex`), and Wiki pages.
*   **No Runtime Dependencies:** Compiles to a single, standalone binary (~13.7 MB) with instant startup.
*   **Automatic Naming Conflict Resolution:** Appends numbers (e.g., `document_1.md`) automatically if a converted file already exists in the output folder.

---

## Directory Structure

*   [**`go/`**](./go): Contains the Go application source code.
    *   `main_windows.go`: GUI entry point using the Windows common controls.
    *   `main_cli.go`: CLI entry point for Linux and macOS.
    *   `converter.go`: Shared document parsing and Pandoc routing logic.
*   [**`python/`**](./python): Contains the original Python script and its PyInstaller `.spec` packaging configuration.

---

## How it Works (Pandoc Auto-Detection)

To keep the application binary lightweight, the Go engine searches for Pandoc dynamically at startup:
1.  In the **same directory** as the application executable (e.g., `pandoc.exe` next to `markitdown.exe` on Windows).
2.  In the system **`PATH`** environment variable.

If Pandoc is found, it will automatically take over `.docx` and `.html` conversions for higher layout fidelity and enable PowerPoint (`.pptx`), EPUB, and RTF parsing. If Pandoc is missing, the application falls back to its built-in native Go parsers for basic formats (PDF, DOCX, XLSX, HTML).

---

## Installation & Compilation

### Requirements
*   **Go** (version 1.22 or higher)
*   **Windows Only (for GUI):** `rsrc` tool to compile the Windows manifest:
    ```bash
    go install github.com/akavel/rsrc@latest
    ```

### Compilation

#### For Windows (GUI Mode)
Compile from the `go/` folder:
```bash
cd go
rsrc -manifest app.manifest -o rsrc.syso
go build -ldflags="-H=windowsgui" -o ../markitdown.exe
```

#### For macOS & Linux (CLI Mode)
Compile the CLI version:
```bash
cd go
go build -o ../markitdown
```

---

## Usage

### GUI Mode (Windows)
Double-click `markitdown.exe` to launch the graphical user interface:
1.  Queue files by clicking **Add File(s)**, **Add Folder**, or by **dragging and dropping** files from Windows Explorer.
2.  Select the **Output folder**.
3.  Click **Convert**.
4.  Double-click any result in the final window to open the Markdown file in your default editor.

### CLI Mode (macOS / Linux / Windows Console)
Run the binary via the terminal:
```bash
./markitdown -out /path/to/output/folder /path/to/document1.docx /path/to/folder2/
```

---

## Third-Party Licenses

This project is released under the **MIT License**. It utilizes the following open-source libraries:
*   **lxn/walk** (GUI) - BSD 3-Clause
*   **ledongthuc/pdf** (PDF parser) - BSD 3-Clause
*   **xuri/excelize** (Excel parser) - BSD 3-Clause
*   **html-to-markdown** (HTML parser) - MIT
*   **docx2md** (Word parser) - MIT
*   **Pandoc** (Bundled in releases) - GPLv2

See [LICENSE-THIRD-PARTY.md](./LICENSE-THIRD-PARTY.md) for full attributions.
