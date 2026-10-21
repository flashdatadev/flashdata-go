# Security

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

Do not put API keys, setup/handoff/registration proofs, signed URLs, cloud credentials or private keys in GitHub issues, source code or logs. Use the support channel available in your [FlashData account](https://flashdata.dev) for private security reports; do not invent a public disclosure of secrets.

The SDK redacts known credential values and common secret patterns from API error text. Raw successful responses, result payloads and explicit JSON serialization can contain sensitive material, especially onboarding responses. The caller controls their storage and logging. Go String/GoString for credential-bearing onboarding response objects redacts the secret values; this does not replace a secure store.

Use trusted HTTPS API origins and a trusted result origin. Custom transports must not inject account credentials into signed storage requests. The SDK disables automatic redirects and cookies on its copied http.Client; it does not modify or take ownership of an injected transport. Context cancellation is local and does not revoke an accepted server job or key.
