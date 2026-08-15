# Backend Third-Party Interfaces

`backend/third_party` contains vendored or generated third-party interface code needed by the backend.

## Folder Role

- Preserves external interface contracts such as generated gRPC/protobuf clients and compatibility adapters.
- `others/http_client` contains the platform-side HTTP protocol-plugin client contract and tests; it does not contain the production HTTP adapter server. The real adapter remains an explicitly enabled external optional runtime.
- Changes here should be treated as external contract updates.

## Review Notes

- Problem: third-party generated code is easy to hand-edit accidentally.
- Improvement: document source inputs and regeneration commands in `../../GENERATED_FILES.md`, and keep `.gitattributes` generated markers current. If an upstream proto or schema is not checked in, record its external provenance, checked-at, and approved regeneration owner there; do not invent a local source file or hand-edit generated output.
- Expected effect: reproducible third-party interface maintenance.
