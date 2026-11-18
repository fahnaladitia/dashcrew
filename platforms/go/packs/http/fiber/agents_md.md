---
target: agents_md
order: 310
rules: [http-fiber.usage]
---
## HTTP: Fiber

- Values from the context (`c.Params`, `c.Query`, `c.Body`) are only valid inside the handler: copy them (`strings.Clone`, `bytes.Clone`) before keeping them or handing them to a goroutine, unless the app sets `Immutable`.
- net/http middleware does not apply directly; use Fiber middleware or the adaptor package.
- Pass the request's `context.Context` from Fiber (v2: `c.UserContext()`) to the service, never the `*fiber.Ctx`.
