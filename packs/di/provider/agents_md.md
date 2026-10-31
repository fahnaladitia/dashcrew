---
target: agents_md
order: 310
rules: [di.provider-create-above-users, di.provider-proxy-for-dependencies, di.no-construct-in-widgets]
---
## Dependency injection: provider

- Repositories and services are created in `create` (`Provider(create: (_) => OrderRepository())`) above the screens that use them, grouped in `MultiProvider`.
- A dependency built from another uses `ProxyProvider`; a model gets its dependencies through `context.read` in its own `create`.
- Never construct a repository or service inside a widget, a model or a state holder.
