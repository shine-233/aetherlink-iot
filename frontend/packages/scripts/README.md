# frontend scripts package

`frontend/packages/scripts` contains the local CLI used by the frontend
workspace for maintenance tasks such as route generation, cleanup, dependency
updates, changelog generation, and release preparation.

## Boundaries

- Do not store local paths, tokens, accounts, or environment-specific values in
  this package.
- Cleanup commands must stay bounded to generated files and dependency folders.
- Release commands should remain aligned with the root publication and
  validation documents.
