param(
    [string]$ProtocVersion = "29.3",
    [string]$Target = ""
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

$apiProtoFiles = @()
if ($Target) {
    $apiProtoFiles = @($Target.Replace("\", "/"))
}
else {
    $apiProtoFiles = Get-ChildItem -Path (Join-Path $backendDir "api") -Recurse -Filter *.proto |
        ForEach-Object { $_.FullName.Substring($backendDir.Length + 1).Replace("\", "/") }
}

if (-not $apiProtoFiles) {
    throw "no api proto files found"
}

Push-Location $backendDir
try {
    & $protocExe `
        --proto_path=. `
        --proto_path=./third_party `
        --go_out=paths=source_relative:. `
        --go-http_out=paths=source_relative:. `
        --go-grpc_out=paths=source_relative:. `
        --openapi_out=fq_schema_naming=true,default_response=false:. `
        $apiProtoFiles
}
finally {
    Pop-Location
}
