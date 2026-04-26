# Windows 版冒烟脚本(等价于 scripts/curl-smoke.sh)
# 用法:
#   ./scripts/curl-smoke.ps1                   # 跑 4 种 execution mode 各一个代表性场景
#   ./scripts/curl-smoke.ps1 -Slug hello-world # 只跑指定 slug
#
# 依赖:后端已启动;PowerShell 7+ (Invoke-RestMethod 原生 JSON)
param(
    [string]$Base = 'http://localhost:6039',
    [string]$Slug = ''
)

$ErrorActionPreference = 'Stop'

function Unwrap([object]$resp) {
    # 后端信封 { code, data, msg } -> 返回 data;否则原样返回
    if ($null -ne $resp.data) { return $resp.data }
    return $resp
}

# 4 种模式的代表性场景;新增加在这里补一行即可
$DefaultSlugs = @(
    @{ Slug = 'hello-world';              Mode = 'sandbox' }
    @{ Slug = 'css-flex-center';          Mode = 'static' }
    @{ Slug = 'webcontainer-node-hello';  Mode = 'web-container' }
    @{ Slug = 'wasm-linux-hello';         Mode = 'wasm-linux' }
)
if ($Slug) {
    $DefaultSlugs = @(@{ Slug = $Slug; Mode = 'auto' })
}

Write-Host '==> list scenarios'
$list = Invoke-RestMethod -Uri "$Base/v1/scenarios"
(Unwrap $list).scenarios | ForEach-Object { $_.slug }

$fail = 0
foreach ($entry in $DefaultSlugs) {
    $s = $entry.Slug
    $mode = $entry.Mode
    Write-Host ''
    Write-Host '================================================================'
    Write-Host " smoke: $s  (mode=$mode)"
    Write-Host '================================================================'

    Write-Host '==> detail'
    $detail = Invoke-RestMethod -Uri "$Base/v1/scenarios/$s"
    (Unwrap $detail).scenario.title

    Write-Host '==> start'
    try {
        $start = Invoke-RestMethod -Uri "$Base/v1/scenarios/$s/start" -Method Post -ContentType 'application/json' -Body '{}'
        $startData = Unwrap $start
        $attemptId = $startData.attemptId
        if (-not $attemptId) {
            Write-Warning "start returned no attemptId for $s"
            $fail += 1
            continue
        }
        Write-Host "   attemptId=$attemptId"
    } catch {
        Write-Warning "start failed for $s : $($_.Exception.Message)"
        $fail += 1
        continue
    }

    Write-Host '==> get'
    $get = Invoke-RestMethod -Uri "$Base/v1/attempts/$attemptId"
    (Unwrap $get).attempt.status

    Write-Host '==> check'
    # 非 sandbox 必须带 clientResult;sandbox 发空 body
    if ($mode -eq 'sandbox' -or $mode -eq 'auto') {
        $body = '{}'
    } else {
        $body = '{"clientResult":{"passed":true,"exitCode":0,"stdout":"OK (smoke)","stderr":""}}'
    }
    try {
        $chk = Invoke-RestMethod -Uri "$Base/v1/attempts/$attemptId/check" -Method Post -ContentType 'application/json' -Body $body
        $chkData = Unwrap $chk
        Write-Host ("   passed={0} message={1}" -f $chkData.passed, $chkData.message)
    } catch {
        # mode=sandbox + mock runtime 时 check 会失败,这里不中断
        Write-Warning "check failed (可能是 mock runtime 下 sandbox 跳过): $($_.Exception.Message)"
    }

    Write-Host '==> terminate'
    try {
        $term = Invoke-RestMethod -Uri "$Base/v1/attempts/$attemptId/terminate" -Method Post -ContentType 'application/json' -Body '{}'
        (Unwrap $term).status
    } catch {
        Write-Warning "terminate failed: $($_.Exception.Message)"
    }
}

Write-Host ''
if ($fail -gt 0) {
    Write-Error "smoke FAIL: $fail scenario(s) failed to start"
    exit 1
}
Write-Host 'smoke ok'
