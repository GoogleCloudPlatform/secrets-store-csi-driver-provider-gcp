# Releasing

Notes on building a release.

1. Create a release branch (`release-X.X`)

    ```bash
    git checkout -b release-X.X
    git push origin release-X.X
    ```

2. Ensure integration tests pass on branch
3. Build a release image

    ```bash
    gcloud builds submit --config scripts/cloudbuild-release.yaml --substitutions=_VERSION=<vX.X.X>,_BRANCH_NAME=<release-X.X> --no-source
    ```

4. Create a PR to the `release-X.X` branch updating the `deploy/` `yaml` file(s) to point to the content addressable sha and update `CHANGELOG.md` (example [tag compare](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/compare/v0.5.0...main))
5. Manually test release image
6. Tag the commit from step 4 as `vX.X.X` by creating a new release
7. Merge changes from `release-X.X` into `main`
8. After the changes are merged, ensure the latest tag is updated for the `manifest_staging` Helm chart, along with the main Helm chart being updated with the previous release tag, once the `Bump Helm Charts Versions` job is completed.

## Release template

* Tag Version: `vX.X.X`
* Target: branch `release-X.X`
* Release Title: `vX.X.X`

```markdown
Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:vX.X.X`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:vX.X.X`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:vX.X.X`

Digest: `sha256:<sha>`

See CHANGELOG.md for more details.
```

## Fixes

If a release needs a fix, commit the fixes to the release branch and start from
step 2, incrementing the minor version number.
