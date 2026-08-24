# AetherLink IoT One-Click Deployment

This folder contains the private-deployment bootstrap files for a single-node
AetherLink IoT install.

## Start Here For Non-Technical Installers

If you only want to get the platform running, use the shortest path below.
Run commands from the repository root.

Windows PowerShell, local machine:

```powershell
.\start-aetherlink.ps1 -Doctor
.\start-aetherlink.ps1
```

Low-resource machine:

```powershell
.\start-aetherlink.ps1 -Doctor -PerformanceTier light
.\start-aetherlink.ps1 -PerformanceTier light
```

Windows double-click entry:

- Double-click `start-aetherlink.cmd` from the repository root for the guided
  first startup. It prints the browser URL and device MQTT address after a
  successful start, then opens the browser automatically.
- From PowerShell, add `-Open` when you want the browser to open after startup:
  `.\start-aetherlink.ps1 -Open`.
- `deploy\start-windows.cmd` remains available for older packages. When
  `.env` does not exist yet, it asks whether this is local-only or a
  server/private deployment before generating addresses.
- If `.env` already exists but still uses local-only public addresses such as
  `http://localhost:8080` or `localhost:1883`, the same starter warns before
  startup and can switch those public addresses to server/private values.
- For server addresses, open PowerShell and run `deploy\start-windows.ps1` with
  `-Server`, `-PublicUrl`, and `-MqttAddress`.

Linux/macOS, local machine:

```sh
sh ./start-aetherlink.sh --doctor
sh ./start-aetherlink.sh
```

Low-resource machine:

```sh
sh ./start-aetherlink.sh --doctor --performance-tier light
sh ./start-aetherlink.sh --performance-tier light
```

Server install:

- Replace `1.2.3.4` with the IP address or domain that both the browser and
  devices can reach.
- Use `AETHERLINK_PUBLIC_URL` as the web console address.
- Use `AETHERLINK_MQTT_ACCESS_ADDRESS` as the device MQTT address.
- Do not use `localhost` for devices unless the device program runs on the
  same machine as Docker.
- In `-Server` / `--server` mode, localhost browser or MQTT public addresses
  are blocking doctor errors, not warnings.
- In server mode, init also changes an untouched loopback
  `AETHERLINK_BIND_ADDRESS` to `0.0.0.0` so the published frontend, backend,
  and MQTT ports can accept remote traffic. The broker metrics port (8082) is
  exempt: it is unauthenticated and always published on `127.0.0.1` only.
  Prefer a trusted host interface
  with `-BindAddress` / `--bind-address` when possible, and apply the host
  firewall policy before starting a public deployment.

Windows PowerShell, server:

```powershell
.\start-aetherlink.ps1 -Doctor -Server -PublicUrl "http://1.2.3.4:8080" -MqttAddress "1.2.3.4:1883" -BindAddress "0.0.0.0"
.\start-aetherlink.ps1 -Server -PublicUrl "http://1.2.3.4:8080" -MqttAddress "1.2.3.4:1883" -BindAddress "0.0.0.0"

.\deploy\start-windows.ps1 -Doctor -Server -PublicUrl "http://1.2.3.4:8080" -MqttAddress "1.2.3.4:1883" -BindAddress "0.0.0.0"
.\deploy\start-windows.ps1 -Server -PublicUrl "http://1.2.3.4:8080" -MqttAddress "1.2.3.4:1883" -BindAddress "0.0.0.0"
```

Linux/macOS, server:

```sh
sh ./start-aetherlink.sh --doctor --server --public-url http://1.2.3.4:8080 --mqtt-address 1.2.3.4:1883 --bind-address 0.0.0.0
sh ./start-aetherlink.sh --server --public-url http://1.2.3.4:8080 --mqtt-address 1.2.3.4:1883 --bind-address 0.0.0.0
```

After startup, open `AETHERLINK_PUBLIC_URL/first-device`.
Then follow 接入第一台设备: finish setup, check deployment health, generate the
first device, send the first telemetry, confirm latest data plus the first
chart, and download the success proof.
The root starters also print the resolved browser URL and device MQTT address
after startup so installers do not need to open `.env` just to find the next
step.

### Performance Tier

Choose one tier before startup:

- `light`: default. Use this for first-device trials, weak PCs, and about
  1C/2GB starting resources.
- `standard`: use this for normal small private installs, starting around
  2C/4GB.
- `production`: use this for larger private installs, starting around 4C/8GB.

The tier writes Docker Compose CPU and memory limits for PostgreSQL, Redis,
GMQTT, backend, and frontend into `.env`, then `docker-compose.yml` consumes
those values. This keeps first installs lighter without asking users to tune
container resources by hand.

These tiers are resource presets, not capacity promises. Do not claim a device
count or message rate until `verification/performance-*` contains a real
benchmark run for the selected hardware.

Redis defaults to `AETHERLINK_REDIS_MAXMEMORY=96mb`, below the default 128m
container limit, so AOF, allocator overhead, and client buffers retain headroom.
The resource presets currently set `AETHERLINK_REDIS_MEM_LIMIT` to 128m, 256m,
or 512m. If an operator raises or lowers that container limit, they must also
set `AETHERLINK_REDIS_MAXMEMORY` to a strictly lower value. The shared Redis
instance contains non-cache state, so its `noeviction` policy rejects writes in
a controlled way at the memory watermark instead of silently deleting keys;
AOF remains enabled. Monitor `INFO memory` and `INFO persistence`. Real load,
watermark, write-rejection, restart, and recovery validation remains pending.
See the official Redis documentation for
[Key eviction](https://redis.io/docs/latest/develop/reference/eviction/) and
[Redis persistence](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/).
The init scripts deliberately do not rewrite this setting, so resource-tier
selection cannot overwrite an operator's custom value.

### First Super Admin CLI Recovery

If the first-run browser page is unavailable, blank, or blocked by a frontend
problem, create only the first local super admin from the command line after
the backend is up:

```powershell
.\deploy\first-admin.ps1
```

```sh
sh ./deploy/first-admin.sh
```

The script reads `BACKEND_PORT` and `AETHERLINK_PUBLIC_URL` from `.env` and
defaults to `http://localhost:9999` for the backend. It calls
`GET /api/v1/tenant/setup-state` first and posts to
`POST /api/v1/tenant/super-admin/init` only when `has_admin=false` and
`next_step=create_super_admin`. This avoids creating extra super admins if the
system is already initialized.

Non-interactive examples:

```powershell
$env:AETHERLINK_FIRST_ADMIN_EMAIL = "admin@example.com"
$env:AETHERLINK_FIRST_ADMIN_PASSWORD = "Aa123456!"
.\deploy\first-admin.ps1 -BackendUrl "http://127.0.0.1:9999"
```

```sh
AETHERLINK_FIRST_ADMIN_EMAIL=admin@example.com \
AETHERLINK_FIRST_ADMIN_PASSWORD='Aa123456!' \
sh ./deploy/first-admin.sh --backend-url http://127.0.0.1:9999
```

Do not store the password in `.env`. Use the environment-variable form only
for one-time trusted automation; the interactive prompt is safer for manual
installs. After this script succeeds, sign in as the super admin, create the
tenant admin at `/management/user?setup=tenant-admin`, then sign in as the
tenant admin and continue 接入第一台设备 from `/first-device`.

## Quick Start

Run a no-start preflight first when installing on a new machine:

```sh
./deploy/init.sh --doctor
```

On Windows PowerShell:

```powershell
.\deploy\init.ps1 -Doctor
```

The doctor checks Docker, Compose, Docker daemon reachability, `.env`,
generated database/Redis/JWT/MQTT secrets, the stable broker ID, `.env`
syntax, required and extra env keys, PostgreSQL database/user/password
alignment, MQTT root/plugin credential separation, strict MQTT endpoint syntax
(hostname, IPv4, or bracketed IPv6 plus port 1-65535), public/OTA/MQTT address
consistency, local conflicts and duplicates for ports published by the default
Compose stack, disk, memory, and Compose config without starting containers.
When run with `-Server` / `--server`, the doctor fails if
`AETHERLINK_PUBLIC_URL` or `AETHERLINK_MQTT_ACCESS_ADDRESS` still points at
`localhost`, `127.0.0.1`, or loopback IPv6.

Shared TSV fixtures under `deploy/tests/fixtures` lock the Shell and PowerShell
pure-rule contracts for MQTT endpoint parsing and normalization, localhost
classification, TCP port conversion, and performance-tier normalization. These
contract tests do not replace runtime validation: Docker and Compose behavior,
port occupancy, host resources, and live database reachability still require a
real deployment environment.

The default Compose stack publishes MQTT on port 1883 and does not enable or
publish MQTTS on port 8883. MQTTS is opt-in through a reviewed broker/Compose
override, which must also provide TLS material, port mapping, and its own port
conflict validation.

On Linux/macOS you can run scripts with `sh ./deploy/init.sh` and
`sh ./deploy/doctor.sh` even when executable bits were lost during packaging.
On Windows, run the PowerShell commands from the repository root.

1. Run the helper script. It creates `.env` with generated local secrets when
   `.env` does not exist yet, validates Docker Compose config, starts the
   stack, waits for the main health checks, and writes a startup archive under
   `verification/startup-<timestamp>/`.

```sh
./deploy/init.sh
```

On Windows PowerShell:

```powershell
.\deploy\init.ps1
```

To choose a resource preset:

```sh
./deploy/init.sh --performance-tier light
./deploy/init.sh --performance-tier standard
./deploy/init.sh --performance-tier production
```

On Windows PowerShell:

```powershell
.\deploy\init.ps1 -PerformanceTier light
.\deploy\init.ps1 -PerformanceTier standard
.\deploy\init.ps1 -PerformanceTier production
```

2. For a server install, pass or set the public addresses before the first run:

```powershell
$env:AETHERLINK_PUBLIC_URL = "http://192.168.1.10:8080"
$env:AETHERLINK_MQTT_ACCESS_ADDRESS = "192.168.1.10:1883"
.\deploy\init.ps1 -Server
```

```sh
AETHERLINK_PUBLIC_URL=http://192.168.1.10:8080 \
AETHERLINK_MQTT_ACCESS_ADDRESS=192.168.1.10:1883 \
./deploy/init.sh --server
```

If you already have a reviewed `.env`, `docker compose up -d --build` still
works directly.

Replace `192.168.1.10` with the address that users and devices can actually
reach. For LAN-only installs this can be a LAN IP. For remote customers this is
usually a public IP or domain. If a reverse proxy maps HTTPS or different
external ports, keep `.env` aligned with the address users and devices see.

Important: `init` does not overwrite existing secrets or Docker volumes. If
`.env` already exists and you pass `-PublicUrl` / `--public-url` or
`-MqttAddress` / `--mqtt-address`, init updates only the matching public
address pairs before running doctor. Edit `.env` directly for exposed ports or
passwords, then run doctor again.

When updating public addresses through init, provide both browser and MQTT
addresses together. Passing only one public address is treated as incomplete
because it can leave generated device access material half on localhost.

Server mode is persisted as `AETHERLINK_SERVER_MODE=1` in `.env` and is passed
to the backend by Compose. In that mode, `/ready` adds required checks for the
browser and device addresses and returns `503/down` when either address is
loopback, invalid, or otherwise not suitable for remote access. Local-only
installs keep `AETHERLINK_SERVER_MODE=0` and retain the existing local defaults.

When editing `.env` by hand, keep the generated backend addresses aligned:

- `GOTP_OTA_DOWNLOAD_ADDRESS` should match `AETHERLINK_PUBLIC_URL`.
- `GOTP_MQTT_ACCESS_ADDRESS` should match `AETHERLINK_MQTT_ACCESS_ADDRESS`.

If those pairs drift apart, the web page can open correctly while generated
device access material still points to an old browser/MQTT address. Doctor reports this
as an address mismatch before startup.

`-Server` / `--server` also carries the release address gate through startup
verification. It fails before health polling when either address is missing,
local-only (`localhost`, loopback, or `0.0.0.0`), or a documented placeholder
such as `YOUR-IP` or `example.com`. For a direct verification run, set
`AETHERLINK_SERVER_MODE=1` before invoking `deploy/verify.ps1` or
`deploy/verify.sh`.

First run behavior:

- Generates local PostgreSQL, Redis, MQTT root/plugin, and JWT secrets only when `.env` is
  missing.
- Keeps an existing `.env` and existing Docker volumes.
- Writes startup verification evidence under `verification/startup-<timestamp>/`
  unless `--skip-verify` / `-SkipVerify` is used.
- Does not delete data. A clean reinstall with `docker compose down -v` removes
  Compose volumes, so back up data first.

### PostgreSQL Backup And Restore

Create a custom-format PostgreSQL dump while the default `postgres` service is
running. The scripts stream binary data directly between Docker and a file, then
write a SHA-256 sidecar and a secret-free manifest under
`verification/backups/` by default:

```powershell
.\deploy\backup.ps1
```

```sh
sh ./deploy/backup.sh
```

Pass an output directory as the first Shell argument or as PowerShell
`-OutputDir` when backups must be stored outside the checkout. The command
refuses a non-empty output directory instead of overwriting earlier evidence.
A backup is not recoverable evidence until its hash is verified and a restore
drill succeeds in a disposable environment. Docker is an explicit external
prerequisite; when it is unavailable the scripts stop with an `external blocker`
message rather than creating a partial or empty backup.

Restore is destructive and therefore requires both an explicit dump path and an
explicit confirmation flag. It refuses dumps without a matching SHA-256
sidecar, restores atomically with
`pg_restore --clean --if-exists --exit-on-error --single-transaction`, and runs
`ANALYZE` afterward. A restore error rolls back cleanup and recreation instead of
leaving a partially restored database:

```powershell
.\deploy\restore.ps1 -DumpFile verification\backups\postgres-...\database.dump -ConfirmRestore
```

```sh
sh ./deploy/restore.sh verification/backups/postgres-.../database.dump --confirm-restore
```

Stop application writes or use a maintenance window before restoration, and run
`deploy/verify.*` after it completes. These scripts back up the configured
application database only. They are **not a complete deployment backup**. The
following persistent states remain external-blocked until a coordinated,
maintenance-window backup and isolated restore drill are implemented:

- `redis-data` (AOF-backed plugin/business state),
- `broker-data` (broker-local persistence),
- `backend-files` (uploaded files and OTA artifacts),
- `backend-telemetry-spool` (raw telemetry awaiting replay),
- `backend-uplink-spool` (attribute/event envelopes awaiting replay).

Do not represent a PostgreSQL-only dump as covering those volumes, and do not
copy live volume directories as if that created a cross-service consistency
point. The scripts also do not include cluster-wide roles or tablespaces; use
`pg_dumpall --globals-only` under an independently reviewed operator procedure
when those objects must be preserved. See the
[PostgreSQL 16 SQL dump documentation](https://www.postgresql.org/docs/16/backup-dump.html)
for format, role, and restore semantics. Never run `docker compose down -v`
until a current backup and a restore drill have both been verified.

Security note: if `AETHERLINK_MQTT_ACCESS_ADDRESS` points outside localhost,
confirm broker authentication/ACL and firewall rules before production use.
Do not expose MQTT directly to the internet just because the container starts.

To start faster after images already exist:

```powershell
.\deploy\init.ps1 -NoBuild
```

```sh
./deploy/init.sh --no-build
```

To start without writing the startup verification archive:

```powershell
.\deploy\init.ps1 -SkipVerify
```

```sh
./deploy/init.sh --skip-verify
```

To re-run only startup verification:

```powershell
.\deploy\verify.ps1
```

```sh
./deploy/verify.sh
```

The startup verification archive writes `manifest.json` with the checked
frontend/backend/deployment-health/broker URLs, the resolved device MQTT
address, `first_device_url`, and the first-use next steps for finishing 接入第一台设备.
For the final first-device handoff, download the first-device success proof
from `/first-device`, then copy
`verification/templates/first-device-closeout-manifest.template.json` into the
same verification archive. Fill or pre-fill it with the actual startup archive,
delivery URLs, first admin/tenant evidence, device id, Web MQTT/HTTP tester
result, online/latest telemetry, first chart, deployment-health rows,
downloaded success proof, and API/E2E/Playwright archive paths.
See `verification/templates/README.md` for the evidence required before the
copied manifest can move beyond `verdict=unknown`.

To pre-fill that manifest from an existing startup archive and a downloaded
first-device success proof:

```powershell
.\deploy\first-device-closeout.ps1 -StartupManifest verification\startup-...\manifest.json -SuccessProof path\to\aetherlink-first-device-proof.json
```

```sh
sh ./deploy/first-device-closeout.sh --startup-manifest verification/startup-.../manifest.json --success-proof path/to/aetherlink-first-device-proof.json
```

The helper mirrors the downloaded success-proof handoff fields, keeps
`verdict=unknown`, and records blocking gaps until the copied manifest points
to real runtime plus API/E2E/Playwright evidence.

## Private Deployment Package

Create a portable deployment archive from the repository root:

```powershell
.\deploy\package.ps1
```

```sh
./deploy/package.sh
```

The package includes the Compose stack, backend, frontend, MQTT broker,
root-level one-click starters, deployment scripts, performance harness
manifests, and verification templates. It excludes local build outputs,
caches, `node_modules`, coverage output, and Git metadata so the archive stays
lighter.
Both Windows `.zip` and Linux/macOS `.tar.gz` packages include
`PACKAGE-MANIFEST.json` with the root starter quick-start steps,
`first_device_entry`, first-device next steps, package boundary notes, and the
included/excluded file groups.

Package boundary:

- This is a source-build private deployment package: the target machine runs
  Docker Compose and builds/pulls the required images there.
- After unzip, the intended first entry is `start-aetherlink.cmd` on Windows
  or `sh ./start-aetherlink.sh` on Linux/macOS.
- It is not a fully offline image package. Air-gapped installs still need image
  tarballs or a private registry containing PostgreSQL, Redis, frontend,
  backend, and MQTT broker images.
- The package intentionally excludes local `dist`, generated
  `mqtt-broker/build`, `node_modules`, coverage, verification archives, and Git
  metadata. It retains `frontend/build`, which is Vite configuration source.
- For a real server, do not leave browser/device addresses on `localhost`.
  Set `AETHERLINK_PUBLIC_URL` to the URL operators open, and
  `AETHERLINK_MQTT_ACCESS_ADDRESS` to the MQTT host:port devices can reach.

## Services

- Local browser console: `http://localhost:8080`
- Local backend API: `http://localhost:9999`
- Local MQTT: `localhost:1883`
- Local broker metrics: `http://localhost:8082/metrics`
- Server browser console: `.env` value `AETHERLINK_PUBLIC_URL`
- Server device MQTT address: `.env` value `AETHERLINK_MQTT_ACCESS_ADDRESS`
- PostgreSQL and Redis are internal by default.

## Enabling MQTT TLS (8883)

MQTT TLS is **off by default**. The default broker config only publishes
plaintext `:1883`; the `:8883` TLS listener ships commented out in
`mqtt-broker/cmd/gmqttd/default_config.yml`, and enabling it without mounted
certificate files stops the broker at startup by design. To turn it on:

1. Generate a development certificate:

   ```powershell
   powershell -ExecutionPolicy Bypass -File deploy\gen-mqtt-certs.ps1 -SubjectAltName 192.168.1.10
   ```

   ```sh
   sh deploy/gen-mqtt-certs.sh 192.168.1.10
   ```

   This writes `deploy/certs/server.crt` / `server.key` (git-ignored). Pass the
   server's IP or DNS name so devices can verify the hostname; `localhost` and
   `127.0.0.1` are always included. The generated material is self-signed and
   for development/intranet use only — production must use certificates from a
   proper CA.

2. Enable the listener and mount the certificates:

   - In `mqtt-broker/cmd/gmqttd/default_config.yml`, uncomment the `:8883`
     listener block and point it at the in-container paths:
     `cert: "./certs/server.crt"`, `key: "./certs/server.key"` (the `cacert`
     entry stays optional; only set it together with client-certificate
     verification for mTLS).
   - In `docker-compose.yml` (or an override file), add to the `mqtt-broker`
     service: volume `- ./deploy/certs:/gmqttd/certs:ro` and port
     `- "127.0.0.1:${MQTT_TLS_PORT:-8883}:8883"` (replace `127.0.0.1` with your
     access interface only when remote devices need MQTTS), then restart the
     stack.

3. Re-run the doctor (`start-aetherlink.ps1 -Doctor` /
   `start-aetherlink.sh --doctor`) and confirm the stack comes up healthy;
   then point devices at `mqtts://<host>:8883`.

## Durable Spool Monitoring

Production installs should load
`deploy/observability/telemetry-spool-alerts.yml` and
`deploy/observability/attribute-event-spool-alerts.yml` into their existing
Prometheus and route the resulting alerts through Alertmanager. Both rule
groups cover fallback activation, spool write failure, corruption, persistent
backlog, 80%/95% record and byte capacity, and non-empty quarantine. This
repository does not add Prometheus or Alertmanager to the lightweight Compose
stack; follow `deploy/observability/README.md` for scrape prerequisites,
per-instance capacity calibration, sensitive-data handling, and runbooks.

## First Use After Startup

1. Open `AETHERLINK_PUBLIC_URL/first-device` in a browser.
2. If the system asks for first-run setup, create the super admin and tenant
   admin account shown by the page.
3. Sign in as the tenant admin.
4. Stay on 接入第一台设备.
5. Generate the first device. For the quickest proof, click the page's browser
   online test / send-test action to publish one sample telemetry message; when
   using a real device, copy the generated MQTT/HTTP test command from the same page.
6. Confirm that latest telemetry and the first chart are visible, then download
   the success proof before moving on to automation, dashboards, OTA, or batch
   commands.

## Initialization Notes

The PostgreSQL container runs `deploy/postgres/00-run-migrations.sh` on first
database creation. That script sorts `backend/sql/*.sql` by version number so
`2.sql` runs before `10.sql`, then records the highest applied number in
`sys_version` so the backend does not replay the bootstrap schema. The complete
bootstrap chain and its version marker are sent through one
`psql --single-transaction` call, so a SQL failure rolls back the chain
atomically.

If `postgres-data` already exists, PostgreSQL will not rerun init scripts.
The backend instead applies numbered upgrades newer than `sys_version` during
startup, and now refuses to start if any required migration fails. Before
routing traffic after an upgrade, verify that `sys_version` equals the shipped
`VERSION_NUMBER` and that the expected tables/indexes exist. Older volumes
created before version seeding may require a backup and explicit migration
repair before the backend can start.

For a clean local reinstall, remove the compose volume after backing up data:

```sh
docker compose down -v
```

## Current Boundaries

This is the lightweight default stack: frontend, backend, PostgreSQL, Redis,
and GMQTT. ThingsVis Studio/Server and industrial protocol adapters should be
added as optional compose profiles instead of making the first install heavier.
Native visualization remains the default local implementation.

The hand-written backend and broker runtime images use the fixed non-root UID/GID
`10001`. Compose drops all Linux capabilities, applies `no-new-privileges`, and
sets bounded process counts for backend, broker, and frontend. The default stack
also separates PostgreSQL and Redis into distinct internal networks; application
traffic uses `core_net`, while the optional ThingsVis services use `thingsvis_net`
and join only the data plane they actually require. Read-only root filesystems
are not enabled yet because the remaining writable paths must first be proven by
a real Compose run. Database image initialization, host rootless Docker, kernel
security policy, and firewall enforcement remain deployment-side validation rather
than claims made by the repository.

MQTT payload-schema enforcement is also local: when
`AETHERLINK_PAYLOAD_SCHEMA_ENABLED=true`, GMQTT reuses its existing PostgreSQL
connection and caches schema lookups for `AETHERLINK_PAYLOAD_SCHEMA_CACHE_TTL`.
It requires migration 46 and remains disabled by default until reject semantics
have been deployment-tested. Resolver lookup and malformed-schema failures are
fail-open so a schema outage does not break the established uplink contract.
The real Broker + PostgreSQL + MQTT publish E2E remains external-blocked when
Docker is unavailable; unit and contract tests do not replace that deployment
evidence.

The backend deployment-health capability contract preserves the existing
`enabled`, `configured`, and `healthy` booleans and also exposes a normalized
`status`: `disabled`, `configuration-required`, `blocked`, `external-blocked`,
or `available`. ThingsVis, the HTTP adapter, Market, SMTP, and map providers are
`external-optional`; a configured external capability that cannot be reached is
`external-blocked`, but it does not make the required lightweight stack report
`down`. A local test fake or email capture adapter is test evidence only and
must never be reported as successful production delivery.

The default frontend Nginx config does not proxy to ThingsVis hosts because the
lightweight Compose stack does not start them. ThingsVis-specific paths return
`503 THINGSVIS_OPTIONAL_SERVICE_DISABLED` until an optional profile supplies
those services and an override Nginx config.

### Real ThingsVis and HTTP-plugin profile

The repository now includes
[`docker-compose.optional-integrations.yml`](docker-compose.optional-integrations.yml)
and [`../frontend/nginx.thingsvis.conf`](../frontend/nginx.thingsvis.conf).
They are deliberately separate from the default stack and use the public
ThingsPanel service contracts for `thingsvis-server`, `thingsvis-studio`, and
`http_adapter`. The profile also rebuilds the frontend with
`VITE_ENABLE_THINGSVIS_COMPAT=Y`; the default image keeps the legacy routes and
standalone preview disabled while native visualization remains available. The
profile is the supported way to turn the previously external-blocked E2E paths
into a real local run; it is not a mock or a `route.fulfill` substitute.

1. Copy the normal project `.env` and set a non-default ThingsVis secret:

   ```sh
   THINGSVIS_AUTH_SECRET=replace-with-a-local-random-secret
   ```

2. Start the existing stack together with the optional services:

   ```sh
   docker compose -f docker-compose.yml \
     -f deploy/docker-compose.optional-integrations.yml \
     --profile optional-integrations up -d
   ```

   The profile publishes ThingsVis Server on `8000`, Studio on `3000`, and the
   HTTP adapter on `19090/19091`. These host ports bind to `127.0.0.1` by
   default, matching the lightweight stack. Set `AETHERLINK_BIND_ADDRESS`
   explicitly only when another host must reach them, and apply the deployment
   firewall policy at the same time. The adapter intentionally keeps the seeded
   backend address `http_adapter:19091`, while its platform and MQTT targets
   are the current Compose service names `backend:9999` and
   `mqtt-broker:1883`.

3. For the Vite development server, set `VITE_THINGSVIS_API_URL` to the host
   address of the optional server (normally `http://127.0.0.1:8000`) and
   `VITE_THINGSVIS_STUDIO_URL` to `http://127.0.0.1:3000/main.html` when the
   frontend is not using the optional Nginx gateway.

The profile does not fabricate a backend `vis_dashboard` row. The dashboard
menu API still checks tenant ownership against that local table, so the
menu-persistence E2E requires a real mirrored/registered dashboard ID via
`THINGSVIS_MIRRORED_DASHBOARD_ID` (or a future explicit synchronization path).
Do not weaken that check or insert a fake row merely to turn the test green.
