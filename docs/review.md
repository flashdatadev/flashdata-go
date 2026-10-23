# Go SDK review

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

This records the local review before initial GitHub source publication. Current workflow results are available in [GitHub Actions](https://github.com/flashdatadev/flashdata-go/actions). Version tags, GitHub Releases and backend deployment are separate from the source push.

The Go SDK remains a local 1.0.0 candidate. This review compared the source catalog, bulk options, agent registration policy and response behavior with the local backend, and checked parity with the Python and TypeScript SDK snapshots. All 15 source schemas and the original 24 REST operations are retained. Go additionally exposes verified key identity and the eight machine-side onboarding methods; browser-session operations stay in the FlashData website.

## Response-format review

Current source adds `WithFormat` methods while preserving existing typed JSON
signatures and serialization. The review checks all 14 realtime sources, all 15
stored-result sources, generated background helpers, receipt/status envelopes,
metadata, recovery IDs, structured errors, UTF-8, gzip bounds and cancellation.
JSON tags on existing models remain compatible with v1.0.0.

The full suite executes the new command-line example and the response-format guide's
Go recipes. Stored-result fixtures use completed job envelopes, matching IDs and
completion/expiry metadata. See [response formats](response-formats.md).

## Initial publication fixes

- Authentication mode is independent of the proof value. A proof whose literal value is `key` uses only `Authorization: Bearer`; it cannot select the client's API-key header.
- Signed storage errors redact decoded and escaped signature/credential/token values, including response metadata. Storage, service-account and signed-upload models redact ordinary diagnostic formatting while retaining their JSON wire representation.
- Cancellation and request deadlines survive compressed part reads, gzip header parsing and decompression. CRLF separators do not consume the JSON row length limit; overflowing limit options are rejected before dispatch.
- Manifests must identify the requested bulk job. Query responses must match the submitted source; batches must return distinct job IDs. Malformed mutation responses remain uncertain and are never retried. `SubmissionUnknownError.JobIDs` preserves any valid recovered IDs for reconciliation.
- Identity, registration, connection, claim and handoff responses reject inconsistent ownership, generation or timestamp metadata. This validates response structure; the server still authorizes every operation, and applications must perform the documented installation and recurring-work checks.
- Background and bulk examples reserve recovery paths with exclusive creation before submission, sync accepted IDs before continuing, and refuse to overwrite prior records. Bulk uploads use the exact validated input snapshot. Executable regressions cover a repeated invocation, an orphaned recovery record and recovery after a failed start.
- The English and Chinese READMEs share the complete executable bulk sequence. Documentation distinguishes ID-only example recovery from application-owned secure persistence of signed upload capabilities.

## Reproduce the checks

Run `go test ./...` and `go vet ./...` with Go 1.25 or newer. The full test suite builds and executes all 22 shipped examples and all four Go programs across the two READMEs against loopback fixtures. Short mode deliberately skips these subprocess checks.

For a local backend checkout with its dependencies installed, `node scripts/check_backend_contracts.cjs /path/to/backend` compares all source schemas with the backend catalog and validates the five bulk sources plus agent registration. This is an optional development check; Node and the backend are not SDK runtime dependencies. It sends no requests and does not open databases.

Use the [module archive verifier](release.md) for a clean consumer installation and five platform cross-builds. See [validation evidence](validation.md) for the exact local results and platform limitations.

See the [joint release-readiness check](release-readiness.md) for response-type recovery, confirmation consistency and package-content improvements.

## Remaining deployment work

The identity endpoint is still a local backend patch awaiting service deployment. Agent onboarding depends on the deployment's feature flag. Live credentials, cloud-provider permissions, callback delivery and browser handoff need validation against an authorized deployment. The SDK does not create a vault or manage durable application state; the local CLI supplies those application-level facilities. GitHub publishing and tagging remain separate authorized actions.
