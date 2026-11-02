# quota-reset

This is the cpaDash fork of [quota-activation](https://github.com/Cody292/quota-activation). It uses a live Codex quota probe and can clear CPA's local cooldown after recovery. Its plugin ID is `quota-reset`, so it does not replace the official plugin.

## Automatic releases

The source, registry, and releases are hosted in [cpa-plugin-quota-reset](https://github.com/zzj0311/cpa-plugin-quota-reset). Pushing source to the default branch runs the release workflow: it tests the plugin, chooses the next patch version, builds the Linux AMD64 plugin archive, publishes the release, and updates `registry.json` after the assets are available. One push produces one release for its pushed revision, even when the push contains multiple commits.

The major and minor version are maintained as `RELEASE_MAJOR` and `RELEASE_MINOR` in `.github/workflows/release.yml`; edit those values when moving to a new version series. The workflow chooses the next available patch for that series based on existing version tags. Release retries reuse the tag for the same commit rather than replacing an existing release.

The CPA store source remains the raw `registry.json` on the repository's default branch. Once the workflow finishes, refresh the plugin store and install or upgrade `quota-reset` from the dashboard. CI publishes releases; it does not automatically install them into CPA.

Private repositories require CPA to have a configured GitHub store token; keep credentials out of the repository.
## Local build

Run `./scripts/build-local.sh` to build `dist/quota-reset.so`. The GitHub Actions workflow is the normal release path; local builds are useful for development.
