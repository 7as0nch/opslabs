param(
    [string]$ProtocVersion = "29.3"
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$backendDir = Split-Path -Parent $scriptDir
$toolsDir = Join-Path $backendDir ".tools"
$protocRoot = Join-Path $toolsDir "protoc"
$protocDir = Join-Path $protocRoot $ProtocVersion
$protocBinDir = Join-Path $protocDir "bin"
$protocExe = Join-Path $protocBinDir "protoc.exe"

New-Item -ItemType Directory -Force -Path $toolsDir | Out-Null
New-Item -ItemType Directory -Force -Path $protocRoot | Out-Null

$gopath = (& go env GOPATH).Trim()
if (-not $gopath) {
    throw "failed to resolve GOPATH"
}
$goBin = Join-Path $gopath "bin"
New-Item -ItemType Directory -Force -Path $goBin | Out-Null

$env:PATH = "$goBin;$protocBinDir;$env:PATH"

$goTools = @(
    "google.golang.org/protobuf/cmd/protoc-gen-go@latest",
    "google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest",
    "github.com/go-kratos/kratos/cmd/kratos/v2@latest",
    "github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest",
    "github.com/google/gnostic/cmd/protoc-gen-openapi@latest",
    "github.com/google/wire/cmd/wire@latest",
    "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest"
)

foreach ($tool in $goTools) {
    Write-Host "Installing $tool"
    & go install $tool
}

if (-not (Test-Path $protocExe)) {
    $zipName = "protoc-$ProtocVersion-win64.zip"
    $downloadUrl = "https://github.com/protocolbuffers/protobuf/releases/download/v$ProtocVersion/$zipName"
    $zipPath = Join-Path $protocRoot $zipName

    Write-Host "Downloading protoc $ProtocVersion"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath

    if (Test-Path $protocDir) {
        Remove-Item -Recurse -Force $protocDir
    }
    New-Item -ItemType Directory -Force -Path $protocDir | Out-Null

    Write-Host "Extracting protoc"
    Expand-Archive -Path $zipPath -DestinationPath $protocDir -Force
}

Write-Host ""
Write-Host "Tool bootstrap complete."
Write-Host "GOBIN:   $goBin"
Write-Host "PROTOC:  $protocExe"
