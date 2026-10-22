# Changelog

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

## Unreleased

Add English/Chinese workflow selectors, an uploaded-caption recipe, MP4 bulk option-file support in the runnable example, and a shared SDK/CLI comparison. The current examples are fetched from main; the SDK API remains compatible with module v1.0.0.

## 1.0.0

Initial Go SDK with all 15 sources; realtime, async and batch; standard and bulk jobs; signed uploads and streaming results; account/public reads; verified API-key ownership; protocol-2 machine onboarding; typed inputs, response models and provider payload hints; English/简体中文 documentation; runnable examples and offline tests. The v1.0.0 GitHub Release includes the canonical Go module archive, module metadata, current English/Chinese READMEs, release notes, build report and checksums.

Review fixes: isolated bearer authentication from key authentication, redacted signed-storage credentials and diagnostic formatting, preserved cancellation during bulk reads, checked response identity/generation and retained valid recovery IDs on inconsistent submission responses. Examples now reserve recovery paths before submission, sync accepted IDs and upload validated snapshots. Added boundary/recovery regressions and a backend contract checker.

Added public offline source-schema, origin and request validation helpers shared by the local FlashData CLI; submission validation still uses the same checks.

Final joint check: retained candidate recovery IDs across SDK/CLI wire errors; partial-export errors; lost ACK/start recovery and spent upload cleanup; generation consistency; reviewed package manifests, deterministic archives, dependency notices and reproducible package checks. Source is available on GitHub; no version tag, release or backend deployment is performed by the source push.
