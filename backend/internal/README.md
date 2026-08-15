# Backend Internal Modules

`backend/internal` contains the private backend implementation for AetherLink
IoT: HTTP handlers, business services, data access, models, middleware, device
uplink/downlink processing, protocol adapters, diagnostics, and application
assembly.

## Directory Responsibilities

- `api/`: Gin handlers, request binding, entry validation, permission entry
  points, and response shaping.
- `service/`: tenant boundaries, business rules, automation orchestration,
  side effects, and coordination across DAL and adapters.
- `dal/`: database queries, pagination, filters, persistence helpers, and local
  caches.
- `model/`: HTTP request/response structs, database models, and domain structs.
- `middleware/`: authentication, authorization, request-chain handling, and
  compatibility response middleware.
- `query/`: generated GORM query helpers; treat these as generated files.
- `uplink/`, `downlink`, `processor`, `storage`: device message pipelines,
  codec boundaries, and storage coordination.
- `adapter`, `app`, `diagnostics`, `logic`: protocol adaptation, application
  lifecycle, diagnostics, and runtime assembly.

## Maintenance Rules

- Keep handlers thin. Put request and permission entry checks in `api/`, then
  move reusable business behavior into `service/`.
- Guard tenant and permission boundaries before calling `dal/` or external
  adapters.
- Keep product-level business rules out of `dal/`; make query conditions,
  pagination, and error wrapping easy to read.
- Do not hand-edit generated code for long-lived changes. Use the generation
  boundary documented in `../../GENERATED_FILES.md` and `.gitattributes`.
- Document compatibility logic with its source and exit condition so future
  changes do not mistake historical support for the main path.

## Current Refactor Focus

- Split large service files by permission checks, data preparation,
  persistence, and side effects.
- Extract repeated query filters and pagination helpers in `dal/`.
- Sync frontend, API automation, and E2E metadata when changing parameter
  errors, response semantics, or compatibility constants.
- Keep README files tied to current boundaries. Do not describe deleted pages,
  old developer fixtures, or template copy as current capability.
