# Local validation evidence

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

This records the local review before initial GitHub source publication. Current workflow results are available in [GitHub Actions](https://github.com/flashdatadev/flashdata-go/actions). Version tags, GitHub Releases and backend deployment are separate from the source push.

Final review verified on Windows amd64 on 2026-10-22. All work remains local. No repository push, tag, registry publication, backend deployment, paid API request, trial registration or customer-cloud write was performed. Python and TypeScript repositories were left unchanged. See the [review fixes](review.md) and [joint release-readiness report](release-readiness.md).

| Check | Result |
| --- | --- |
| Go 1.27.3 complete suite | 158 tests/subtests pass, zero failures or skips; all source packages build |
| Go 1.25.0 minimum-version compatibility | Same complete suite: 158 tests/subtests pass, zero failures or skips |
| Static analysis | go vet passes on both Go versions; Go formatting and Python script lint/format pass |
| Executable examples | All 22 shipped programs build and run against loopback mock APIs on both Go versions |
| English and Chinese READMEs | All four programs build/run offline on both Go versions; translated Go blocks match; bulk programs match the maintained snippet |
| Recovery regressions | Existing and orphaned records block new API calls; accepted bulk IDs survive a start failure and recover without another creation |
| Backend source and workflow parity | All 15 schemas match the backend catalog; five bulk option shapes and registration policy validated offline by scripts/check_backend_contracts.cjs |
| Python / TypeScript parity | All 15 source schemas match both SDK snapshots; source contract and original 24 REST operations match TypeScript |
| Identity / onboarding contract | All 14 added operation IDs, methods and paths match authoritative management YAML |
| Generated source | Both Go files reproduce byte-for-byte in an isolated directory; 15 source input models and 94 response/parameter models |
| Dependent CLI | Complete offline CLI suite passes: 85 tests/subtests, zero failures or skips; fixtures use realistic unique batch IDs and temporary connection metadata |
| Module packaging | Local module zip inspected and installed by an isolated file-only Go proxy; clean consumer builds, runs and passes vet |
| Portability | CGo-disabled consumer cross-builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64 |
| Security and bounds | Proof/key separation, signed read/upload origin pinning, no credential forwarding or mutation retries, signed-secret redaction, formatting redaction with intact JSON, bounded gzip/NDJSON, CRLF limits, malformed authority and response identity, cancellation and request deadlines |
| Source hygiene | No real FlashData key, customer storage file, recovery record or environment file in candidate source; caches, toolchains and outputs are ignored |

The final event reports are `.cache/test-results-readiness-127.jsonl` and `.cache/test-results-readiness-125.jsonl`. The statement coverage profile is `.cache/coverage-readiness.out`: SDK coverage is 85.1%. Example subprocess execution is checked separately and is not merged into this profile. The CLI compatibility report is in the sibling CLI's `.cache/test-results-readiness-127.jsonl`.

The archive verifier saves `dist/package-check/report.json` and a fresh isolated verification directory on every run. These reports and artifacts are ignored by Git and are not published. The earlier local backend identity/access/onboarding patch had 90 passing tests during its implementation; this SDK review did not change backend source or rerun that database-independent backend suite.

## Response-format validation on current source

The `WithFormat` update adds API-serializer fixtures for all 14 realtime and 15
stored-result sources in all four format combinations. Tests cover generated source
helpers, receipts/status, unchanged JSON defaults, recovery IDs, structured errors,
UTF-8 and gzip limits, cancellation, header scope, documentation recipes and the
runnable example. The module verifier also runs the new API from a clean consumer.

With a local service checkout, run `scripts/update_response_contract.cjs` as
described in [contract status](api-contract.md) to verify snapshot/fixture parity.
Node is only needed for that maintenance check. The SDK itself requires Go only.
The initial-publication counts above are historical; use current test output and
[GitHub Actions](https://github.com/flashdatadev/flashdata-go/actions) for current results.

## Practical limits

The new `/v2/api-access/identity` endpoint remains a tested local backend patch awaiting deployment. Agent onboarding is feature-gated; a deployment can return 404. The SDK fails closed rather than inventing ownership. Live credentials, cloud-provider permissions, external callback delivery and browser claim/handoff installation still require validation against an explicitly authorized deployment.

The SDK intentionally does not implement a credential vault or durable run journal. ID-only examples recover started jobs; pre-upload process recovery requires securely persisting the complete signed creation response in the application or using the local CLI.

No Linux/macOS executable was run on this Windows host. Prepared GitHub CI has not run for this unpublished repository. The host has no C compiler, so the race detector was not run locally; the workflow runs it on Linux. Explicit offline tests cover concurrency and cancellation. Cross-built executables from the module verifier are SDK consumers.

## Final joint package and recovery evidence

- Full Go 1.27.3 and 1.25.0 suites passed with no skipped acceptance tests. Reports use `.cache/test-results-readiness-127.jsonl` and `.cache/test-results-readiness-125.jsonl`.
- Compiled-process acceptance covers lost ACK recovery with the same candidate/proof and lost bulk start recovery without repeated create/upload/start; spent upload capabilities are removed after reconciliation.
- Malformed SDK response fields retain candidate IDs through CLI errors and durable uncertain records. Partial exports and per-item failures return nonzero status with error/export metadata on stderr and no success envelope.
- Both repositories pass four archive-policy unit tests (including unsafe-path subcases); reviewed file manifests, deterministic ZIP metadata and copied dependency notices are included. Python lint/format and backend source/bulk/registration contract checks pass.
- All 18 public FlashData website, docs, pricing and source-documentation links returned HTTP 200 on credential-free HEAD checks. This does not exercise paid APIs or account state.
- Final packages are checked against their manifests and SHA-256 checksums. The SDK verifier installs a clean packaged consumer through a file-only proxy and cross-builds all five targets. CLI reports record both runtime source hashes and the build toolchain.
