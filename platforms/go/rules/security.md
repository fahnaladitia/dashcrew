---
target: agents_md
order: 72
rules: [go.security-parameters, go.security-secrets, go.security-logs, go.security-authz, go.security-timeouts, go.security-body-limit]
---
## Security

- Queries take parameters (`$1`, `?`, the ORM's placeholders). Never build SQL with `fmt.Sprintf` or string concatenation from input.
- No key, token or password in source or in committed config; secrets arrive through the environment or a secret store.
- Never log tokens, passwords, full request bodies or personal data.
- Every handler that reads or changes a resource checks that the caller may do so, inside the handler or a middleware that provably runs for it. Authentication alone is not authorization.
- Servers set read, write and idle timeouts (`ReadHeaderTimeout` at least); outgoing HTTP clients set a timeout.
- Request bodies are limited in size (`http.MaxBytesReader` or the framework's limit) before decoding.
- The `backend-security` skill holds the review checklist.
