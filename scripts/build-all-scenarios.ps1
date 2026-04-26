# Windows 版场景镜像构建(等价于 scripts/build-all-scenarios.sh)
# 依赖:Docker Desktop 已启动
# 用法:
#   ./scripts/build-all-scenarios.ps1                   # 扫 scenarios/ 下所有带 Dockerfile 的目录
#   ./scripts/build-all-scenarios.ps1 -Slug hello-world # 只构建指定场景
#
# 构建顺序:
#   1) scenarios-build/<name>/Dockerfile  → opslabs/<name>:v1(所有基础镜像)
#      (目录名就是 tag,新增 base 只要在 scenarios-build/ 下加目录即可)
#   2) scenarios/<slug>/Dockerfile        → opslabs/<slug>:v1(场景镜像,自动扫描)
#
# 2026-04-24:原先 $Scenarios 是硬编码数组,加场景要动脚本;现改为扫 scenarios/
#             自动发现 Dockerfile,新增场景只要建目录就行,脚本无需改动。
param(
    [string]$Slug = ''
)

$ErrorActionPreference = 'Stop'
$BaseDir = Split-Path -Parent (Split-Path -Parent $PSCommandPath)

# -------- 1. 扫描并构建所有 base-* 镜像 --------
$BasesDir = Join-Path $BaseDir 'scenarios-build'
if (Test-Path $BasesDir) {
    Get-ChildItem -Path $BasesDir -Directory | ForEach-Object {
        $df = Join-Path $_.FullName 'Dockerfile'
        if (-not (Test-Path $df)) { return }
        $tag = "opslabs/$($_.Name):v1"
        Write-Host "==> building $tag"
        docker build -t $tag $_.FullName
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

# -------- 2. 构建场景镜像(自动扫描 scenarios/*/Dockerfile) --------
$ScenariosDir = Join-Path $BaseDir 'scenarios'
if ($Slug) {
    $Scenarios = @($Slug)
} elseif (Test-Path $ScenariosDir) {
    $Scenarios = Get-ChildItem -Path $ScenariosDir -Directory |
        Where-Object { Test-Path (Join-Path $_.FullName 'Dockerfile') } |
        ForEach-Object { $_.Name }
} else {
    $Scenarios = @()
}

if (-not $Scenarios -or $Scenarios.Count -eq 0) {
    Write-Host '==> no scenarios with Dockerfile found under scenarios/'
    Write-Host '    (non-sandbox 模式场景如 static / web-container / wasm-linux 不需要构建镜像)'
    exit 0
}

foreach ($s in $Scenarios) {
    $df = Join-Path $BaseDir "scenarios/$s/Dockerfile"
    if (-not (Test-Path $df)) {
        Write-Host "==> skip $s (no Dockerfile)"
        continue
    }
    Write-Host "==> building opslabs/$s`:v1"
    docker build -t "opslabs/$s`:v1" "$BaseDir/scenarios/$s"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host 'all images built'
