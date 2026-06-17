# PowerShell Script to Build MarkItDown Unified Installer
# Run from repository root

$RootDir = $PSScriptRoot
$EmbedDir = Join-Path $RootDir "go\cmd\installer\embed"

Write-Host "=== Starting Unified Installer Build Process ===" -ForegroundColor Cyan

# 1. Ensure embed directory exists
if (-not (Test-Path $EmbedDir)) {
    New-Item -ItemType Directory -Path $EmbedDir -Force | Out-Null
}

# 2. Check for Go compiler
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go compiler not found! Go must be installed to build the project."
    Exit 1
}

# 3. Check for rsrc utility (needed for Windows manifests)
if (-not (Get-Command rsrc -ErrorAction SilentlyContinue)) {
    Write-Host "Installing rsrc tool..." -ForegroundColor Yellow
    go install github.com/akavel/rsrc@latest
    # Check if go/bin is in PATH, if not add it dynamically for this session
    $GoBin = Join-Path (go env GOPATH) "bin"
    if ($env:PATH -notlike "*$GoBin*") {
        $env:PATH += ";$GoBin"
    }
}

# 4. Compile Sub-Binaries
Write-Host "Compiling sub-binaries..." -ForegroundColor Yellow

Push-Location (Join-Path $RootDir "go")
try {
    Write-Host "  Building markitdown.exe (GUI)..."
    go build -ldflags="-H=windowsgui" -o "$EmbedDir\markitdown.exe" ./cmd/converter-gui
    if ($LASTEXITCODE -ne 0) { throw "go build markitdown.exe failed" }

    Write-Host "  Building markitdown-cli.exe (CLI)..."
    go build -o "$EmbedDir\markitdown-cli.exe" ./cmd/converter-cli
    if ($LASTEXITCODE -ne 0) { throw "go build markitdown-cli.exe failed" }

    Write-Host "  Building print-watcher.exe..."
    go build -ldflags="-H=windowsgui" -o "$EmbedDir\print-watcher.exe" ./cmd/print-watcher
    if ($LASTEXITCODE -ne 0) { throw "go build print-watcher.exe failed" }

    Write-Host "  Building markitdown-printer.exe..."
    go build -ldflags="-H=windowsgui" -o "$EmbedDir\markitdown-printer.exe" ./cmd/printer-markdown
    if ($LASTEXITCODE -ne 0) { throw "go build markitdown-printer.exe failed" }

    Write-Host "  Building epub-printer.exe..."
    go build -ldflags="-H=windowsgui" -o "$EmbedDir\epub-printer.exe" ./cmd/printer-epub
    if ($LASTEXITCODE -ne 0) { throw "go build epub-printer.exe failed" }
} catch {
    Write-Error "Failed during sub-binary Go compilation: $_"
    Pop-Location
    Exit 1
}
Pop-Location

# 5. Copy DLLs and Licenses
Write-Host "Copying library DLLs and licenses..." -ForegroundColor Yellow
Copy-Item -Path (Join-Path $RootDir "resources\mfilemon.dll") -Destination "$EmbedDir\mfilemon.dll" -Force
Copy-Item -Path (Join-Path $RootDir "resources\mfilemonUI.dll") -Destination "$EmbedDir\mfilemonUI.dll" -Force
Copy-Item -Path (Join-Path $RootDir "LICENSE-THIRD-PARTY.md") -Destination "$EmbedDir\LICENSE-THIRD-PARTY.md" -Force

# 6. Download Pandoc if not present
$PandocPath = Join-Path $EmbedDir "pandoc.exe"
if (-not (Test-Path $PandocPath)) {
    Write-Host "Pandoc not found in embed folder. Downloading..." -ForegroundColor Yellow
    $ZipPath = Join-Path $RootDir "pandoc.zip"
    $TempDir = Join-Path $RootDir "pandoc_temp"
    
    Invoke-WebRequest -Uri "https://github.com/jgm/pandoc/releases/download/3.1.11.1/pandoc-3.1.11.1-windows-x86_64.zip" -OutFile $ZipPath
    Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force
    Copy-Item -Path "$TempDir\pandoc-3.1.11.1\pandoc.exe" -Destination $PandocPath -Force
    
    # Cleanup
    Remove-Item -Path $ZipPath -Force
    Remove-Item -Recurse -Force $TempDir
}

# 7. Compile Windows Resources for the Installer
Write-Host "Compiling Windows resources for the installer..." -ForegroundColor Yellow
Push-Location (Join-Path $RootDir "go\cmd\installer")
try {
    rsrc -manifest installer.manifest -o rsrc.syso
} catch {
    Write-Error "Failed to compile resources using rsrc: $_"
    Pop-Location
    Exit 1
}
Pop-Location

# 8. Compile the Final Installer
Write-Host "Building markitdown-setup.exe..." -ForegroundColor Yellow
Push-Location (Join-Path $RootDir "go")
try {
    go build -ldflags="-H=windowsgui" -o "$RootDir\markitdown-setup.exe" ./cmd/installer
    if ($LASTEXITCODE -ne 0) { throw "go build installer failed" }
} catch {
    Write-Error "Failed to compile installer executable: $_"
    Pop-Location
    Exit 1
}
Pop-Location

Write-Host "=== Installer Build Successfully Completed! ===" -ForegroundColor Green
Write-Host "Output: markitdown-setup.exe"
