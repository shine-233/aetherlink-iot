param(
  [string]$OutputDir = "",
  [string]$PackageName = "aetherlink-iot-private-deploy"
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
if (-not $OutputDir) {
  $OutputDir = Join-Path $Root "dist"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputDir)) {
  $OutputDir = Join-Path $Root $OutputDir
}

$OutputDir = [System.IO.Path]::GetFullPath($OutputDir)
if (
  [string]::IsNullOrWhiteSpace($PackageName) -or
  $PackageName -in @(".", "..") -or
  $PackageName.IndexOfAny([System.IO.Path]::GetInvalidFileNameChars()) -ge 0 -or
  $PackageName.Contains("/") -or
  $PackageName.Contains("\")
) {
  throw "Package refused: package name must be a single path segment."
}
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$stageDir = [System.IO.Path]::GetFullPath((Join-Path $OutputDir "$PackageName-$timestamp"))
$archivePath = [System.IO.Path]::GetFullPath((Join-Path $OutputDir "$PackageName-$timestamp.zip"))
$outputPrefix = $OutputDir.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
foreach ($candidate in @($stageDir, $archivePath)) {
  if (-not $candidate.StartsWith($outputPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Package refused: package path escapes the output directory."
  }
}

$includeRoots = @(
  ".env.example",
  "docker-compose.yml",
  "start-aetherlink.cmd",
  "start-aetherlink.ps1",
  "start-aetherlink.sh",
  "deploy",
  "backend",
  "frontend",
  "mqtt-broker",
  "performance",
  "verification/templates",
  "START-HERE.md",
  "README.md",
  "SECURITY.md",
  "VALIDATION.md",
  "THIRD_PARTY_NOTICES.md"
)

$excludedSegments = @(
  ".git",
  ".idea",
  ".vscode",
  "node_modules",
  "dist",
  "coverage",
  ".cache",
  ".vite",
  ".turbo",
  "__pycache__",
  "_localrun"
)

# Keep frontend/build: it is Vite configuration source imported by
# frontend/vite.config.ts. Only known generated build paths belong here.
$excludedPaths = @(
  "mqtt-broker/build"
)

function Test-AetherLinkExcludedPath {
  param([string]$RelativePath)

  $normalizedPath = $RelativePath.Replace("\", "/").Trim("/")
  $leafName = [System.IO.Path]::GetFileName($normalizedPath)
  if (
    $leafName.Equals(".env", [System.StringComparison]::OrdinalIgnoreCase) -or
    $leafName.StartsWith(".env.", [System.StringComparison]::OrdinalIgnoreCase) -or
    $leafName.EndsWith(".log", [System.StringComparison]::OrdinalIgnoreCase) -or
    $leafName.Equals(".tsbuildinfo", [System.StringComparison]::OrdinalIgnoreCase)
  ) {
    # Only the explicitly included root .env.example is allowed in a package;
    # recursive project-local env files, runtime logs, and compiler metadata
    # are machine-specific artifacts rather than deployment inputs.
    return $true
  }
  foreach ($excludedPath in $excludedPaths) {
    if (
      $normalizedPath -eq $excludedPath -or
      $normalizedPath.StartsWith("$excludedPath/", [System.StringComparison]::OrdinalIgnoreCase)
    ) {
      return $true
    }
  }

  $segments = $normalizedPath -split "/"
  foreach ($segment in $segments) {
    if ($excludedSegments -contains $segment) {
      return $true
    }
  }
  return $false
}

function Get-AetherLinkRelativePath {
  param([Parameter(Mandatory = $true)][string]$AbsolutePath)

  # [IO.Path]::GetRelativePath exists in .NET Core/PowerShell 7, but the
  # Windows starter and deployment workflow also support Windows PowerShell
  # 5.1. All package sources are descendants of $Root, so a validated prefix
  # strip is deterministic and keeps the package path inside the root.
  $rootPrefix = $Root.Path.TrimEnd('\', '/') + [System.IO.Path]::DirectorySeparatorChar
  $fullPath = [System.IO.Path]::GetFullPath($AbsolutePath)
  if (-not $fullPath.StartsWith($rootPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Package refused: source path escapes the repository root: $AbsolutePath"
  }
  return $fullPath.Substring($rootPrefix.Length).Replace('\', '/')
}

function Copy-AetherLinkPackageItem {
  param(
    [string]$Source,
    [string]$RelativeRoot
  )

  if ((Get-Item -LiteralPath $Source).PSIsContainer) {
    Get-ChildItem -LiteralPath $Source -Recurse -Force | ForEach-Object {
      $relativePath = Get-AetherLinkRelativePath -AbsolutePath $_.FullName
      if (Test-AetherLinkExcludedPath $relativePath) {
        return
      }

      $targetPath = Join-Path $stageDir $relativePath
      if ($_.PSIsContainer) {
        New-Item -ItemType Directory -Force -Path $targetPath | Out-Null
      } else {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $targetPath) | Out-Null
        Copy-Item -LiteralPath $_.FullName -Destination $targetPath -Force
      }
    }
    return
  }

  $targetFile = Join-Path $stageDir $RelativeRoot
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $targetFile) | Out-Null
  Copy-Item -LiteralPath $Source -Destination $targetFile -Force
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
if (Test-Path -LiteralPath $stageDir) {
  Remove-Item -LiteralPath $stageDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $stageDir | Out-Null

foreach ($relativeRoot in $includeRoots) {
  $source = Join-Path $Root $relativeRoot
  if (-not (Test-Path -LiteralPath $source)) {
    Write-Warning "Skipping missing package item: $relativeRoot"
    continue
  }
  Copy-AetherLinkPackageItem -Source $source -RelativeRoot $relativeRoot
}

$manifest = [ordered]@{
  package = $PackageName
  created_at = (Get-Date).ToString("o")
  quick_start = @(
    "Unzip the package",
    "Double-click start-aetherlink.cmd on Windows or run start-aetherlink.ps1",
    "Run sh ./start-aetherlink.sh on Linux/macOS",
    "Use -PerformanceTier light or --performance-tier light on low-resource machines",
    "Open AETHERLINK_PUBLIC_URL/first-device after containers become healthy",
    "Follow the first-device onboarding flow until the success proof can be downloaded and handed off for closeout manifest generation"
  )
  first_device_entry = "AETHERLINK_PUBLIC_URL/first-device"
  next_after_startup = @(
    "Open the first_device_entry URL",
    "Finish super admin and tenant admin setup if prompted",
    "Check deployment health",
    "Generate the first device",
    "Run the Web MQTT/HTTP online tester or publish from a real device",
    "Confirm online status, latest telemetry, and the first chart",
    "Download the first-device success proof",
    "Generate or hand off the first-device closeout manifest with deploy/first-device-closeout.*"
  )
  package_boundary = @(
    "Source-build private deployment package",
    "Target machine needs Docker and network access to pull/build images unless images are prepared separately",
    "Performance tiers are resource presets, not measured capacity claims"
  )
  required_external_inputs = @(
    "AETHERLINK_PUBLIC_URL: provide the real browser address; do not deploy server mode with localhost or loopback",
    "AETHERLINK_MQTT_ACCESS_ADDRESS: provide the real device MQTT host and port; do not deploy server mode with localhost or loopback",
    "Docker and Docker Compose: install and start them on the target machine before running init"
  )
  server_mode_command_windows = ".\\deploy\\init.ps1 -Server -PublicUrl <public-url> -MqttAddress <mqtt-host:port> -PerformanceTier standard"
  server_mode_command_posix = "sh ./deploy/init.sh --server --public-url <public-url> --mqtt-address <mqtt-host:port> --performance-tier standard"
  included = $includeRoots
  excluded_segments = $excludedSegments
  excluded_paths = $excludedPaths
  excluded_file_patterns = @("*.log", "*.tsbuildinfo")
  retained_source_paths = @("frontend/build")
}

$manifest | ConvertTo-Json -Depth 4 | Set-Content -Path (Join-Path $stageDir "PACKAGE-MANIFEST.json") -Encoding utf8

if (Test-Path -LiteralPath $archivePath) {
  Remove-Item -LiteralPath $archivePath -Force
}
Compress-Archive -Path (Join-Path $stageDir "*") -DestinationPath $archivePath

Write-Host "Created deployment package:"
Write-Host $archivePath
