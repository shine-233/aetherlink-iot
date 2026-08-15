#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

ARCHIVE_ROOT=""
STARTUP_MANIFEST=""
SUCCESS_PROOF=""
API_E2E_ARCHIVE=""
OUTPUT=""
OPERATOR_ROLE=""
NOTES=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help)
      cat <<'EOF'
AetherLink IoT first-device closeout manifest helper

Usage:
  sh ./deploy/first-device-closeout.sh
  sh ./deploy/first-device-closeout.sh --startup-manifest verification/startup-.../manifest.json --success-proof path/to/proof.json
  sh ./deploy/first-device-closeout.sh --archive-root verification/startup-... --success-proof path/to/proof.json --api-e2e-archive verification/...

This helper pre-fills a closeout manifest. It never marks verdict as passed.
EOF
      exit 0
      ;;
    --archive-root) ARCHIVE_ROOT="${2:-}"; shift ;;
    --startup-manifest) STARTUP_MANIFEST="${2:-}"; shift ;;
    --success-proof) SUCCESS_PROOF="${2:-}"; shift ;;
    --api-e2e-archive) API_E2E_ARCHIVE="${2:-}"; shift ;;
    --output) OUTPUT="${2:-}"; shift ;;
    --operator-role) OPERATOR_ROLE="${2:-}"; shift ;;
    --notes) NOTES="${2:-}"; shift ;;
    *)
      echo "Unknown option: $1" >&2
      exit 2
      ;;
  esac
  shift
done

python_bin=""
if command -v python3 >/dev/null 2>&1; then
  python_bin="python3"
elif command -v python >/dev/null 2>&1; then
  python_bin="python"
fi

if [ -z "$python_bin" ]; then
  echo "python3 or python is required to safely write the closeout JSON manifest." >&2
  exit 1
fi

"$python_bin" - "$ROOT_DIR" "$ARCHIVE_ROOT" "$STARTUP_MANIFEST" "$SUCCESS_PROOF" "$API_E2E_ARCHIVE" "$OUTPUT" "$OPERATOR_ROLE" "$NOTES" <<'PY'
import json
import os
import subprocess
import sys
from datetime import datetime, timezone

root, archive_root, startup_manifest, success_proof, api_e2e_archive, output, operator_role, notes = sys.argv[1:9]

def full_path(value):
    if not value:
        return ""
    if os.path.isabs(value):
        return os.path.abspath(value)
    return os.path.abspath(os.path.join(root, value))

def rel_path(value):
    if not value:
        return ""
    try:
        return os.path.relpath(value, root).replace("\\", "/")
    except Exception:
        return value

def as_dict(value):
    return value if isinstance(value, dict) else {}

def as_list(value):
    return value if isinstance(value, list) else []

def as_text(value):
    return "" if value is None else str(value)

def latest_startup_manifest():
    verification = os.path.join(root, "verification")
    if not os.path.isdir(verification):
        return ""
    candidates = []
    for name in os.listdir(verification):
        path = os.path.join(verification, name)
        manifest = os.path.join(path, "manifest.json")
        if name.startswith("startup-") and os.path.isfile(manifest):
            candidates.append((os.path.getmtime(manifest), manifest))
    if not candidates:
        return ""
    return sorted(candidates, reverse=True)[0][1]

def git_commit():
    try:
        result = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, check=True, capture_output=True, text=True)
        return result.stdout.strip().splitlines()[0]
    except Exception:
        return ""

template_path = os.path.join(root, "verification", "templates", "first-device-closeout-manifest.template.json")
with open(template_path, "r", encoding="utf-8") as fh:
    manifest = json.load(fh)

startup_manifest_path = full_path(startup_manifest) or latest_startup_manifest()
archive_root_path = full_path(archive_root)
if not archive_root_path and startup_manifest_path:
    archive_root_path = os.path.dirname(startup_manifest_path)
if not archive_root_path:
    stamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    archive_root_path = os.path.join(root, "verification", f"first-device-closeout-{stamp}")
os.makedirs(archive_root_path, exist_ok=True)

output_path = full_path(output) or os.path.join(archive_root_path, "first-device-closeout-manifest.json")

manifest["timestamp"] = datetime.now(timezone.utc).isoformat()
manifest["git_commit"] = git_commit()
manifest["operator"]["role"] = operator_role
manifest["operator"]["notes"] = notes

blocking_gaps = []

if startup_manifest_path and os.path.isfile(startup_manifest_path):
    with open(startup_manifest_path, "r", encoding="utf-8") as fh:
        startup = json.load(fh)
    manifest["startup_verification"]["archive"] = rel_path(os.path.dirname(startup_manifest_path))
    manifest["startup_verification"]["manifest"] = rel_path(startup_manifest_path)
    manifest["startup_verification"]["ok"] = bool(startup.get("ok"))
    manifest["target_url"] = str(startup.get("target_url") or "")
    manifest["first_device_url"] = str(startup.get("first_device_url") or "")
    if not manifest["first_device_url"] and manifest["target_url"]:
        manifest["first_device_url"] = manifest["target_url"].rstrip("/") + "/first-device"
    manifest["backend_url"] = str(startup.get("backend_url") or "")
    manifest["mqtt_access_address"] = str(startup.get("mqtt_access_address") or "")
    manifest["delivery"]["first_device_url"] = manifest["first_device_url"]
    manifest["delivery"]["proof_url"] = (
        manifest["target_url"].rstrip("/") + "/home?onboarding=first-device&focus=proof"
        if manifest["target_url"] else ""
    )
    if not startup.get("ok"):
        blocking_gaps.append("startup_verification_not_ok")
else:
    blocking_gaps.append("missing_startup_verification_manifest")

success_proof_path = full_path(success_proof)
if success_proof_path and os.path.isfile(success_proof_path):
    with open(success_proof_path, "r", encoding="utf-8") as fh:
        proof = json.load(fh)
    manifest["success_proof"]["downloaded"] = True
    manifest["success_proof"]["file"] = rel_path(success_proof_path)
    manifest["success_proof"]["schema"] = str(proof.get("schema") or "")
    manifest["success_proof"]["generated_at"] = as_text(proof.get("generated_at"))
    manifest["success_proof"]["ready"] = bool(proof.get("ready"))
    manifest["success_proof"]["conclusion"] = as_text(proof.get("conclusion"))
    manifest["success_proof"]["next_action"] = as_text(proof.get("next_action"))
    manifest["success_proof"]["current_blocker"] = proof.get("current_blocker")
    manifest["success_proof"]["handoff_summary"] = as_dict(proof.get("handoff_summary")) or manifest["success_proof"]["handoff_summary"]
    manifest["success_proof"]["proof_items"] = as_list(proof.get("proof_items"))

    delivery = as_dict(proof.get("delivery"))
    device = as_dict(proof.get("device"))
    connection = as_dict(proof.get("connection"))
    browser_test = as_dict(proof.get("browser_test"))
    latest_telemetry = as_dict(proof.get("latest_telemetry"))
    chart = as_dict(proof.get("chart"))
    handoff_summary = as_dict(proof.get("handoff_summary"))

    manifest["delivery"]["first_device_url"] = as_text(delivery.get("first_device_url")) or manifest["first_device_url"]
    manifest["delivery"]["proof_url"] = as_text(delivery.get("proof_url")) or (
        manifest["target_url"].rstrip("/") + "/home?onboarding=first-device&focus=proof"
        if manifest["target_url"] else ""
    )
    manifest["delivery"]["generated_from_page"] = as_text(delivery.get("generated_from_page"))
    manifest["delivery"]["proof_file_hint"] = as_text(delivery.get("proof_file_hint"))
    if manifest["delivery"]["first_device_url"]:
        manifest["first_device_url"] = manifest["delivery"]["first_device_url"]

    manifest["first_device"]["device_id"] = str(device.get("id") or "")
    manifest["first_device"]["device_name"] = str(device.get("name") or "")
    manifest["first_device"]["device_number"] = str(device.get("number") or "")
    manifest["first_device"]["device_created"] = bool(device.get("id"))
    manifest["first_device"]["product_or_template_created"] = bool(device.get("config_id"))
    manifest["first_device"]["protocol"] = str(connection.get("protocol") or "")
    manifest["first_device"]["connection_endpoint"] = str(connection.get("endpoint") or "")
    manifest["first_device"]["report_entry"] = str(connection.get("report_entry") or "")
    manifest["first_device"]["control_entry"] = str(connection.get("control_topic") or "")
    manifest["first_device"]["credential_state"] = "redacted"

    manifest["publish_test"]["tester"] = "Web MQTT/HTTP online tester"
    manifest["publish_test"]["sample_copied"] = handoff_summary.get("sample_command_state") == "present"
    manifest["publish_test"]["browser_test_sent"] = bool(browser_test.get("sent_at"))
    manifest["publish_test"]["browser_test_confirmed"] = browser_test.get("status") == "confirmed"
    manifest["publish_test"]["status"] = as_text(browser_test.get("status"))
    manifest["publish_test"]["message"] = as_text(browser_test.get("message"))
    manifest["publish_test"]["sent_at"] = str(browser_test.get("sent_at") or "")
    manifest["publish_test"]["telemetry_key"] = str(browser_test.get("telemetry_key") or "")
    manifest["publish_test"]["telemetry_value"] = str(browser_test.get("telemetry_value") or "")
    manifest["publish_test"]["raw_evidence"] = rel_path(success_proof_path)

    manifest["latest_telemetry"]["available"] = bool(latest_telemetry.get("available"))
    manifest["latest_telemetry"]["source"] = as_text(latest_telemetry.get("source"))
    manifest["latest_telemetry"]["key"] = as_text(latest_telemetry.get("key"))
    manifest["latest_telemetry"]["value"] = as_text(latest_telemetry.get("value"))
    manifest["latest_telemetry"]["observed_at"] = as_text(latest_telemetry.get("observed_at"))
    manifest["latest_telemetry"]["online"] = bool(latest_telemetry.get("online"))

    manifest["runtime_confirmation"]["device_online"] = bool(device.get("online"))
    manifest["runtime_confirmation"]["latest_telemetry_visible"] = bool(
        latest_telemetry.get("available") or chart.get("primary_key")
    )
    manifest["runtime_confirmation"]["first_chart_generated"] = bool(chart.get("ready"))
    manifest["runtime_confirmation"]["ready_banner_visible"] = bool(proof.get("ready"))
    manifest["runtime_confirmation"]["raw_evidence"] = rel_path(success_proof_path)

    deployment_health_rows = as_list(proof.get("deployment_health"))
    deployment_health_ok = sum(1 for row in deployment_health_rows if as_dict(row).get("ok"))
    manifest["deployment_health"]["total"] = len(deployment_health_rows)
    manifest["deployment_health"]["ok"] = deployment_health_ok
    manifest["deployment_health"]["failed"] = max(len(deployment_health_rows) - deployment_health_ok, 0)
    manifest["deployment_health"]["rows"] = deployment_health_rows

    if proof.get("current_blocker"):
        blocking_gaps.append("success_proof_current_blocker")
    if not proof.get("ready"):
        blocking_gaps.append("success_proof_not_ready")
else:
    blocking_gaps.append("missing_downloaded_success_proof")

api_archive_path = full_path(api_e2e_archive)
if api_archive_path:
    manifest["api_e2e_playwright"]["archive"] = rel_path(api_archive_path)
else:
    blocking_gaps.append("missing_api_e2e_playwright_archive")

manifest["verdict"] = "unknown"
manifest["blocking_gaps"] = blocking_gaps

with open(output_path, "w", encoding="utf-8") as fh:
    json.dump(manifest, fh, ensure_ascii=False, indent=2)
    fh.write("\n")

print("Created first-device closeout manifest:")
print(output_path)
print("Verdict remains unknown until real runtime and API/E2E/Playwright evidence are reviewed.")
PY
