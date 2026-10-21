# Onboarding integration

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

Existing owners normally use `Options{APIKey: os.Getenv("FLASHDATA_API_KEY")}`. The SDK does not create a trial unless the caller explicitly invokes `Onboarding.Register`. No environment files, profiles or credential vaults are created by the SDK.

## Protocol and credential boundary

Read `Onboarding.Capabilities` first. Require `protocolVersion == 2` and `available == true` before offering registration. A feature-disabled deployment can return 404; do not fall back to an older registration flow. Capability discovery is public and free. The server currently limits temporary onboarding to Google Search and YouTube Search, Metadata, Transcript and Captions: 1 Credit, 72 hours, 1 request/second and batch size 1. Per-source request counts are display metadata derived from current prices; do not treat them as a refill or guarantee.

| Operation | Credential | HTTP |
| --- | --- | --- |
| `Capabilities` | None | GET `/v2/agent-connections/capabilities` |
| `Register` | Saved 43-character registration proof | POST `/v2/agent-connections/registrations` |
| `TemporaryStatus` | Setup credential | GET `/v2/agent-connections/{id}` |
| `Identity` | Current ordinary data API key | GET `/v2/agent-connections/{id}/identity` |
| `Status` | Handoff proof | GET `/v2/agent-connections/{id}/onboarding` |
| `CreateClaim` | Setup credential | POST `/v2/agent-connections/{id}/claims` |
| `Handoff` | Handoff proof | POST `/v2/agent-connections/{id}/handoff` |
| `Acknowledge` | Handoff proof; installed API key in body | POST `/v2/agent-connections/{id}/handoff/ack` |

These machine-side routes use purpose-specific `Authorization: Bearer` headers and reject `X-API-Key`. Normal data, job and account APIs use `X-API-Key`. These proofs and ordinary data keys are separate from MCP OAuth access tokens.

## Durable registration and recovery

1. Obtain explicit intent to start an anonymous trial. Generate a proof with `NewRegistrationProof()` using 32 random bytes. Build a `RegistrationRequest` containing the chosen name, scopes, optional audience/client/transport.
2. Atomically save the proof **and exact request** in an application-owned secure store before calling Register. Never put the proof in logs, command-line arguments or browser URLs. Client/transport labels are self-reported metadata, not authorization.
3. Call `Register(ctx, proof, request)`. Save the returned connection ID, API key, setup credential, handoff proof, generation and data origin securely. Do not log JSON registration responses; credentials serialize intentionally for caller-controlled storage. Standard String/GoString formatting redacts the credential-bearing response types.
4. If the request outcome is uncertain, load the same saved proof and identical request. Explicit recovery with the same input returns the same trial while it remains recoverable. A changed body conflicts; a new proof can create another trial and must not be used as a retry strategy. There is no automatic refill or submission replay.
5. Validate the returned `DataOrigin` against the application's trusted service configuration before creating another client. The SDK never silently changes origins or installs returned credentials.

The caller-owned store must implement durable atomic writes, permissions/encryption appropriate for the OS, crash recovery, and process coordination. Filesystem persistence belongs to an application or the local FlashData CLI. `os.WriteFile` alone is not an implementation of a secure vault, especially on Windows.

## Browser claim and owner-key handoff

1. Call `CreateClaim` with the stored setup credential. Build a browser URL using `ClaimURL("https://flashdata.dev", claim)` and show the eight-digit confirmation code separately. The path is validated; no proof, API key or code is placed in the URL.
2. The human signs in, verifies the code, reviews scopes and explicitly approves account-balance funding in the FlashData browser UI. Session cookies, Origin checks and CSRF protection stay in that UI. Browser approval cannot be performed with a setup proof. The Go SDK intentionally exposes no browser-cookie impersonation helpers.
3. Read `Status` with the handoff proof. Respect `pollAfterSeconds` and apply an application deadline. `awaiting_claim` and `claim_processing` are pending; `handoff_ready` permits retrieval; `installed` means server ACK is already complete. `expired`, `revoked`, `blocked` and `handoff_expired` require the indicated next action rather than another temporary registration.
4. Call `Handoff`. A delivered response contains generation >=2 and the candidate owner API key. It has a 24-hour installation window. A repeated recovery retrieves the same candidate while available; a confirmed response has no key material.
5. Atomically persist the candidate owner key and generation before any ACK. Launch a fresh process, reload it from the application's durable store, and call `Onboarding.Identity(connectionID, installedKey)` with that loaded value. Require matching connection ID, ownership `owned`, lifecycle `valid`, access `ready` and matching generation. At this point installed may still be false: ACK follows verification.
6. Only after that independent verification, call `Acknowledge(connectionID, handoffProof, installedKey)`. The server validates the candidate, marks installation, removes its temporary expiry and deletes recoverable secret material. The SDK cannot prove a fresh process was used; the caller must enforce this invariant.
7. Verify again in a new read. Require `Account.Identity().RecurringAllowed` before enabling recurring work. Retain only the minimum local recovery state necessary after confirmed installation.

If a candidate is saved but ACK fails, keep it and recover using the saved handoff proof and generation. Never discard a valid owner key and create a new trial to repair local state. If the proof expires before ACK, follow the server's reconnect action. Cancellation cannot undo a server-side claim or key delivery.

## General key import and recurring tasks

`Account.Identity` authenticates an explicitly supplied key, then verifies current key/account/owner/connection records. It returns public IDs, ownership, ready/blocked access, installed status, scoped sources, expiry and a verification timestamp. Temporary keys always have `recurringAllowed:false`. A verified active human owner key created outside onboarding does not need an invented connection ID or installation marker. A handoff key does require the server installation ACK.

The identity endpoint is a new local backend patch and has not been deployed by this task. Fail closed on 404, dependency failure or malformed data. Revalidate before a recurring execution and before serving a saved result after an access change; do not infer authority from the `fd_` prefix, a plan name, generation or local phase. Server-side query authorization remains authoritative even immediately after a successful identity read.

Run the safe read/claim examples:

```bash
go run ./examples/onboarding
go run ./examples/onboarding -action account-identity
go run ./examples/onboarding -action status -connection acn_YOUR_ID
go run ./examples/onboarding -action claim -connection acn_YOUR_ID
go run ./examples/onboarding -action identity -connection acn_YOUR_ID
```

Supply `FLASHDATA_HANDOFF_PROOF` or `FLASHDATA_SETUP_CREDENTIAL` from your secure store when needed; `FLASHDATA_API_KEY` is passed explicitly in the example. The runnable example deliberately performs no registration, credential storage or ACK because those require the application's durable-store and fresh-process guarantees. The complete machine lifecycle is exercised by offline tests with the same SDK methods.
