# frontend/src/views/_builtin/legal

## Folder Role

This page keeps the old `/terms` and `/privacy` links reachable from login and
registration screens. The old site exposed those links, but the available old
site source did not include formal legal document bodies.

## Maintenance Notes

- Keep the route constant and unauthenticated.
- Do not invent legal text in this component.
- Replace the placeholder with a configured document/CMS source when the
  deployment has approved terms and privacy content.
