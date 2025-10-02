# Create placeholder icons for Chrome extension

Add-Type -AssemblyName System.Drawing

$sizes = @(16, 48, 128)
$iconDir = Join-Path $PSScriptRoot "..\icons"

# Create icons directory if it doesn't exist
if (-not (Test-Path $iconDir)) {
    New-Item -ItemType Directory -Path $iconDir | Out-Null
    Write-Host "Created icons directory: $iconDir" -ForegroundColor Green
}

foreach ($size in $sizes) {
    $bmp = New-Object System.Drawing.Bitmap($size, $size)
    $g = [System.Drawing.Graphics]::FromImage($bmp)

    # Blue background (Aktis Parser theme)
    $g.Clear([System.Drawing.Color]::FromArgb(41, 128, 185))

    # White "AP" text
    $fontSize = [Math]::Floor($size * 0.35)
    $font = New-Object System.Drawing.Font('Arial', $fontSize, [System.Drawing.FontStyle]::Bold)
    $brush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::White)

    $text = 'AP'
    $sf = New-Object System.Drawing.StringFormat
    $sf.Alignment = [System.Drawing.StringAlignment]::Center
    $sf.LineAlignment = [System.Drawing.StringAlignment]::Center

    $rect = New-Object System.Drawing.RectangleF(0, 0, $size, $size)
    $g.DrawString($text, $font, $brush, $rect, $sf)

    # Save PNG
    $iconPath = Join-Path $iconDir "icon$size.png"
    $bmp.Save($iconPath, [System.Drawing.Imaging.ImageFormat]::Png)

    Write-Host "Created $iconPath" -ForegroundColor Green

    # Cleanup
    $g.Dispose()
    $bmp.Dispose()
}

Write-Host "Icon creation complete" -ForegroundColor Cyan
