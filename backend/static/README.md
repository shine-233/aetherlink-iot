# Backend Static Assets

`backend/static` contains static files served or packaged by the backend.

## Folder Role

- Stores public static assets needed by backend routes or deployment packaging.
- Should not contain generated reports, local uploads, credentials, or private runtime files.

## Review Notes

- Problem: static folders can accidentally accumulate runtime artifacts.
- Improvement: keep generated/runtime files under ignored runtime directories and document intentional public assets.
- Expected effect: cleaner public source uploads.

## Vendored Metrics Viewer Dependency

`echarts.min.js` is the minified ECharts browser bundle used by the embedded
metrics viewer. It is intentionally vendored and served from the same backend
origin instead of loading executable code from a CDN at runtime. ECharts is
distributed under the Apache License 2.0; see the upstream project at
https://github.com/apache/echarts for the corresponding license and source.
