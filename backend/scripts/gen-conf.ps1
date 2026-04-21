param(
    [string]$ProtocVersion = "29.3"
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$backendDir = Split-Path -Parent $scriptDir
$protocExe = Join-Path $backendDir ".tools\protoc\$ProtocVersion\bin\protoc.exe"

if (-not (Test-Path $protocExe)) {
    throw "protoc not found. run scripts/bootstrap-tools.ps1 first"
}

$gopath = (& go env GOPATH).Trim()
$goBin = Join-Path $gopath "bin"
$env:PATH = "$goBin;$(Split-Path -Parent $protocExe);$env:PATH"

Push-Location $backendDir
try {
    & $protocExe `
        --proto_path=./internal/conf `
        --proto_path=./third_party `
        --go_out=paths=source_relative:./internal/conf `
        conf.proto
}
finally {
    Pop-Location
}
