# Umbrel app package stubs for OrdexCoin.

These files are the starting point for the Umbrel App Store package. See
`docs/UMBREL.md` for the full, resumable integration guide. Nothing here is
final — fields marked TODO must be filled in, and the image must be published
and pinned by digest before use.

## To finish

1. Publish the combined image (`docker buildx build -f webapp/Dockerfile .`)
   to a registry and paste the `sha256:<digest>` into `docker-compose.yml`.
2. Fill in every `TODO` in `umbrel-app.yml`.
3. Add a square `icon.svg` and 3–5 gallery screenshots.
4. Test via a community app store repo before submitting to `getumbrel/umbrel-apps`.

Target platform is **umbrelOS on x86_64** — the amd64 image is ready to go.
ARM64 (Raspberry Pi) needs an aarch64 daemon build and a multi-arch manifest;
that is a separate future task (see `docs/UMBREL.md`).

The image and webapp are verified: `./webapp/test-docker-web.sh` builds the
image, starts a fresh container and runs 23 checks against the web UI and daemon
(fresh node → create wallet → generate address → all endpoints). Run it before
packaging any change.
