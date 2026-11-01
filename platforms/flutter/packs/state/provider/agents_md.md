---
target: agents_md
order: 300
rules: [provider.create-not-value, provider.multiprovider-at-owner, provider.watch-read-select, provider.proxy-for-dependencies, provider.no-mutation-during-build, provider.scoped-rebuilds, doc.required.provider-models]
---
## State: provider

- Models are `ChangeNotifier`s in `{{pack.state.feature_dir}}`, created inside `create` (`ChangeNotifierProvider(create: (_) => OrderModel())`). Use `.value` only to pass on an object that already exists, never to create one.
- The screen that owns a model provides it; group several with `MultiProvider`.
- In `build`, use `context.watch<T>()` or `context.select`; in callbacks and `initState`, use `context.read<T>()`. Never `read` in `build`, never `watch` in `initState`.
- A model built from other providers uses `ProxyProvider` or `ChangeNotifierProxyProvider`, never `create` reading a value that changes.
- Never change a model while the tree builds: start a load in the model's constructor or `create`, or with `Future.microtask` in `initState` when it needs a parameter.
- Limit rebuilds with `Consumer` or `Selector` and their `child` argument.
- Every model class carries Dartdoc.
- Model shape, disposal and tests: skill `provider-patterns`.
