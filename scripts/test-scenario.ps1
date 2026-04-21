# Windows 版场景回归测试(等价于 scripts/test-scenario.sh)
# 流程:regression(验故障) -> solution(参考解) -> check(应 OK)
# 用法:
#   ./scripts/test-scenario.ps1 -Slug hello-world
param(
    [Parameter(Mandatory = $true)]
    [string]$Slug
)

$ErrorActionPreference = 'Stop'
$BaseDir = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$ScenarioDir = Join-Path $BaseDir "scenarios/$Slug"
$Image = "opslabs/$Slug`:v1"
$Name = "opslabs-test-$Slug-$PID"

if (-not (Test-Path $ScenarioDir)) {
    Write-Host "scenario dir not found: $ScenarioDir"
    exit 1
}

function Cleanup {
    try { docker rm -f $script:Name *> $null } catch {}
}
$script:Name = $Name
try {
    Write-Host "==> starting container $Name"
    docker run -d --name $Name --privileged $Image | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'docker run failed' }
    Start-Sleep -Seconds 3

    Write-Host '[1/3] regression check (故障应已预埋)...'
    docker cp "$ScenarioDir/tests/regression.sh" "${Name}:/tmp/regression.sh"
    docker exec $Name bash /tmp/regression.sh
    if ($LASTEXITCODE -ne 0) { throw 'regression failed' }

    Write-Host '[2/3] running solution...'
    docker cp "$ScenarioDir/tests/solution.sh" "${Name}:/tmp/solution.sh"
    docker exec $Name bash /tmp/solution.sh
    if ($LASTEXITCODE -ne 0) { throw 'solution failed' }

    Write-Host '[3/3] check after solution (应返回 OK)...'
    $result = docker exec $Name bash /opt/opslabs/check.sh | Select-Object -First 1
    if ($result -ne 'OK') {
        throw "check.sh after solution = '$result'"
    }
    Write-Host "all pass: $Slug"
} finally {
    Cleanup
}
