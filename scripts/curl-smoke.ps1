# Windows 版冒烟脚本(等价于 scripts/curl-smoke.sh)
# 依赖:后端已启动;PowerShell 7 推荐(Invoke-RestMethod 原生 JSON)
param(
    [string]$Base = 'http://localhost:6039',
    [string]$Slug = 'hello-world'
)

$ErrorActionPreference = 'Stop'

function Unwrap([object]$resp) {
    # 后端信封 { code, data, msg } -> 返回 data;否则原样返回
    if ($null -ne $resp.data) { return $resp.data }
    return $resp
}

Write-Host '==> list scenarios'
$list = Invoke-RestMethod -Uri "$Base/v1/scenarios"
(Unwrap $list).scenarios | ForEach-Object { $_.slug }

Write-Host "==> get scenario $Slug"
$detail = Invoke-RestMethod -Uri "$Base/v1/scenarios/$Slug"
(Unwrap $detail).scenario.title

Write-Host '==> start attempt'
$start = Invoke-RestMethod -Uri "$Base/v1/scenarios/$Slug/start" -Method Post -ContentType 'application/json' -Body '{}'
$startData = Unwrap $start
$startData | ConvertTo-Json -Depth 5
$attemptId = $startData.attemptId
if (-not $attemptId) { throw 'failed to get attemptId' }
Write-Host "==> attemptId: $attemptId"

Write-Host '==> get attempt'
$get = Invoke-RestMethod -Uri "$Base/v1/attempts/$attemptId"
(Unwrap $get) | ConvertTo-Json -Depth 5

Write-Host '==> check attempt'
$chk = Invoke-RestMethod -Uri "$Base/v1/attempts/$attemptId/check" -Method Post -ContentType 'application/json' -Body '{}'
(Unwrap $chk) | ConvertTo-Json -Depth 5

Write-Host '==> terminate attempt'
$term = Invoke-RestMethod -Uri "$Base/v1/attempts/$attemptId/terminate" -Method Post -ContentType 'application/json' -Body '{}'
(Unwrap $term) | ConvertTo-Json -Depth 5

Write-Host 'smoke ok'
