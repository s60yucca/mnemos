# Mnemos Windows Installer
# Usage: irm https://raw.githubusercontent.com/s60yucca/mnemos/main/install.ps1 | iex
# Or with execution policy bypass:
#   powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/s60yucca/mnemos/main/install.ps1 | iex"

$ErrorActionPreference = "Stop"

$repo    = "s60yucca/mnemos"
$binName = "mnemos.exe"
$installDir = Join-Path $env:LOCALAPPDATA "mnemos"

Write-Host ""
Write-Host "  Installing Mnemos..." -ForegroundColor Cyan

# --- Detect architecture ---
$arch = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else {
    Write-Host "  ERROR: Mnemos requires a 64-bit Windows system." -ForegroundColor Red
    exit 1
}

# --- Fetch latest release version ---
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -UseBasicParsing
    $version = $release.tag_name
} catch {
    Write-Host "  ERROR: Could not fetch latest release from GitHub." -ForegroundColor Red
    Write-Host "  Check your internet connection or visit: https://github.com/$repo/releases" -ForegroundColor Yellow
    exit 1
}

$zipName = "mnemos_windows_$arch.zip"
$downloadUrl = "https://github.com/$repo/releases/download/$version/$zipName"

Write-Host "  Version : $version" -ForegroundColor Gray
Write-Host "  Platform: windows/$arch" -ForegroundColor Gray
Write-Host "  Install : $installDir" -ForegroundColor Gray
Write-Host ""

# --- Download ---
$tmpDir  = Join-Path $env:TEMP "mnemos-install-$([System.IO.Path]::GetRandomFileName())"
$zipPath = Join-Path $tmpDir $zipName

New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
    Write-Host "  Downloading $zipName..." -ForegroundColor Gray
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
} catch {
    Write-Host "  ERROR: Download failed." -ForegroundColor Red
    Write-Host "  URL: $downloadUrl" -ForegroundColor Yellow
    Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
    exit 1
}

# --- Extract ---
try {
    Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force
} catch {
    Write-Host "  ERROR: Failed to extract archive." -ForegroundColor Red
    Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
    exit 1
}

# --- Install ---
New-Item -ItemType Directory -Path $installDir -Force | Out-Null

$exeSrc = Join-Path $tmpDir $binName
if (-not (Test-Path $exeSrc)) {
    # goreleaser may nest inside a subdirectory
    $exeSrc = Get-ChildItem -Path $tmpDir -Filter $binName -Recurse | Select-Object -First 1 -ExpandProperty FullName
}
if (-not $exeSrc) {
    Write-Host "  ERROR: $binName not found in archive." -ForegroundColor Red
    Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
    exit 1
}

Copy-Item -Path $exeSrc -Destination (Join-Path $installDir $binName) -Force

# --- Cleanup ---
Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue

# --- Add to PATH (user-level, no admin required) ---
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", "User")
    Write-Host "  Added $installDir to your PATH." -ForegroundColor Gray
}

# Also update current session so mnemos is usable immediately
if ($env:Path -notlike "*$installDir*") {
    $env:Path = "$env:Path;$installDir"
}

# --- Verify ---
$installed = Join-Path $installDir $binName
if (Test-Path $installed) {
    Write-Host ""
    Write-Host "  ✅ Mnemos $version installed successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Run these commands to get started:" -ForegroundColor White
    Write-Host "    mnemos init" -ForegroundColor Cyan
    Write-Host "    mnemos serve" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  If 'mnemos' is not found, restart your terminal to pick up the PATH change." -ForegroundColor Yellow
} else {
    Write-Host "  ERROR: Installation failed — binary not found at $installed" -ForegroundColor Red
    exit 1
}
