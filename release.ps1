param(
    [string]$Version = "",
    [switch]$SkipPush
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

# Determine version
if (-not $Version) {
    $ts = Get-Date -Format "yyyyMMdd-HHmmss"
    $Version = "dev-$ts"
}
$tag = "v$Version"

Write-Host "=== hid2xbox release $tag ===" -ForegroundColor Cyan

# Check for existing tag
$existing = git tag -l $tag 2>$null
if ($existing) {
    Write-Warning "Tag $tag already exists locally."
    if (-not $SkipPush) {
        $choice = Read-Host "Delete and recreate? (y/n)"
        if ($choice -ne "y") { Write-Host "Aborted."; exit 0 }
        git tag -d $tag
    }
}

# Check for existing GitHub release
if (-not $SkipPush) {
    $releaseExists = gh release view $tag 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Warning "Release $tag already exists on GitHub."
        $choice = Read-Host "Delete and recreate? (y/n)"
        if ($choice -ne "y") { Write-Host "Aborted."; exit 0 }
        gh release delete $tag --yes
        git push origin :refs/tags/$tag 2>$null
    }
}

# Build
Write-Host "Building..." -ForegroundColor Yellow
go build -o hid2xbox.exe -ldflags="-s -w" .
if (-not (Test-Path hid2xbox.exe)) {
    Write-Error "Build failed"
    exit 1
}
Write-Host "  hid2xbox.exe ($((Get-Item hid2xbox.exe).Length) bytes)"

# Package
$releaseDir = "hid2xbox-release"
New-Item -ItemType Directory -Path $releaseDir -Force | Out-Null
Copy-Item ViGEmClient.dll -Destination "$releaseDir/" -Force
Copy-Item hid2xbox.exe -Destination "$releaseDir/" -Force

$zipName = "hid2xbox-$Version.zip"
Write-Host "Packaging $zipName..." -ForegroundColor Yellow
Compress-Archive -Path "$releaseDir/*" -DestinationPath $zipName -Force
Write-Host "  $zipName created"

Remove-Item $releaseDir -Recurse -Force -ErrorAction SilentlyContinue

# Create GitHub release
if (-not $SkipPush) {
    Write-Host "Creating GitHub release..." -ForegroundColor Yellow
    git tag $tag
    git push origin $tag
    gh release create $tag $zipName hid2xbox.exe ViGEmClient.dll `
        --title "hid2xbox $tag" `
        --generate-notes
    Write-Host "  Release: https://github.com/ramilamparo/hid2xbox/releases/tag/$tag"
} else {
    Write-Host "Skipping GitHub push (--SkipPush)."
    Write-Host "  Artifacts: $zipName, hid2xbox.exe"
}

Write-Host "Done." -ForegroundColor Green
