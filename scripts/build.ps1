# -----------------------------------------------------------------------
# Build Script for Aktis Parser
# -----------------------------------------------------------------------

param (
    [string]$Environment = "dev",
    [string]$Version = "",
    [switch]$Clean,
    [switch]$Test,
    [switch]$Verbose,
    [switch]$Release,
    [switch]$Run
)

<#
.SYNOPSIS
    Build script for Aktis Parser

.DESCRIPTION
    This script builds the Aktis Parser service for local development and testing.
    Outputs the executable to the project's bin directory.

.PARAMETER Environment
    Target environment for build (dev, staging, prod)

.PARAMETER Version
    Version to embed in the binary (defaults to .version file or git commit hash)

.PARAMETER Clean
    Clean build artifacts before building

.PARAMETER Test
    Run tests before building

.PARAMETER Verbose
    Enable verbose output

.PARAMETER Release
    Build optimized release binary

.PARAMETER Run
    Run the application in a new terminal after successful build

.EXAMPLE
    .\build.ps1
    Build aktis parser for development

.EXAMPLE
    .\build.ps1 -Release
    Build optimized release version

.EXAMPLE
    .\build.ps1 -Environment prod -Version "1.0.0"
    Build for production with specific version

.EXAMPLE
    .\build.ps1 -Run
    Build and run the application in a new terminal
#>

# Error handling
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

# Build configuration
$gitCommit = ""

try {
    $gitCommit = git rev-parse --short HEAD 2>$null
    if (-not $gitCommit) { $gitCommit = "unknown" }
}
catch {
    $gitCommit = "unknown"
}

Write-Host "Aktis Parser Build Script" -ForegroundColor Cyan
Write-Host "=========================" -ForegroundColor Cyan

# Setup paths
$scriptDir = $PSScriptRoot
$projectRoot = Split-Path -Parent $scriptDir
$versionFilePath = Join-Path -Path $projectRoot -ChildPath ".version"
$binDir = Join-Path -Path $projectRoot -ChildPath "bin"
$outputPath = Join-Path -Path $binDir -ChildPath "aktis-parser.exe"

Write-Host "Project Root: $projectRoot" -ForegroundColor Gray
Write-Host "Environment: $Environment" -ForegroundColor Gray
Write-Host "Git Commit: $gitCommit" -ForegroundColor Gray

# Handle version file creation and maintenance
$buildTimestamp = Get-Date -Format "MM-dd-HH-mm-ss"

if (-not (Test-Path $versionFilePath)) {
    # Create .version file if it doesn't exist
    $versionContent = @"
server_version: 0.1.0
server_build: $buildTimestamp
extension_version: 0.1.0
"@
    Set-Content -Path $versionFilePath -Value $versionContent
    Write-Host "Created .version file with version 0.1.0" -ForegroundColor Green
} else {
    # Read current version and increment patch versions
    $versionLines = Get-Content $versionFilePath
    $currentServerVersion = ""
    $currentExtVersion = ""
    $updatedLines = @()

    foreach ($line in $versionLines) {
        if ($line -match '^server_version:\s*(.+)$') {
            $currentServerVersion = $matches[1].Trim()

            # Parse version (format: major.minor.patch)
            if ($currentServerVersion -match '^(\d+)\.(\d+)\.(\d+)$') {
                $major = [int]$matches[1]
                $minor = [int]$matches[2]
                $patch = [int]$matches[3]

                # Increment patch version
                $patch++
                $newVersion = "$major.$minor.$patch"

                $updatedLines += "server_version: $newVersion"
                Write-Host "Incremented server version: $currentServerVersion -> $newVersion" -ForegroundColor Green
            } else {
                $updatedLines += $line
                Write-Host "Server version format not recognized, keeping: $currentServerVersion" -ForegroundColor Yellow
            }
        } elseif ($line -match '^server_build:\s*') {
            $updatedLines += "server_build: $buildTimestamp"
        } elseif ($line -match '^extension_version:\s*(.+)$') {
            $currentExtVersion = $matches[1].Trim()

            # Parse version (format: major.minor.patch)
            if ($currentExtVersion -match '^(\d+)\.(\d+)\.(\d+)$') {
                $major = [int]$matches[1]
                $minor = [int]$matches[2]
                $patch = [int]$matches[3]

                # Increment patch version
                $patch++
                $newExtVersion = "$major.$minor.$patch"

                $updatedLines += "extension_version: $newExtVersion"
                Write-Host "Incremented extension version: $currentExtVersion -> $newExtVersion" -ForegroundColor Green
            } else {
                $updatedLines += $line
                Write-Host "Extension version format not recognized, keeping: $currentExtVersion" -ForegroundColor Yellow
            }
        } else {
            $updatedLines += $line
        }
    }

    Set-Content -Path $versionFilePath -Value $updatedLines
    Write-Host "Updated build timestamp to: $buildTimestamp" -ForegroundColor Green
}

# Read version information from .version file
$versionInfo = @{}
$versionLines = Get-Content $versionFilePath
foreach ($line in $versionLines) {
    if ($line -match '^server_version:\s*(.+)$') {
        $versionInfo.ServerVersion = $matches[1].Trim()
    }
    if ($line -match '^server_build:\s*(.+)$') {
        $versionInfo.ServerBuild = $matches[1].Trim()
    }
    if ($line -match '^extension_version:\s*(.+)$') {
        $versionInfo.ExtensionVersion = $matches[1].Trim()
    }
}

Write-Host "Using server version: $($versionInfo.ServerVersion), build: $($versionInfo.ServerBuild)" -ForegroundColor Cyan
Write-Host "Using extension version: $($versionInfo.ExtensionVersion)" -ForegroundColor Cyan

# Clean build artifacts if requested
if ($Clean) {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Yellow
    if (Test-Path $binDir) {
        Remove-Item -Path $binDir -Recurse -Force
    }
    if (Test-Path "go.sum") {
        Remove-Item -Path "go.sum" -Force
    }
}

# Create bin directory
if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir | Out-Null
}

# Run tests if requested
if ($Test) {
    Write-Host "Running tests..." -ForegroundColor Yellow
    go test ./... -v
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Tests failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "Tests passed!" -ForegroundColor Green
}

# Stop executing process if it's running
try {
    $processName = "aktis-parser"
    $process = Get-Process -Name $processName -ErrorAction SilentlyContinue

    if ($process) {
        Write-Host "Stopping existing Aktis Parser process..." -ForegroundColor Yellow
        Stop-Process -Name $processName -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 1  # Give process time to fully terminate
        Write-Host "Process stopped successfully" -ForegroundColor Green
    } else {
        Write-Host "No Aktis Parser process found running" -ForegroundColor Gray
    }
}
catch {
    Write-Warning "Could not stop Aktis Parser process: $($_.Exception.Message)"
}

# Tidy and download dependencies
Write-Host "Tidying dependencies..." -ForegroundColor Yellow
go mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to tidy dependencies!" -ForegroundColor Red
    exit 1
}

Write-Host "Downloading dependencies..." -ForegroundColor Yellow
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to download dependencies!" -ForegroundColor Red
    exit 1
}

# Build flags
$module = "github.com/bobmc/aktis-parser/internal/common"
$buildFlags = @(
    "-X", "$module.Version=$($versionInfo.ServerVersion)",
    "-X", "$module.Build=$($versionInfo.ServerBuild)",
    "-X", "$module.GitCommit=$gitCommit"
)

if ($Release) {
    $buildFlags += @("-w", "-s")  # Strip debug info and symbol table
}

$ldflags = $buildFlags -join " "

# Build command
Write-Host "Building aktis-parser..." -ForegroundColor Yellow

$env:CGO_ENABLED = "0"
if ($Release) {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
}

$buildArgs = @(
    "build"
    "-ldflags=$ldflags"
    "-o", $outputPath
    ".\cmd\aktis-parser\main.go"
)

# Change to project root for build
Push-Location $projectRoot

if ($Verbose) {
    $buildArgs += "-v"
}

Write-Host "Build command: go $($buildArgs -join ' ')" -ForegroundColor Gray

& go @buildArgs

# Return to original directory
Pop-Location

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}

# Copy configuration file to bin directory
$configSourcePath = Join-Path -Path $projectRoot -ChildPath "deployments\aktis-parser.toml"
$configDestPath = Join-Path -Path $binDir -ChildPath "aktis-parser.toml"

if (Test-Path $configSourcePath) {
    if (-not (Test-Path $configDestPath)) {
        Copy-Item -Path $configSourcePath -Destination $configDestPath
        Write-Host "Copied configuration: deployments/aktis-parser.toml -> bin/" -ForegroundColor Green
    } else {
        Write-Host "Using existing bin/aktis-parser.toml (preserving customizations)" -ForegroundColor Cyan
    }
}

# Build and deploy Chrome Extension
Write-Host "`nBuilding Chrome Extension..." -ForegroundColor Yellow

$extensionSourcePath = Join-Path -Path $projectRoot -ChildPath "cmd\aktis-chrome-extension"
$extensionDestPath = Join-Path -Path $binDir -ChildPath "aktis-chrome-extension"

# Check if extension source exists
if (Test-Path $extensionSourcePath) {
    # Update manifest.json with extension version
    $manifestPath = Join-Path -Path $extensionSourcePath -ChildPath "manifest.json"
    if (Test-Path $manifestPath) {
        $manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json
        $manifest.version = $versionInfo.ExtensionVersion
        $manifest | ConvertTo-Json -Depth 10 | Set-Content $manifestPath
        Write-Host "Updated manifest.json to version $($versionInfo.ExtensionVersion)" -ForegroundColor Green
    }

    # Create extension directory in bin
    if (Test-Path $extensionDestPath) {
        Remove-Item -Path $extensionDestPath -Recurse -Force
    }
    New-Item -ItemType Directory -Path $extensionDestPath | Out-Null

    # Copy extension files (exclude create-icons.ps1 as it's a dev tool)
    $extensionFiles = @(
        "manifest.json",
        "background.js",
        "content.js",
        "popup.html",
        "popup.js",
        "sidepanel.html",
        "sidepanel.js",
        "README.md"
    )

    foreach ($file in $extensionFiles) {
        $sourcePath = Join-Path -Path $extensionSourcePath -ChildPath $file
        if (Test-Path $sourcePath) {
            Copy-Item -Path $sourcePath -Destination $extensionDestPath
        } else {
            Write-Warning "Extension file not found: $file"
        }
    }

    # Copy icons directory
    $iconsSourcePath = Join-Path -Path $extensionSourcePath -ChildPath "icons"
    $iconsDestPath = Join-Path -Path $extensionDestPath -ChildPath "icons"

    if (Test-Path $iconsSourcePath) {
        Copy-Item -Path $iconsSourcePath -Destination $iconsDestPath -Recurse
        Write-Host "Copied extension icons: icons/ -> bin/aktis-chrome-extension/icons/" -ForegroundColor Green
    } else {
        # Icons don't exist, create them
        Write-Host "Icons not found, creating placeholder icons..." -ForegroundColor Yellow

        New-Item -ItemType Directory -Path $iconsDestPath -Force | Out-Null

        $createIconScript = Join-Path -Path $scriptDir -ChildPath "create-icons.ps1"
        if (Test-Path $createIconScript) {
            & powershell.exe -ExecutionPolicy Bypass -File $createIconScript
            # Copy newly created icons
            if (Test-Path (Join-Path $projectRoot "icons")) {
                Copy-Item -Path (Join-Path $projectRoot "icons\*") -Destination $iconsDestPath -Recurse
                Write-Host "Created and copied extension icons" -ForegroundColor Green
            }
        } else {
            Write-Warning "Icon creation script not found, extension may not have icons"
        }
    }

    Write-Host "Deployed Chrome Extension: bin/aktis-chrome-extension/" -ForegroundColor Green
} else {
    Write-Warning "Chrome extension source not found at: $extensionSourcePath"
}

# Verify executable was created
if (-not (Test-Path $outputPath)) {
    Write-Error "Build completed but executable not found: $outputPath"
    exit 1
}

# Get file info for binary
$fileInfo = Get-Item $outputPath
$fileSizeMB = [math]::Round($fileInfo.Length / 1MB, 2)

Write-Host "`n==== Build Summary ====" -ForegroundColor Cyan
Write-Host "Status: SUCCESS" -ForegroundColor Green
Write-Host "Environment: $Environment" -ForegroundColor Green
Write-Host "Server Version: $($versionInfo.ServerVersion)" -ForegroundColor Green
Write-Host "Server Build: $($versionInfo.ServerBuild)" -ForegroundColor Green
Write-Host "Extension Version: $($versionInfo.ExtensionVersion)" -ForegroundColor Green
Write-Host "Server Output: $outputPath ($fileSizeMB MB)" -ForegroundColor Green
Write-Host "Extension Output: $extensionDestPath" -ForegroundColor Green
Write-Host "Build Time: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Green

if ($Test) {
    Write-Host "Tests: EXECUTED" -ForegroundColor Green
}

if ($Clean) {
    Write-Host "Clean: EXECUTED" -ForegroundColor Green
}

Write-Host "`nBuild completed successfully!" -ForegroundColor Green
Write-Host "Server: $outputPath" -ForegroundColor Cyan
Write-Host "Extension: $extensionDestPath" -ForegroundColor Cyan

# Run application if -Run flag is set
if ($Run) {
    Write-Host "`n==== Starting Application ====" -ForegroundColor Yellow

    # Start in a new terminal window (cmd) that closes when app exits
    $startCommand = "cd /d `"$projectRoot`" && `"$outputPath`""

    Start-Process cmd -ArgumentList "/c", $startCommand

    Write-Host "Application started in new terminal window" -ForegroundColor Green
    Write-Host "Working Directory: $projectRoot" -ForegroundColor Cyan
    Write-Host "Executable: $outputPath" -ForegroundColor Cyan
    Write-Host "(Terminal will automatically close when application stops)" -ForegroundColor Gray
} else {
    Write-Host "`nTo run:" -ForegroundColor Yellow
    Write-Host "./bin/aktis-parser.exe" -ForegroundColor White
    Write-Host "`nTo load extension in Chrome:" -ForegroundColor Yellow
    Write-Host "1. Open chrome://extensions/" -ForegroundColor White
    Write-Host "2. Enable 'Developer mode'" -ForegroundColor White
    Write-Host "3. Click 'Load unpacked' and select: $extensionDestPath" -ForegroundColor White
}
