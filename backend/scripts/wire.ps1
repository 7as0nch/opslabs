param()

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$backendDir = Split-Path -Parent $scriptDir
$gopath = (& go env GOPATH).Trim()
$goBin = Join-Path $gopath "bin"
$env:PATH = "$goBin;$env:PATH"

Push-Location (Join-Path $backendDir "cmd\backend")
try {
    & wire
}
finally {
    Pop-Location
}
