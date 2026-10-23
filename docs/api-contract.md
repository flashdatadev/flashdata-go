# API contract status

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

This checkout resolves two local contract gaps needed before a Go-based CLI can safely support onboarding and recurring work:

1. The existing protocol-2 agent connection routes were absent from the developer REST contract. Their machine and browser operations, credential boundaries, registration proof, ownership, handoff, installation and availability states are now described in the authoritative management OpenAPI file and the SDK snapshot.
2. An arbitrary imported API key could report sources and prices, but not authoritative ownership. The local backend now implements GET `/v2/api-access/identity` using current MongoDB key/account/verified-owner/connection state. The unchanged GET `/api-access` response still exposes only sources/result origin.

The Go module snapshot `internal/contract/openapi.json` contains the existing 24 operations plus 14 additional identity/onboarding operations: eight machine onboarding, five browser-owned operations and one key identity operation. Every operation describes its actual header/session requirements. Data and management origins remain separate. `internal/contract/onboarding-contract.json` isolates the additional contract for future SDK parity work.

## Response formats

The current source snapshot includes independent `Accept` negotiation for realtime,
submission, batch, job-status and completed-result responses. `X-Respond-With`
selects Markdown only for realtime and completed results. Existing SDK methods
remain typed JSON; see [response formats](response-formats.md) for `WithFormat`
methods and examples. These additions are available on `main`, after v1.0.0.

Maintainers with a local service checkout can verify the five affected operations
and all-source fixtures without API calls:

```bash
node scripts/update_response_contract.cjs /path/to/public-openapi.generated.json /path/to/backend/http/responseFormatting.js --check
```

## Initial identity/onboarding deployment notes

The notes below record the original identity/onboarding integration, independently
of response formats. Check the target deployment's capabilities before onboarding.

All changes are local. The identity endpoint needs backend deployment before a live client can use it. Agent onboarding already has local route/service implementations but is feature-gated and disabled unless the server is configured for it. The SDK returns the resulting API error; it does not enable a feature, create accounts on failure, invent ownership, or substitute an OAuth credential.

The parent backend patch is separate from this standalone SDK repository: `backend/modules/access/apiKeyIdentity.js`, API access routing, an API-key lookup index on AgentConnection, focused tests, and `docs/v2/openapi.yaml`. Review and deploy those changes using the service's normal rollout process. None was pushed or deployed by this task.

Identity is a point-in-time readiness read. It is not an authorization token or an exactly-once guarantee. Data requests still undergo server authorization/admission. Redis key projections may lag, so the new identity read rejects revoked/expired current records and intersects projected scopes with current scopes. Temporary identities never allow recurring tasks; a handoff owner key remains ineligible until installation ACK. A verified ordinary owner key does not need fabricated onboarding state.
