# v1.0.0 release and packages

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

The public module is `github.com/flashdatadev/flashdata-go`. The `v1.0.0` source tag identifies this version. Install it with `go get github.com/flashdatadev/flashdata-go@v1.0.0` and retain `go.mod`/`go.sum`. Go versions are distributed from repository tags through Go tooling; there is no separate registry upload. The [formal GitHub Release](https://github.com/flashdatadev/flashdata-go/releases/tag/v1.0.0) provides downloadable module files and documentation.

The release includes the canonical Go module ZIP, its `.mod` and `.info` metadata, [release notes](release-notes-1.0.0.md), current English/Chinese READMEs, a build report and `SHA256SUMS`. The module ZIP comes from a normal checksum-verified public `go mod download`, and its files are compared with the tagged Git tree. SHA-256 checksums cover every attached file except the checksum list itself. GitHub's Go module checksum and archive SHA-256 serve different purposes; both are recorded in the build report.

[GitHub Actions](https://github.com/flashdatadev/flashdata-go/actions) runs tests and vet on Linux/Windows with Go 1.25/1.27, Linux race checks, generation/format checks, package-policy tests and five-platform consumer builds. These workflows do not publish GitHub Releases or upload registry packages.

The CLI pins SDK v1.0.0 without a local replacement. SDK development can use an explicit `go.work` workspace. Publishing the SDK does not deploy identity/onboarding APIs or enable server features; see [contract status](api-contract.md). The v1.0.0 tag and canonical module ZIP remain unchanged. The build report records the original source commit and the separate documentation commit, since installation documentation was finalized after tagging.

The local module verifier remains available: `python scripts/verify_package.py --go /path/to/go --dependency-cache /path/to/verified/module/cache`. It creates a synthetic file-proxy candidate, never uploads or creates a Git tag, and builds/runs a clean consumer plus five cross-builds. Its `GOSUMDB=off` setting applies only to that isolated synthetic consumer; normal public-module installs retain checksum verification.

`release-files.json` selects reviewed source files for the verifier. It rejects unsafe paths and excludes private state, secrets, caches and toolchains. Run package-policy tests with `python -m unittest discover -s scripts -p "test_*.py"`. ZIP metadata is deterministic.
