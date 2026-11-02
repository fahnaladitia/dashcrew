---
target: agents_md
order: 310
rules: [http-gin.usage]
---
## HTTP: Gin

- Decode with `c.ShouldBindJSON` (or `ShouldBind...`) and answer the error yourself; `c.BindJSON` writes a 400 and aborts on its own.
- Pass `c.Request.Context()` to the service, not the `*gin.Context`. A goroutine that needs request data gets `c.Copy()`, never `c`.
- Map service errors to status codes in one middleware or helper, not in each handler.
- Production runs with `gin.SetMode(gin.ReleaseMode)` (or `GIN_MODE=release`).
