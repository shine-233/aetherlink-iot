# Broker Scripts

`mqtt-broker/script` contains broker maintenance or helper scripts.

## Folder Role

- Provides local operational scripts used around broker development or deployment.
- Scripts should remain free of environment-specific secrets.

## Review Notes

- Problem: scripts can encode local paths or credentials.
- Improvement: document inputs, outputs, and required environment variables before publishing.
- Expected effect: safer public repository hygiene.
