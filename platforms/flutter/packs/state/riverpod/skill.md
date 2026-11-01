---
target: skill/riverpod-patterns
order: 100
rules: [riverpod.choose-provider-type, riverpod.family-via-arguments, riverpod.select-rebuilds, riverpod.listen-side-effects, riverpod.dispose-hooks, riverpod.invalidate-to-refresh, riverpod.provider-exception]
---
# Riverpod patterns

## Which provider

- A value computed once from other providers, or a one-shot fetch: `Provider` or `FutureProvider`.
- State that changes through methods: `Notifier`; when its first value is loaded: `AsyncNotifier`.

```dart
final orderProvider = AsyncNotifierProvider<OrderNotifier, Order>(OrderNotifier.new);

class OrderNotifier extends AsyncNotifier<Order> {
  @override
  Future<Order> build() => ref.watch(orderRepositoryProvider).fetchCurrent();

  Future<void> rename(String name) async {
    final order = await ref.read(orderRepositoryProvider).rename(name);
    if (!ref.mounted) return;
    state = AsyncData(order);
  }
}
```

## Families

A provider that needs a parameter is a family; the argument reaches the notifier through its constructor, and `build()` takes none.

## Rebuild scope

- Watch only the slice the widget shows: `ref.watch(orderProvider.select((order) => order.value?.total))`.
- Split a large screen into `ConsumerWidget`s so each one watches what it needs.

## Side effects

- `ref.listen` reacts to a change without rebuilding: navigation, snackbars, dialogs, analytics.
- Never start a side effect from the value `ref.watch` returned.

## Disposal and refresh

- Clean up timers, subscriptions and controllers a provider creates in `ref.onDispose`.
- Keep a provider alive past its last listener only on purpose, with `ref.keepAlive()`.
- To refetch, call `ref.invalidate(provider)` instead of copying the fetch into a method.

## Errors

- A provider that fails while building is retried automatically; a read of a failed provider throws `ProviderException`, with the original error in `exception`.
- `AsyncValue.error`, `ref.listen` `onError` and observers still see the original error.
