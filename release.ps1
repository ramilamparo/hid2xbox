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

# Fetch remote tags and check for existing tag/release
if (-not $SkipPush) {
    Write-Host "Fetching remote tags..." -ForegroundColor Yellow
    git fetch --tags 2>$null

    $remoteTag = git ls-remote --tags origin $tag 2>$null
    if ($remoteTag) {
        Write-Warning "Tag $tag already exists on remote."
        $choice = Read-Host "Delete and recreate? (y/n)"
        if ($choice -ne "y") { Write-Host "Aborted."; exit 0 }
        git push origin :refs/tags/$tag 2>$null
        git tag -d $tag 2>$null
    }

    $localTag = git tag -l $tag 2>$null
    if ($localTag) {
        Write-Warning "Tag $tag exists locally."
        git tag -d $tag
    }

    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    gh release view $tag 2>$null
    $exists = ($LASTEXITCODE -eq 0)
    $ErrorActionPreference = $prev
    if ($exists) {
        Write-Warning "Release $tag already exists on GitHub."
        $choice = Read-Host "Delete and recreate? (y/n)"
        if ($choice -ne "y") { Write-Host "Aborted."; exit 0 }
        gh release delete $tag --yes
        git push origin :refs/tags/$tag 2>$null
        git tag -d $tag 2>$null
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
