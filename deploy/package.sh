#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

PACKAGE_NAME="${PACKAGE_NAME:-aetherlink-iot-private-deploy}"
OUTPUT_DIR="${OUTPUT_DIR:-dist}"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"

case "$PACKAGE_NAME" in
  ""|.|..|*/*|*\\*)
    echo "Package refused: package name must be a single path segment." >&2
    exit 1
    ;;
esac

mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR="$(CDPATH= cd -- "$OUTPUT_DIR" && pwd -P)"
STAGE_DIR="${OUTPUT_DIR}/${PACKAGE_NAME}-${TIMESTAMP}"
ARCHIVE_PATH="${OUTPUT_DIR}/${PACKAGE_NAME}-${TIMESTAMP}.tar.gz"
case "$STAGE_DIR" in
  "$OUTPUT_DIR"/*) ;;
  *) echo "Package refused: staging path escapes the output directory." >&2; exit 1 ;;
esac
case "$ARCHIVE_PATH" in
  "$OUTPUT_DIR"/*) ;;
  *) echo "Package refused: archive path escapes the output directory." >&2; exit 1 ;;
esac

rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"

# Exclude generated output paths without dropping frontend/build, which contains
# Vite configuration source imported by frontend/vite.config.ts.
tar \
  --exclude='.git' \
  --exclude='.idea' \
  --exclude='.vscode' \
  --exclude='node_modules' \
  --exclude='mqtt-broker/build' \
  --exclude='dist' \
  --exclude='coverage' \
  --exclude='.cache' \
  --exclude='.vite' \
  --exclude='.turbo' \
  --exclude='__pycache__' \
  --exclude='_localrun' \
  --exclude='*.log' \
  --exclude='*.tsbuildinfo' \
  --exclude='*/.env*' \
  -cf - \
  .env.example \
  docker-compose.yml \
  start-aetherlink.cmd \
  start-aetherlink.ps1 \
  start-aetherlink.sh \
  deploy \
  backend \
  frontend \
  mqtt-broker \
  performance \
  verification/templates \
  START-HERE.md \
  README.md \
  SECURITY.md \
  VALIDATION.md \
  THIRD_PARTY_NOTICES.md | tar -xf - -C "$STAGE_DIR"

cat >"${STAGE_DIR}/PACKAGE-MANIFEST.json" <<EOF
{
  "package": "${PACKAGE_NAME}",
  "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "quick_start": [
    "Unzip the package",
    "Double-click start-aetherlink.cmd on Windows or run start-aetherlink.ps1",
    "Run sh ./start-aetherlink.sh on Linux/macOS",
    "Use -PerformanceTier light or --performance-tier light on low-resource machines",
    "Open AETHERLINK_PUBLIC_URL/first-device after containers become healthy",
    "Follow the first-device onboarding flow until the success proof can be downloaded and handed off for closeout manifest generation"
  ],
  "first_device_entry": "AETHERLINK_PUBLIC_URL/first-device",
  "next_after_startup": [
    "Open the first_device_entry URL",
    "Finish super admin and tenant admin setup if prompted",
    "Check deployment health",
    "Generate the first device",
    "Run the Web MQTT/HTTP online tester or publish from a real device",
    "Confirm online status, latest telemetry, and the first chart",
    "Download the first-device success proof",
    "Generate or hand off the first-device closeout manifest with deploy/first-device-closeout.*"
  ],
  "package_boundary": [
    "Source-build private deployment package",
    "Target machine needs Docker and network access to pull/build images unless images are prepared separately",
    "Performance tiers are resource presets, not measured capacity claims"
  ],
  "required_external_inputs": [
    "AETHERLINK_PUBLIC_URL: provide the real browser address; do not deploy server mode with localhost or loopback",
    "AETHERLINK_MQTT_ACCESS_ADDRESS: provide the real device MQTT host and port; do not deploy server mode with localhost or loopback",
    "Docker and Docker Compose: install and start them on the target machine before running init"
  ],
  "server_mode_command_windows": ".\\\\deploy\\\\init.ps1 -Server -PublicUrl <public-url> -MqttAddress <mqtt-host:port> -PerformanceTier standard",
  "server_mode_command_posix": "sh ./deploy/init.sh --server --public-url <public-url> --mqtt-address <mqtt-host:port> --performance-tier standard",
  "included": [
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
  ],
  "excluded_segments": [
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
  ],
  "excluded_file_patterns": [
    "*.log",
    "*.tsbuildinfo"
  ],
  "excluded_paths": [
    "mqtt-broker/build"
  ],
  "retained_source_paths": [
    "frontend/build"
  ]
}
EOF

tar -czf "$ARCHIVE_PATH" -C "$STAGE_DIR" .

echo "Created deployment package:"
echo "$ARCHIVE_PATH"
