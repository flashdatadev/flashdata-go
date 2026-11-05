# Releases and packages

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

Install with `go get github.com/flashdatadev/flashdata-go@v1.2.0` (Go 1.25+).
Version 1.2.0 adds multiple transcript/caption languages and per-language results.
Read the [release notes](release-notes-1.2.0.md). Existing single-language calls and
JSON response defaults remain compatible.

The [GitHub release](https://github.com/flashdatadev/flashdata-go/releases/tag/v1.2.0)
includes the canonical Go module ZIP, `.mod` and `.info` files, both READMEs,
release notes, a build report and `SHA256SUMS`. The tag, module archive and attached
documentation identify the same source commit. Go modules are distributed through
repository tags and normal Go tooling; no separate registry upload is required.

## Maintainer checks

Run `go test ./...`, `go vet ./...`, source generation and package-policy checks.
`python scripts/verify_package.py --go /path/to/go --dependency-cache /path/to/module/cache`
checks an isolated packaged consumer and cross-builds it for all five supported targets.
That synthetic local check disables the checksum service only inside its isolated
file proxy. Public release downloads use normal Go checksum verification.

GitHub CI checks Linux/Windows with Go 1.25/1.27, Linux race checks, generation,
package contents and consumer builds. Verify the public module archive against the
tagged Git tree before attaching it to a release. Keep existing tags and assets
unchanged. Publishing the SDK does not deploy backend services.
