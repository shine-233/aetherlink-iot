# Custom Components

`frontend/src/components/custom` contains small shared UI components used across
the AetherLink IoT console. These components should stay presentation-focused:
icons, icon buttons, scrolling containers, verification canvas, number display,
chart containers, icon selection, and brand visuals.

## Files

- `svg-icon.vue`: renders Iconify icons or local SVG symbols with class/style passthrough.
- `button-icon.vue`: wraps a Naive UI icon button with optional tooltip content.
- `better-scroll.vue`: owns a BetterScroll instance for scrollable local content.
- `ChartComponent.vue`: hosts an ECharts instance through the shared chart hook.
- `count-to.vue`: animates formatted numeric values.
- `icon-select.vue`: lets forms search and choose from a stable icon list.
- `image-verify.vue`: renders and refreshes the canvas verification code.
- `wave-bg.vue`: draws the branded login/display background.
- `aetherlink-avatar.vue`: renders the static AetherLink brand avatar asset.

## Maintenance Notes

- Keep business workflows, API calls, permissions, and page state out of this directory.
- When changing icon rendering, check both local SVG and Iconify fallback behavior.
- When changing chart or scroll behavior, keep resize/refresh paths explicit and testable.
- Do not add sample pages or template KPI copy here; route-level examples belong in route-specific views or tests.
