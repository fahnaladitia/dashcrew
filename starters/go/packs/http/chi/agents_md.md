---
target: agents_md
order: 310
rules: [http-chi.usage]
---
## HTTP: chi

- Routes and middleware live in one router setup; group with `r.Route` and `r.With`, read path values with `chi.URLParam(r, "id")`.
- Middleware is plain `func(http.Handler) http.Handler`, so net/http middleware works unchanged.
- Pass `r.Context()` down to the service; never store a request in a struct.
