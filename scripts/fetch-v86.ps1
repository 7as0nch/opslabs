# ------------------------------------------------------------------
# opslabs / scripts/fetch-v86.ps1 (Windows PowerShell 版本)
# ------------------------------------------------------------------
# 功能与 fetch-v86.sh 完全一致:把 v86 + BusyBox 的 5 个二进制拉到
# backend/internal/scenario/bundles/wasm-linux-hello/vendor/。
#
# 用法:
#   ./scripts/fetch-v86.ps1
#   # 或自定义镜像:
#   $env:V86_MIRROR = 'https://mymirror/v86'; ./scripts/fetch-v86.ps1
# ------------------------------------------------------------------

$ErrorActionPreference = 'Stop'

$mirror = if ($env:V86_MIRROR) { $env:V86_MIRROR } else { 'https://copy.sh/v86' }

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot  = Resolve-Path (Join-Path $scriptDir '..')
$vendorDir = Join-Path $repoRoot 'backend\internal\scenario\bundles\wasm-linux-hello\vendor'

New-Item -ItemType Directory -Force -Path $vendorDir | Out-Null

Write-Host "[fetch-v86] mirror = $mirror"
Write-Host "[fetch-v86] target = $vendorDir"
Write-Host ""

# 落地时保留 mirror 的目录结构(build/ bios/ images/),
# 让前端 index.html 的 bootLib/bootEmulator 对 ./vendor 和 https://copy.sh/v86
# 使用完全相同的 URL 后缀,不需要做路径切换。
$files = @(
  'build/libv86.js',
  'build/v86.wasm',
  'bios/seabios.bin',
  'bios/vgabios.bin',
  'images/linux.iso'
)

foreach ($rel in $files) {
  $url = "$mirror/$rel"
  $out = Join-Path $vendorDir $rel
  $outDir = Split-Path -Parent $out
  New-Item -ItemType Directory -Force -Path $outDir | Out-Null
  Write-Host "[fetch-v86] GET  $url"
  # Invoke-WebRequest 非 200 会抛异常,因为 $ErrorActionPreference='Stop'
  Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $out
  $size = (Get-Item $out).Length
  Write-Host "[fetch-v86] ok   $rel ($size bytes)"
}

Write-Host ""
Write-Host '[fetch-v86] 完成。接下来:'
Write-Host '  cd backend; go build ./...'
Write-Host '  # 让 //go:embed all:wasm-linux-hello 把 vendor/ 一起打进二进制'
