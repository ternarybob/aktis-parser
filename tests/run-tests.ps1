# Aktis Parser Test Runner
param(
    [Parameter(Mandatory=$false)]
    [string]$Type = "all",
    [Parameter(Mandatory=$false)]
    [string]$Test = $null
)

Write-Host "Test Runner for Aktis Parser" -ForegroundColor Cyan
Write-Host "=============================" -ForegroundColor Cyan

# Get script directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

# Create results directory with timestamp
$timestamp = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
$resultsDir = Join-Path $scriptDir "results"
$runDir = Join-Path $resultsDir "run_$timestamp"

if (-not (Test-Path $runDir)) {
    New-Item -ItemType Directory -Path $runDir -Force | Out-Null
}

Write-Host "Test run directory: $runDir" -ForegroundColor Cyan

$resultsFile = Join-Path $runDir "test-results.log"

# Check Go
try {
    $goVersion = go version
    Write-Host "Go found: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "Go not found. Please install Go." -ForegroundColor Red
    exit 1
}

# Build the application
Write-Host "`nBuilding application..." -ForegroundColor Cyan
$projectRoot = Split-Path -Parent $scriptDir
$buildScript = Join-Path $projectRoot "scripts\build.ps1"

Push-Location $projectRoot
try {
    & powershell -ExecutionPolicy Bypass -File $buildScript
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "Build successful" -ForegroundColor Green
} finally {
    Pop-Location
}

# Start the service after build
Write-Host "`nStarting service..." -ForegroundColor Cyan
$serverPath = Join-Path $projectRoot "bin\aktis-parser.exe"
$serverProcess = Start-Process -FilePath $serverPath -PassThru -WindowStyle Hidden
Write-Host "Service started (PID: $($serverProcess.Id))" -ForegroundColor Green
Start-Sleep -Seconds 3

# Check service is running
Write-Host "Checking if service is running..." -ForegroundColor Cyan
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8085/" -Method GET -TimeoutSec 2 -ErrorAction Stop
    Write-Host "Service is running at http://localhost:8085" -ForegroundColor Green
} catch {
    Write-Host "Warning: Service may not be responding" -ForegroundColor Yellow
}

# Determine test directories and test filter
$testDirs = @()
$testFilter = $null

if ($Type -eq "all") {
    $testDirs = @("api", "ui")
} elseif ($Type -eq "api") {
    $testDirs = @("api")
} elseif ($Type -eq "ui") {
    $testDirs = @("ui")
} elseif ($Type -eq "confluence") {
    $testDirs = @("api", "ui")
    $testFilter = "Confluence"
} else {
    Write-Host "Invalid test type. Use: all, api, ui, or confluence" -ForegroundColor Red
    exit 1
}

# Run tests
$testResults = @()

foreach ($dir in $testDirs) {
    $testPath = Join-Path $scriptDir $dir

    if (-not (Test-Path $testPath)) {
        Write-Host "Test directory not found: $testPath" -ForegroundColor Yellow
        continue
    }

    Write-Host "`nRunning $dir tests..." -ForegroundColor Cyan
    Write-Host "=============================" -ForegroundColor Cyan

    $testCmd = "go test -v -timeout 10m"

    if ($Test) {
        $testCmd += " -run $Test"
    } elseif ($testFilter) {
        $testCmd += " -run $testFilter"
    }

    # Create subdirectory for this test type
    $testRunDir = Join-Path $runDir $dir
    if (-not (Test-Path $testRunDir)) {
        New-Item -ItemType Directory -Path $testRunDir -Force | Out-Null
    }

    $testLogFile = Join-Path $testRunDir "test.log"

    Push-Location $testPath
    Write-Host "Command: $testCmd" -ForegroundColor Gray
    Write-Host "Logging to: $testLogFile" -ForegroundColor Gray
    Write-Host "Screenshots will be saved to: $testRunDir" -ForegroundColor Gray

    # Set environment variable for tests to find run directory
    $env:TEST_RUN_DIR = $testRunDir

    Invoke-Expression "$testCmd 2>&1" | Tee-Object -FilePath $testLogFile
    $exitCode = $LASTEXITCODE

    Pop-Location

    $testResults += [PSCustomObject]@{
        Type = $dir
        Success = ($exitCode -eq 0)
        ExitCode = $exitCode
        LogFile = $testLogFile
    }

    if ($exitCode -eq 0) {
        Write-Host "$dir tests PASSED" -ForegroundColor Green
    } else {
        Write-Host "$dir tests FAILED (exit code: $exitCode)" -ForegroundColor Red
        Write-Host "Log: $testLogFile" -ForegroundColor Yellow
    }
}

# Summary
Write-Host "`nTest Summary" -ForegroundColor Cyan
Write-Host "=============================" -ForegroundColor Cyan

$total = $testResults.Count
$passed = ($testResults | Where-Object { $_.Success }).Count
$failed = $total - $passed

Write-Host "Total:  $total"
Write-Host "Passed: $passed" -ForegroundColor Green
Write-Host "Failed: $failed" -ForegroundColor $(if ($failed -gt 0) { "Red" } else { "Green" })

# Write summary file
$summary = "Test Run Summary`n================`nTimestamp: $timestamp`nType: $Type`nTotal: $total`nPassed: $passed`nFailed: $failed`n`nTest Results:`n"

foreach ($result in $testResults) {
    $status = if ($result.Success) { "PASS" } else { "FAIL" }
    $summary += "[$status] $($result.Type) - Log: $($result.LogFile)`n"
}

$summary | Out-File -FilePath $resultsFile -Encoding UTF8

Write-Host "`nResults saved to: $resultsFile" -ForegroundColor Cyan

# Stop the server process
if ($serverProcess) {
    Write-Host "`nStopping server (PID: $($serverProcess.Id))..." -ForegroundColor Cyan
    Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
    Write-Host "Server stopped" -ForegroundColor Green
}

if ($failed -gt 0) {
    exit 1
} else {
    exit 0
}
