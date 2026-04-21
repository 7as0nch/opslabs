# opslabs Windows 任务跑法(等价于 POSIX 的 Makefile)
# 用法:
#   ./make.ps1 dev-backend
#   ./make.ps1 dev-frontend
#   ./make.ps1 dev                 # 前后端并行,各自新开一个 PowerShell 窗口
#   ./make.ps1 install-frontend
#   ./make.ps1 gen
#   ./make.ps1 scenarios
#   ./make.ps1 scenarios-test
#   ./make.ps1 smoke
param(
    [Parameter(Position = 0)]
    [string]$Target = 'help'
)

$ErrorActionPreference = 'Stop'
$RootDir = Split-Path -Parent $PSCommandPath

function Invoke-Backend {
    Push-Location "$RootDir/backend"
    try {
        go run ./cmd/backend -conf configs/config.yaml
    } finally {
        Pop-Location
    }
}

function Invoke-Frontend {
    Push-Location "$RootDir/frontend"
    try {
        npm run dev
    } finally {
        Pop-Location
    }
}

function Invoke-InstallFrontend {
    Push-Location "$RootDir/frontend"
    try {
        npm install
    } finally {
        Pop-Location
    }
}

function Invoke-Dev {
    # 各自新开一个 PowerShell 窗口,互不干扰,关窗口即停
    Write-Host '==> starting backend in new window'
    Start-Process pwsh -ArgumentList '-NoExit', '-Command', "Set-Location '$RootDir/backend'; go run ./cmd/backend -conf configs/config.yaml"
    Start-Sleep -Seconds 1
    Write-Host '==> starting frontend in new window'
    Start-Process pwsh -ArgumentList '-NoExit', '-Command', "Set-Location '$RootDir/frontend'; npm run dev"
    Write-Host 'Both running in separate PowerShell windows.'
}

function Invoke-Gen {
    pwsh -NoProfile -File "$RootDir/backend/scripts/gen-api.ps1"
    pwsh -NoProfile -File "$RootDir/backend/scripts/wire.ps1"
}

function Invoke-Scenarios {
    pwsh -NoProfile -File "$RootDir/scripts/build-all-scenarios.ps1"
}

function Invoke-ScenariosTest {
    param([string]$Slug = 'hello-world')
    pwsh -NoProfile -File "$RootDir/scripts/test-scenario.ps1" -Slug $Slug
}

function Invoke-Smoke {
    pwsh -NoProfile -File "$RootDir/scripts/curl-smoke.ps1"
}

function Show-Help {
    Write-Host 'opslabs make.ps1 targets:'
    Write-Host '  dev-backend        启动后端 (默认 mock runtime)'
    Write-Host '  dev-frontend       启动前端 (http://localhost:5173)'
    Write-Host '  dev                前后端分别起新窗口'
    Write-Host '  install-frontend   首次 npm install'
    Write-Host '  gen                重新生成 proto / wire'
    Write-Host '  scenarios          docker build 全部场景镜像'
    Write-Host '  scenarios-test     对 hello-world 跑 regression+solution+check'
    Write-Host '  smoke              curl 打一遍后端 REST API'
}

switch ($Target) {
    'dev-backend'        { Invoke-Backend }
    'dev-frontend'       { Invoke-Frontend }
    'dev'                { Invoke-Dev }
    'install-frontend'   { Invoke-InstallFrontend }
    'gen'                { Invoke-Gen }
    'scenarios'          { Invoke-Scenarios }
    'scenarios-test'     { Invoke-ScenariosTest }
    'smoke'              { Invoke-Smoke }
    default              { Show-Help }
}
