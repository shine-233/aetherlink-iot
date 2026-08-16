# Container Release

AetherLink IoT publishes three source-built images to GitHub Container Registry:

- `ghcr.io/shine-233/aetherlink-iot-backend`
- `ghcr.io/shine-233/aetherlink-iot-frontend`
- `ghcr.io/shine-233/aetherlink-iot-mqtt-broker`

## Trigger and tags

`.github/workflows/container-release.yml` runs only for a `vMAJOR.MINOR.PATCH` tag push. It binds the build to the commit SHA from that tag-push event and rechecks the tag before publication. A stable tag produces semver tags (`0.1.0`, `0.1`) and the metadata action's stable `latest` tag.

The workflow uses the repository `GITHUB_TOKEN` with `packages: write`; it does not require production, API, database, broker, or device credentials. It is a package publication workflow, not a deployment workflow.

The three matrix legs publish independently. The validation gate blocks all
legs when the tagged source or required checks are invalid, but a build or
registry failure in one image can leave the other image tags already
published. A failed workflow must therefore be treated as a partial-publication
incident and the affected image tags should be retried or reconciled before
announcing the release.

## Supply-chain evidence

Each image build records the pushed image digest as the attestation subject. BuildKit is configured to publish an SBOM and maximum-detail provenance, and `actions/attest-build-provenance` pushes a registry-backed provenance attestation for that digest. The workflow is restricted to pinned action SHAs and has separate `id-token` and `attestations` permissions.

The image digest, SBOM, and provenance are hosted-release evidence. They do not prove that the image was deployed, that external integrations are configured, or that a real device completed an API/E2E workflow. Those claims remain governed by `VALIDATION.md` and the `integration` environment.
