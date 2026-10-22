# Python, TypeScript and Go coverage

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

See the [current SDK and CLI comparison](sdk-parity.md) for all language entry points, README workflows and recovery behavior.

| Capability | Python 1.0.0 | TypeScript 1.0.0 | Go 1.0.0 |
| --- | --- | --- | --- |
| Existing developer REST operations | 24 | 24 | Same 24 |
| Source schemas | 15 | Same | Byte-equivalent parsed source schemas |
| Realtime / async / batch | 14 / 15 / 15 sources | Same | Same; typed `*Async` helpers separate return types |
| Google / YouTube helpers | Six + pagination / nine | Same | Same plus explicit async helper names |
| Standard job methods | Get, wait, result | Same | Same |
| Bulk lifecycle/list/results | 15 methods | Same | Same plus reader-based Upload |
| Account / public methods | 5 / 4 | Same | Same plus verified Identity |
| Agent onboarding | Not part of the released SDK snapshot | Same | Eight machine-side methods, protocol 2 |
| Optional provider payload hints | TypedDict | TypeScript payload types | Generated optional Go structs and DecodeResults |
| Raw future fields and large numbers | Python JSON values | JS number precision constraints | json.RawMessage and json.Number preserve numbers |
| Cancellation | Native sync/async conventions | AbortSignal | context.Context; accepted work continues |
| Response / row default limits | 64 MiB / 16 MiB | Same | Same, also bounds compressed parts |
| Submission retry | None | None | None |
| Readme languages | English + 简体中文 | Same | Same, code blocks are executed offline |

The Go schema inputs come from the existing TypeScript source snapshot. Core response models share that snapshot. Provider payload hints are generated from the same sanitized examples and remain optional, not a guarantee that provider data has a fixed shape. Original results remain available for fields not in a hint.

Go uses `time.Duration`, pointer optional parameters, reusable concurrent clients, error values, and `iter.Seq2` for bounded pagination/streaming. It does not imitate Python's dynamic arguments or TypeScript's mode-overloaded return types. Generic `Queries.Submit(input.Request())` and source-specific `*Async` helpers both accept all supported sources. The SDK owns HTTP only; the local FlashData CLI owns profiles, credential vault, run journal, fresh-process install checks, output files and scheduling.

Onboarding documentation includes the browser-owned operations for contract completeness, but the SDK exposes only the eight machine-side operations. No browser authentication bypass is added. See [contract status](api-contract.md) and [onboarding](onboarding.md).
