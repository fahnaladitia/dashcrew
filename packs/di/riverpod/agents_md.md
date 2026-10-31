---
target: agents_md
order: 310
rules: [di.riverpod-providers-for-dependencies, di.riverpod-overrides, di.riverpod-self-init, di.no-second-container]
---
## Dependency injection: Riverpod

- Repositories, API clients and services are top-level `Provider`s that build them from other providers: `final orderRepositoryProvider = Provider((ref) => OrderRepository(ref.watch(apiClientProvider)));`. Never construct them inside a widget, and never create a provider dynamically.
- Tests and environments swap an implementation with `overrides` on `ProviderScope` or `ProviderContainer` (`overrideWithValue`, `overrideWith`).
- Providers initialize themselves; a widget never calls an init method on one.
- Never add a service locator or a second DI package next to Riverpod.
