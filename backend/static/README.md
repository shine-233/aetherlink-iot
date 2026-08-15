# Backend Static Assets

`backend/static` contains static files served or packaged by the backend.

## Folder Role

- Stores public static assets needed by backend routes or deployment packaging.
- Should not contain generated reports, local uploads, credentials, or private runtime files.

## Review Notes

- Problem: static folders can accidentally accumulate runtime artifacts.
- Improvement: keep generated/runtime files under ignored runtime directories and document intentional public assets.
- Expected effect: cleaner public source uploads.
