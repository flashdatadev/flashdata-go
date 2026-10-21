# Source publication and later tagged release

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

The public module is `github.com/flashdatadev/flashdata-go`. Source is hosted in [the official GitHub repository](https://github.com/flashdatadev/flashdata-go); the development version is 1.0.0. No version tag or GitHub Release has been created. Go consumers can use `go get github.com/flashdatadev/flashdata-go@latest`; before a stable tag exists, the resolved dependency is a commit-based pseudo-version. Keep that version and checksum in your application's module files.

[GitHub Actions](https://github.com/flashdatadev/flashdata-go/actions) runs tests and vet on Linux/Windows with Go 1.25/1.27, Linux race checks, generation/format checks, package-policy tests and five platform consumer builds. These workflows do not publish releases or registry packages.

The standalone CLI uses a pinned SDK commit without a local module replacement. SDK development can use an explicit `go.work` workspace. Source publication does not deploy identity/onboarding APIs or enable server features; see [contract status](api-contract.md).

Before a later v1.0.0 release, inspect API compatibility, current CI, service availability, docs and package checks. Create a version tag and release only after explicit release authorization. Go versions are normally distributed from repository tags/commits through the Go module proxy; there is no PyPI/npm-style upload step.

The local module verifier creates a synthetic v1.0.0 file-proxy candidate only: `python scripts/verify_package.py --go /path/to/go --dependency-cache /path/to/verified/module/cache`. It never uploads or creates a Git tag. Its `GOSUMDB=off` setting applies only to that isolated synthetic consumer; normal public-module installs retain checksum verification.

`release-files.json` selects reviewed source files for the verifier. It rejects unsafe paths and excludes private state, secrets, caches and toolchains. Package policy tests run with `python -m unittest discover -s scripts -p "test_*.py"`. ZIP metadata is deterministic, and the verifier builds the packaged consumer for all five supported targets.
