# Releasing

Notes on building a release.

1. Create a release branch (release-x.x)
2. Ensure integration tests on branch
3. Build a release image

    ```bash
    gcloud builds submit --config scripts/cloudbuild-release.yaml --substitutions=_VERSION=<VERSION>,_BRANCH_NAME=<branch> --no-source
    ```

4. Update `deploy/` `yaml` files to point to the content addressable sha
5. Tag the commit from step 4 as `vx.x.0`

## Fixes

If a release needs a fix, commit the fixes to the release branch and start from
step 2, incrementing the minor version number.
