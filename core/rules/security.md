---
target: agents_md
order: 72
rules: [security.https-only, security.no-hardcoded-secrets, security.sensitive-storage, security.no-sensitive-logs, security.no-pii-analytics, security.tls-validation, security.deeplink-validation]
---
## Security

- HTTPS only; no cleartext endpoint.
- No API key, token or password in source. Sensitive values arrive through the environment or the build configuration{{#project.build_wrapper}} (injected by `{{.}}`){{/project.build_wrapper}}, never as literals.
- Tokens, refresh tokens, PINs and payment data go to secure storage or an encrypted store, never to plain storage. Non-sensitive cache may use plain storage.
- Never log tokens, auth responses or personal data: release logs are readable.
- Never send raw email, phone, full name or birth date as an analytics property; send an enum, boolean, count or year instead.
- Disabled TLS certificate validation is a blocker; a debug-only exception is guarded so it never reaches production.
- Deep-link parameters are untrusted: validate them before navigating or using them as ids, and re-check authorization for protected screens.
