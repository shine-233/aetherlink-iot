# Built-in Views

This directory contains framework-level pages that are not ordinary business
modules.

Current subdirectories:

- `403/`, `404/`, `500/`: built-in error pages used by route guards and fallback
  routing.
- `legal/`: reachable `/terms` and `/privacy` placeholders. Keep the routes, but
  do not treat them as final legal content until formal copy is supplied.
- `login/`: login and account-entry views.
- `__tests__/`: focused tests for built-in pages.

When changing these views, keep route names, visible text, and fallback behavior
in sync with router metadata and route-coverage contracts.
