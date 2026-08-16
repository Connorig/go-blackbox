# 版本同步校验:比对 banner.go 的 Version 常量与目标版本/tag。
# 用法:
#   .\scripts\check-version.ps1 -ExpectedVersion 1.43.0   # 打 tag 前(推荐)
#   .\scripts\check-version.ps1                           # 比对最新 tag(tag 已打后)
param(
    [string]$ExpectedVersion = ""
)
$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot)

$bannerPath = Join-Path (Get-Location) "banner.go"
if (-not (Test-Path $bannerPath)) {
    Write-Error "banner.go not found"
    exit 1
}
$content = [System.IO.File]::ReadAllText($bannerPath)
if ($content -notmatch 'const Version = "([^"]+)"') {
    Write-Error "cannot find Version constant in banner.go"
    exit 1
}
$constVersion = $Matches[1]

$target = $ExpectedVersion.Trim()
if ($target -eq "") {
    # 未传参:比对最新 tag
    $latestTag = (git describe --tags --abbrev=0 2>$null).Trim()
    if (-not $latestTag) {
        Write-Host "no tags yet, skip check (const=$constVersion)"
        exit 0
    }
    $target = $latestTag.TrimStart("v")
}

if ($target -ne $constVersion) {
    Write-Error "VERSION MISMATCH: banner.go const=$constVersion but expected=$target. Update banner.go Version first."
    exit 1
}
Write-Host "OK: banner version $constVersion matches $target"
