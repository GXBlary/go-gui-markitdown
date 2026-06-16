# Unified Virtual Printers Installer for Markdown & EPUB
# MUST BE RUN AS ADMINISTRATOR

# Check for admin rights
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "This script must be run as an administrator!"
    Exit 1
}

$RootDir = $PSScriptRoot
$InstallDir = "C:\Program Files\MkdEpubPrinters"
$WatcherExe = Join-Path $InstallDir "print-watcher.exe"
$MkdPrinterExe = Join-Path $InstallDir "markitdown-printer.exe"
$EpubPrinterExe = Join-Path $InstallDir "epub-printer.exe"
$MarkitDownCliSource = Join-Path $RootDir "resources\markitdown-cli.exe"
$MarkitDownCliDest = Join-Path $InstallDir "markitdown-cli.exe"

Write-Host "=== Starting Unified Virtual Printers Installation ===" -ForegroundColor Cyan

# 1. Compile all executables
if (Get-Command go -ErrorAction SilentlyContinue) {
    Write-Host "Compiling print-watcher.exe, markitdown-printer.exe, and epub-printer.exe..." -ForegroundColor Yellow
    Push-Location (Join-Path $RootDir "go")
    go build -ldflags="-H=windowsgui" -o print-watcher.exe ./cmd/print-watcher
    go build -ldflags="-H=windowsgui" -o markitdown-printer.exe ./cmd/printer-markdown
    go build -ldflags="-H=windowsgui" -o epub-printer.exe ./cmd/printer-epub
    Pop-Location
} else {
    Write-Warning "Go compiler not found. Using pre-compiled executables if available."
}

# Verify binaries exist
$watcherSrc = Join-Path $RootDir "go\print-watcher.exe"
$mkdSrc = Join-Path $RootDir "go\markitdown-printer.exe"
$epubSrc = Join-Path $RootDir "go\epub-printer.exe"

if (-not (Test-Path $watcherSrc) -or -not (Test-Path $mkdSrc) -or -not (Test-Path $epubSrc)) {
    Write-Error "Missing compiled executables! Compile failed or binaries are missing."
    Exit 1
}

# 2. Stop running print-watcher instance to allow overwrite
Write-Host "Stopping any running print-watcher process..."
Stop-Process -Name "print-watcher" -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

# 3. Create install directory and copy files
Write-Host "Copying executables to permanent location: $InstallDir" -ForegroundColor Yellow
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

Copy-Item -Path $watcherSrc -Destination $WatcherExe -Force
Copy-Item -Path $mkdSrc -Destination $MkdPrinterExe -Force
Copy-Item -Path $epubSrc -Destination $EpubPrinterExe -Force

if (Test-Path $MarkitDownCliSource) {
    Write-Host "Copying markitdown-cli.exe to install directory..."
    Copy-Item -Path $MarkitDownCliSource -Destination $MarkitDownCliDest -Force
} else {
    Write-Warning "markitdown-cli.exe not found in resources. Rich document conversion (docx/pptx/xlsx/html) may not work without python installed."
}

# 4. Create spool folders and set permissions
Write-Host "Creating spool folders and setting permissions..." -ForegroundColor Yellow
$mkdSpool = "C:\Windows\Temp\markitdown-spool"
$epubSpool = "C:\Windows\Temp\epub-spool"

if (-not (Test-Path $mkdSpool)) { New-Item -ItemType Directory -Path $mkdSpool -Force | Out-Null }
if (-not (Test-Path $epubSpool)) { New-Item -ItemType Directory -Path $epubSpool -Force | Out-Null }

# Grant Full Control to Users (S-1-5-32-545) and SYSTEM (S-1-5-18)
& icacls.exe $mkdSpool /grant "*S-1-5-32-545:(OI)(CI)F" /T /C /Q
& icacls.exe $mkdSpool /grant "*S-1-5-18:(OI)(CI)F" /T /C /Q
& icacls.exe $epubSpool /grant "*S-1-5-32-545:(OI)(CI)F" /T /C /Q
& icacls.exe $epubSpool /grant "*S-1-5-18:(OI)(CI)F" /T /C /Q

# 5. Register PDF Spool Local Ports in Registry
Write-Host "Registering PDF spool ports in the registry..." -ForegroundColor Yellow
$PortsRegistryPath = "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Ports"
Set-ItemProperty -Path $PortsRegistryPath -Name "C:\Windows\Temp\markitdown-spool\spool.pdf" -Value "" -Type String -ErrorAction Stop
Set-ItemProperty -Path $PortsRegistryPath -Name "C:\Windows\Temp\epub-spool\spool.pdf" -Value "" -Type String -ErrorAction Stop

# 6. Restart Print Spooler to load new ports
Write-Host "Restarting Print Spooler..." -ForegroundColor Yellow
Restart-Service -Name "spooler" -Force

# 7. Check and install "Microsoft Print To PDF" driver
$DriverName = "Microsoft Print To PDF"
if (-not (Get-PrinterDriver -Name $DriverName -ErrorAction SilentlyContinue)) {
    Write-Host "Driver '$DriverName' not found. Installing Windows Print-to-PDF feature..." -ForegroundColor Yellow
    try {
        Enable-WindowsOptionalFeature -Online -FeatureName "Printing-PrintToPDFServices-Features" -All -NoRestart -ErrorAction Stop
        Write-Host "Feature enabled. Registering driver..."
        Add-PrinterDriver -Name $DriverName -ErrorAction Stop
    } catch {
        Write-Error "Failed to install '$DriverName' driver: $_"
        Exit 1
    }
}

# 8. Create virtual printers
Write-Host "Installing virtual printers..." -ForegroundColor Yellow

# Print to Markdown
if (Get-Printer -Name "Print to Markdown" -ErrorAction SilentlyContinue) {
    Write-Host "Removing existing 'Print to Markdown' printer..."
    Remove-Printer -Name "Print to Markdown"
}
Add-Printer -Name "Print to Markdown" -DriverName $DriverName -PortName "C:\Windows\Temp\markitdown-spool\spool.pdf"

# Print to EPUB
if (Get-Printer -Name "Print to EPUB" -ErrorAction SilentlyContinue) {
    Write-Host "Removing existing 'Print to EPUB' printer..."
    Remove-Printer -Name "Print to EPUB"
}
Add-Printer -Name "Print to EPUB" -DriverName $DriverName -PortName "C:\Windows\Temp\epub-spool\spool.pdf"

# 9. Register print-watcher.exe in HKLM Run for all users
Write-Host "Configuring automatic startup for print-watcher..." -ForegroundColor Yellow
$RunRegistryPath = "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run"
Set-ItemProperty -Path $RunRegistryPath -Name "MkdEpubPrintWatcher" -Value "`"$WatcherExe`"" -Type String -ErrorAction Stop

# 10. Start print-watcher process immediately
Write-Host "Starting print-watcher.exe in background..." -ForegroundColor Yellow
Start-Process -FilePath $WatcherExe

Write-Host "=== Installation Completed Successfully! ===" -ForegroundColor Green
Write-Host "Virtual printers 'Print to Markdown' and 'Print to EPUB' are ready."
Write-Host "They will automatically convert printed documents and prompt you to save them."
