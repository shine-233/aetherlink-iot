# frontend/src/views/dashboard

## Current scope

This folder now covers the live dashboard-facing views that still matter in the
current AetherLink IoT frontend.

Current active subfolders:

- `workspace/`: a live visualization entry page that links customers into
  ThingsVis projects, dashboard menus, and the IoT workbench
- `rdi-overview/`: live RDI operations overview content, including an owner-scoped yearly selector and twelve-month alarm-occurrence trend
- `workbench/`: live IoT workbench entry with real metric cards and operational
  navigation

## Retired slices

The older dashboard residue has already been narrowed down:

- `analysis/` is no longer a real customer page chain; its old component tree
  is gone and any remaining references should be treated as compatibility or
  stale automation/doc residue
- `mobile-workspace/` has been removed from the live source tree
- old dashboard wording in this area should not be treated as a current runtime
  feature

## Maintenance rule

When editing this area, keep the distinction clear between:

- real IoT metric or workflow views
- honest navigation shells
- retired compatibility residue that should be deleted instead of dressed up as
  a live dashboard
