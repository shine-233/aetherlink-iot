# AetherLink backend coverage runner.
#
# This script keeps the raw Go profile and also writes a filtered profile that
# excludes only source files carrying Go's generated-code header. Passing the
# minimum percentage gate is not proof of API, E2E, or business-flow closure.
param(
    [ValidateSet("all", "api", "router", "mqtt", "service", "dal", "utils")]
    [string]$Target = "all",
    [switch]$Html,
    [switch]$Check,
    [double]$Threshold = 30.0,
    [ValidateRange(1, 16)]
    [int]$PackageParallelism = 1
)

$ErrorActionPreference = "Stop"
$CoverageDir = Join-Path $PSScriptRoot "coverage"

if (-not (Test-Path -LiteralPath $CoverageDir)) {
    New-Item -ItemType Directory -Path $CoverageDir | Out-Null
}

function Get-CoveragePercent {
    param([Parameter(Mandatory = $true)][string]$Profile)

    $output = & go tool cover "-func=$Profile" 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "go tool cover failed for $Profile`n$($output -join [Environment]::NewLine)"
    }
    $lastLine = $output | Select-Object -Last 1
    if ($lastLine -notmatch '(\d+(?:\.\d+)?)%') {
        throw "Could not parse total coverage from: $lastLine"
    }
    return [double]$Matches[1]
}

function Get-ModulePath {
    $moduleLine = Get-Content -LiteralPath (Join-Path $PSScriptRoot "go.mod") |
        Where-Object { $_ -match '^module\s+' } |
        Select-Object -First 1
    if ($moduleLine -notmatch '^module\s+(.+)$') {
        throw "Could not read the module path from go.mod"
    }
    return $Matches[1].Trim()
}

function Test-GeneratedSource {
    param(
        [Parameter(Mandatory = $true)][string]$CoverageSource,
        [Parameter(Mandatory = $true)][string]$ModulePath
    )

    $prefix = "$ModulePath/"
    if (-not $CoverageSource.StartsWith($prefix, [System.StringComparison]::Ordinal)) {
        return $false
    }

    $relativePath = $CoverageSource.Substring($prefix.Length).Replace('/', [IO.Path]::DirectorySeparatorChar)
    $sourcePath = Join-Path $PSScriptRoot $relativePath
    if (-not (Test-Path -LiteralPath $sourcePath)) {
        return $false
    }

    $header = Get-Content -LiteralPath $sourcePath -TotalCount 8 -ErrorAction Stop
    return [bool]($header -match '^// Code generated .* DO NOT EDIT\.$')
}

function Write-FilteredCoverageProfile {
    param(
        [Parameter(Mandatory = $true)][string]$RawProfile,
        [Parameter(Mandatory = $true)][string]$FilteredProfile,
        [Parameter(Mandatory = $true)][string]$ModulePath
    )

    $lines = Get-Content -LiteralPath $RawProfile
    if ($lines.Count -eq 0 -or $lines[0] -notmatch '^mode:') {
        throw "Invalid Go coverage profile: $RawProfile"
    }

    $kept = [System.Collections.Generic.List[string]]::new()
    $excluded = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
    [void]$kept.Add($lines[0])

    foreach ($line in $lines | Select-Object -Skip 1) {
        if ($line -notmatch '^(.+?):\d+\.\d+,\d+\.\d+\s+\d+\s+\d+$') {
            throw "Invalid coverage row: $line"
        }
        $source = $Matches[1]
        if (Test-GeneratedSource -CoverageSource $source -ModulePath $ModulePath) {
            [void]$excluded.Add($source)
            continue
        }
        [void]$kept.Add($line)
    }

    $utf8NoBom = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllLines($FilteredProfile, $kept, $utf8NoBom)
    return @($excluded | Sort-Object)
}

$targetMap = @{
    all     = @("./...")
    api     = @("./internal/api/...")
    router  = @("./router/...")
    mqtt    = @("./mqtt/...", "./internal/processor/...")
    service = @("./internal/service/...")
    dal     = @("./internal/dal/...")
    utils   = @("./pkg/utils/...")
}

$rawProfile = Join-Path $CoverageDir "$Target.raw.out"
$filteredProfile = Join-Path $CoverageDir "$Target.out"
$summaryFile = Join-Path $CoverageDir "$Target-summary.json"
$packageArgs = @($targetMap[$Target])
$testArgs = @($packageArgs) + @(
    "-coverprofile=$rawProfile",
    "-covermode=atomic",
    "-count=1",
    "-p=$PackageParallelism",
    "-timeout=10m"
)

Write-Host "Running backend coverage for: $($packageArgs -join ' ')" -ForegroundColor Yellow
Push-Location $PSScriptRoot
try {
    & go test @testArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Backend tests failed; coverage cannot be accepted"
    }

    $modulePath = Get-ModulePath
    $excludedFiles = Write-FilteredCoverageProfile `
        -RawProfile $rawProfile `
        -FilteredProfile $filteredProfile `
        -ModulePath $modulePath
    $rawCoverage = Get-CoveragePercent -Profile $rawProfile
    $filteredCoverage = Get-CoveragePercent -Profile $filteredProfile

    $summary = [ordered]@{
        generatedAt = (Get-Date).ToUniversalTime().ToString("o")
        target = $Target
        packages = $packageArgs
        packageParallelism = $PackageParallelism
        rawProfile = $rawProfile
        filteredProfile = $filteredProfile
        rawCoveragePercent = $rawCoverage
        filteredCoveragePercent = $filteredCoverage
        generatedFilesExcluded = $excludedFiles
        minimumThresholdPercent = $Threshold
        minimumThresholdMet = ($filteredCoverage -ge $Threshold)
        businessClosureProven = $false
        note = "The percentage gate excludes only generated sources and does not prove API, E2E, or business-flow closure."
    }
    $summaryJson = $summary | ConvertTo-Json -Depth 6
    [System.IO.File]::WriteAllText($summaryFile, $summaryJson, [System.Text.UTF8Encoding]::new($false))

    Write-Host ""
    Write-Host "Raw coverage:      $rawCoverage%" -ForegroundColor Cyan
    Write-Host "Filtered coverage: $filteredCoverage%" -ForegroundColor Cyan
    Write-Host "Generated files excluded: $($excludedFiles.Count)"
    Write-Host "Summary: $summaryFile"

    if ($Html) {
        $htmlFile = Join-Path $CoverageDir "$Target.html"
        & go tool cover "-html=$filteredProfile" "-o=$htmlFile"
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to generate HTML coverage report"
        }
        Write-Host "HTML report: $htmlFile" -ForegroundColor Green
    }

    if ($Check -and $filteredCoverage -lt $Threshold) {
        throw "Filtered coverage $filteredCoverage% is below the minimum gate $Threshold%"
    }

    if ($Check) {
        Write-Host "Minimum coverage gate passed. This is not a business-closure claim." -ForegroundColor Green
    }
}
finally {
    Pop-Location
}
