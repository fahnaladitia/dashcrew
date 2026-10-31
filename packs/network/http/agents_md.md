---
target: agents_md
order: 320
rules: [network.http-one-client, network.http-platform-client-in-factory, network.http-retry-client, network.http-close-client]
---
## Network: http

- Requests go through one configured `http.Client`, built by a factory function in `{{pack.network.client_dir}}` and passed to repositories through constructors. Never call the top-level `http.get` or `http.post` from app code.
- A platform client (`CronetClient`, `CupertinoClient`) is chosen in that factory only, never at a call site.
- Retries wrap the client in `RetryClient`; never write a retry loop around a request.
- The owner of a client closes it when it is disposed.
