# Mkd & Epub Exporters

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A unified repository containing high-performance utilities to convert documents to clean **Markdown (.md)** and **EPUB (.epub)** formats. 

This project contains two main packages:
1. **Desktop App (GUI / CLI)**: Drag-and-drop graphical user interface and command-line tool.
2. **Windows Virtual Printers**: Standalone print-to-file virtual printers to print any document directly to Markdown or EPUB from any application (Word, Edge, etc.).

---

## Features

### 💻 Desktop App (`markitdown.exe`)
*   **Dual Mode**: Win32 desktop application (built with Walk) and a cross-platform command-line tool.
*   **Drag-and-Drop**: Queue multiple files and folders directly from Windows Explorer.
*   **Image Handling**: Options to either extract and copy images to a local folder or embed them inline as self-contained Base64 Data URIs.
*   **Dual Output**: Generate `.md`, `.epub`, or both simultaneously.

### 🖨️ Virtual Printers (`Print to Markdown` & `Print to EPUB`)
*   **Direct Capture**: Prints files from any Windows application directly into clean Markdown or EPUB.
*   **Session 0 Bypass**: Safely transitions execution from the Windows Print Spooler service context to the active user's desktop session to show file save dialogs.
*   **Automatic Focus**: Uses Windows API calls to ensure Save dialogs pop up in the foreground.
*   **Page Break Preservation**: Automatically converts physical page breaks into standard markdown dividers (`---`).

---

## Directory Structure

*   [**`go/`**](./go): Contains the Go application source code.
    *   [**`pkg/converter/`**](./go/pkg/converter): Shared parser package (advanced PDF layout parsing, DOCX, HTML, EPUB compiler, image embedding).
    *   [**`cmd/`**](./go/cmd): Command packages for individual components:
        *   `converter-gui`: Desktop GUI application.
        *   `converter-cli`: Desktop CLI application.
        *   `printer-markdown`: Virtual printer capturing Markdown prints.
        *   `printer-epub`: Virtual printer capturing EPUB prints.
        *   `print-watcher`: Background service capturing spooler files.
        *   `installer-markdown`/`installer-epub`: Double-clickable UAC installer wrappers.
*   [**`python/`**](./python): Original python converter application.
*   [**`resources/`**](./resources): Port monitors (`mfilemon.dll`), python executables, and print drivers.
*   [**`windows/`**](./windows): Contains Windows-specific setup scripts (build script and manual virtual printer installer).
*   [**`macos/`**](./macos): Contains macOS-specific print service scripts.

---

## Installation

### 🚀 Windows Installation (Recommended for most users)

To install MarkItDown and/or the Virtual Printers on Windows without compiling from source:
1. Download **`markitdown-setup.exe`** from the [GitHub Releases](https://github.com/gxblary/mkd-epub-exporters/releases) page.
2. Run the installer.
3. Choose the components you want to install:
   * **MarkItDown Converter**: The standalone application (`markitdown.exe` GUI and `markitdown-cli.exe` CLI).
   * **Virtual Printers**: The "Print to Markdown" and "Print to EPUB" virtual printers.
4. Follow the setup wizard and click **Install**.

---

### 🍎 macOS Installation (Recommended for most users)

To install MarkItDown and the Virtual Printers on macOS:
1. Download **`markitdown-setup-macos.pkg`** from the [GitHub Releases](https://github.com/gxblary/mkd-epub-exporters/releases) page.
2. Double-click the downloaded installer and follow the standard instructions.
3. This installer will automatically configure:
   * **CLI Converter**: `/usr/local/bin/markitdown-cli` (natively optimized for both Apple Silicon M1/M2/M3 and Intel CPUs).
   * **PDF Services**: Registers system-wide print shortcuts.
4. **How to use (Virtual Printers)**:
   * Open any document (in Safari, Chrome, Word, Pages, Preview, etc.).
   * Press **CMD + P** to open the print dialog.
   * Click the **PDF** dropdown menu button at the bottom left of the print settings.
   * Select **"Print to Markdown"** or **"Print to EPUB"**.
   * A dialog will pop up requesting where you would like to save the file.

---

### 💻 Advanced Installation & Development (For Power Users / Developers)

#### Requirements
*   **Go** (version 1.22 or higher)
*   **Windows (to compile GUI and resource manifests):**
    Ensure you have `rsrc` installed:
    ```bash
    go install github.com/akavel/rsrc@latest
    ```

#### Building the Unified Installer from Source
To compile all binaries, download external dependencies, and build the unified setup wizard, run from the repository root folder:
```powershell
Set-ExecutionPolicy Bypass -Scope Process
.\windows\build_installer.ps1
```
This generates `markitdown-setup.exe` in the root folder.

#### Compiling the GUI & CLI Separately
Run from the `go/` folder:
```bash
# GUI Mode (Windows)
go build -ldflags="-H=windowsgui" -o ../markitdown.exe ./cmd/converter-gui

# CLI Mode (Cross-platform)
go build -o ../markitdown-cli.exe ./cmd/converter-cli
```

#### Manual Installation of Virtual Printers
If you want to manually install the virtual printers without using the GUI installer:
1. Open **PowerShell as Administrator**.
2. Run the installation script from the root folder:
   ```powershell
   Set-ExecutionPolicy Bypass -Scope Process
   .\windows\install.ps1
   ```
This script will compile `print-watcher.exe`, `markitdown-printer.exe`, and `epub-printer.exe` and configure the physical port mappings on your system.

---

## License

This project is released under the **MIT License**.

It integrates several open source libraries including:
* **lxn/walk** (Windows GUI) - BSD 3-Clause
* **ledongthuc/pdf** (PDF parser) - BSD 3-Clause
* **go-shiori/go-epub** (EPUB generator) - MIT
* **yuin/goldmark** (Markdown converter) - MIT
* **lomo74/mfilemon** (Multi File Port Monitor) - GPLv2
