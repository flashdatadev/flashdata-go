# API reference

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

All network methods accept `context.Context`. Reuse a Client; explicitly pass `Options.APIKey` for authenticated APIs. Zero Options.APIKey is permitted for public/proof-authenticated calls only.

| Resource | Methods |
| --- | --- |
| Queries | Realtime, Submit, Batch |
| Google | Search, Images, News, Shopping, Videos, Places, Pages; six Async helpers |
| YouTube | Search, SearchExtended, Metadata, Transcript, Captions, LicenseCheck, Channel, Suggestions; matching Async helpers; Download |
| Jobs | Get, Wait, Result |
| Bulk | Create, Upload, UploadFile, Get, List, IterJobs, Update, Start, Pause, Resume, Cancel, Wait, Manifest, Part, IterRows, Rows |
| Account | Access, Identity, Balance, Pricing, Limits, Usage |
| Public | Sources, Plans, Health, Status |
| Onboarding | Capabilities, Register, TemporaryStatus, Identity, Status, CreateClaim, Handoff, Acknowledge |

Use `go doc .` or `go doc .BulkService` from this module for exact Go signatures. The generated response types retain wire field tags; generic provider/result objects preserve raw JSON. `Storage` and `ServiceAccount` alias the download destination models. See [sources](sources.md), [bulk](bulk.md), [onboarding](onboarding.md) and [error semantics](#errors-timeouts-and-http-transport).

Offline helpers are available to applications such as the CLI: `Sources()` returns the supported source identifiers, `QuerySchema(source)` returns an independent copy of a source's base input schema (callback, mode and strict root-field policy are additionally enforced by `ValidateQuery`), and `ValidateQuery(query, asynchronous)`, `ValidateBulkCreate(request)` and `ValidateRegistration(request)` use the same validation as network submissions without sending a request. `NormalizeOrigin(origin)` applies the SDK's service-origin validation. These checks validate request shapes; they do not verify credentials, account permissions, credits or server admission.

`SubmissionUnknownError.JobIDs` preserves valid job IDs from inconsistent async, batch or bulk responses. Reconcile these candidates using the original account; the error still means the complete submission outcome is unknown. Normal Go formatting redacts storage and signed-upload credentials; JSON remains suitable for requests and caller-owned secure persistence.

## Response formats

Existing methods return typed JSON. Use `WithFormat` methods to choose headers per
call and receive a `FormattedResponse`: raw JSON is in `JSON`, plain text in `Text`,
and `DecodeJSON(&model)` decodes JSON.

| Methods | Options |
| --- | --- |
| `Queries.RealtimeWithFormat`, all 14 realtime source `WithFormat` helpers, `Jobs.ResultWithFormat` | `ContentOptions{Accept, RespondWith}` |
| `Queries.SubmitWithFormat`, `Queries.BatchWithFormat`, all 15 background source `WithFormat` helpers, `Jobs.GetWithFormat` | `ResponseOptions{Accept}` |

`AcceptJSON` is the default; `AcceptText` selects plain text independently.
`RespondMarkdown` replaces `results` with Markdown `content` in JSON, or returns
metadata followed by Markdown as text. `Jobs.GetWithFormat` preserves the outer
`job` field; `Jobs.Get` unwraps it. Polling, pagination, Bulk, account, public and
onboarding keep their formats. See the [format guide](response-formats.md).

## REST operations

| Method | Path | Operation |
| --- | --- | --- |
| GET | `/health` | `getHealth` |
| GET | `/status` | `getPublicStatus` |
| GET | `/catalog/sources` | `listPublicDataSources` |
| GET | `/plans` | `listPlans` |
| GET | `/api-access` | `getApiAccess` |
| GET | `/api-access/balance` | `getApiAccessBalance` |
| GET | `/api-access/pricing` | `getApiAccessPricing` |
| GET | `/api-access/limits` | `getApiAccessLimits` |
| GET | `/usage` | `getApiKeyUsage` |
| GET | `/jobs/{jobId}` | `getConsoleJob` |
| GET | `/bulk-jobs` | `listBulkJobs` |
| POST | `/bulk-jobs` | `createBulkJob` |
| GET | `/bulk-jobs/{bulkId}` | `getBulkJob` |
| PATCH | `/bulk-jobs/{bulkId}` | `updateBulkJob` |
| DELETE | `/bulk-jobs/{bulkId}` | `cancelBulkJob` |
| POST | `/bulk-jobs/{bulkId}/start` | `startBulkJob` |
| POST | `/bulk-jobs/{bulkId}/pause` | `pauseBulkJob` |
| POST | `/bulk-jobs/{bulkId}/resume` | `resumeBulkJob` |
| GET | `/bulk-jobs/{bulkId}/results` | `downloadBulkJobManifest` |
| GET | `/bulk-jobs/{bulkId}/results/{partNumber}` | `downloadBulkJobPart` |
| GET | `/jobs/{jobId}/results` | `downloadJobResult` |
| POST | `/v1/queries/realtime` | `queryRealtime` |
| POST | `/v1/queries` | `queryAsync` |
| POST | `/v1/queries/batch` | `queryBatch` |
| GET | `/api-access/identity` | `getApiKeyIdentity` |
| GET | `/agent-connections/capabilities` | `getAgentCapabilities` |
| POST | `/agent-connections/registrations` | `registerAgentConnection` |
| GET | `/agent-connections/owned` | `listOwnedAgentConnections` |
| GET | `/agent-connections/{connectionId}` | `getTemporaryAgentConnection` |
| DELETE | `/agent-connections/{connectionId}` | `revokeAgentConnection` |
| GET | `/agent-connections/{connectionId}/identity` | `getAgentConnectionIdentity` |
| GET | `/agent-connections/{connectionId}/onboarding` | `getAgentOnboardingStatus` |
| POST | `/agent-connections/{connectionId}/claims` | `createAgentClaim` |
| POST | `/agent-connections/claims/{claimId}/confirm` | `confirmAgentClaim` |
| GET | `/agent-connections/claims/{claimId}` | `getAgentClaim` |
| POST | `/agent-connections/claims/{claimId}/approve` | `approveAgentClaim` |
| POST | `/agent-connections/{connectionId}/handoff` | `deliverAgentHandoff` |
| POST | `/agent-connections/{connectionId}/handoff/ack` | `acknowledgeAgentHandoff` |

## Errors, timeouts and HTTP transport

Use `errors.As` for `APIError`, `ValidationError`, `SubmissionUnknownError`, `PollingTimeoutError`, `PollingInterruptedError`, `JobPausedError`, `JobFailedError` and `ProtocolError`; use `errors.Is` for context cancellation/deadlines and `ErrResponseTooLarge`. `APIError` includes redacted message/code, status, request ID and Retry-After. Decide any retry policy explicitly; do not blindly retry mutations or paid realtime calls.

Clients are reusable and safe for concurrent calls. Every operation takes `context.Context`. Default request timeout is 120 seconds; job polling is 5 minutes/2-second intervals and bulk polling 1 hour/5-second intervals. Uploads have a 15-minute request budget. Caller deadlines and an injected `HTTPClient.Timeout` can shorten those budgets. Inject an HTTP client/transport for proxies or instrumentation; cookies and automatic redirects are disabled on the client's copy. Avoid credential-adding custom transports on signed storage requests.

Use HTTPS origins; HTTP is permitted only on loopback for tests. Production origins are `https://data.flashdata.dev` and `https://api.flashdata.dev`; management methods append `/v2`. Signed result origins are discovered through authenticated account metadata, validated exactly, and contacted without keys or cookies. Caller-supplied `ResultOrigin` is an explicit trust override. Storage errors redact bare signed-URL credentials as well as complete URLs. Client, onboarding credential, storage and signed-upload models redact normal Go diagnostic formatting; JSON serialization intentionally retains wire credentials and must never be logged.
