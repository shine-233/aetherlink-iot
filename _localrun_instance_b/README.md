# Isolated validation instance B

This directory is a public, repository-local contract for a second isolated
validation instance. Runtime logs, screenshots, reports, credentials, and
compiled binaries belong under this directory but are ignored by Git.

Copy `instance-b.env.example` to a local environment file and replace only
the `CHANGE_ME_*` account values in the ignored copy. Keep database passwords,
JWT keys, MQTT credentials, and browser storage state out of this directory's
tracked files, reports, and command history.

The paths in the template are deliberately relative so the instance can be
created under any checkout. Use a separate database, broker port, backend
port, preview port, report directory, and auth directory for each concurrent
run. This template is configuration guidance only; it is not a deployment
profile or proof that the target services are available.
