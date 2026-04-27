param(
    [string]$Output = "bin/hotnew.exe",
    [string]$GOOS = "windows",
    [string]$GOARCH = "amd64"
)

$ErrorActionPreference = "Stop"

Write-Host "[build] GOOS=$GOOS GOARCH=$GOARCH"
$env:GOOS = $GOOS
$env:GOARCH = $GOARCH

New-Item -ItemType Directory -Force (Split-Path -Parent $Output) | Out-Null
go build -o $Output ./cmd/hotnew

Write-Host "[build] output => $Output"
