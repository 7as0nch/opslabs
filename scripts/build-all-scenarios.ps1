# Windows 版场景镜像构建(等价于 scripts/build-all-scenarios.sh)
# 依赖:Docker Desktop 已启动
# 用法:
#   ./scripts/build-all-scenarios.ps1                   # 全部构建
#   ./scripts/build-all-scenarios.ps1 -Slug hello-world # 只构建指定场景
#
# 构建顺序:
#   1) scenarios-build/<name>/Dockerfile  → opslabs/<name>:v1(所有基础镜像)
#      (目录名就是 tag,新增 base 只要在 scenarios-build/ 下加目录即可)
#   2) scenarios/<slug>/Dockerfile        → opslabs/<slug>:v1(场景镜像)
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

# -------- 2. 构建场景镜像 --------
# 当前可用场景;新增场景追加到这里
$Scenarios = @(
    'hello-world'
    # 'frontend-devserver-down'   # Week 2
    # 'backend-api-500'           # Week 2
    # 'ops-nginx-upstream-fail'   # Week 2
)
if ($Slug) { $Scenarios = @($Slug) }

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
