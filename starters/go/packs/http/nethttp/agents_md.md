---
target: agents_md
order: 310
rules: [http-nethttp.usage]
---
## HTTP: net/http

- Register routes with method and pattern: `mux.HandleFunc("GET /orders/{id}", h)`; read wildcards with `r.PathValue("id")`.
- Middleware is `func(http.Handler) http.Handler`; wrap the mux once, in one place.
- Pass `r.Context()` down to the service; never store a request in a struct.
