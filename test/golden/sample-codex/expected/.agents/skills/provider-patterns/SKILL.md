---
name: provider-patterns
description: "provider patterns for this project - ChangeNotifier model shape, dependencies between providers, rebuild scope, disposal and tests. Use when: When adding or changing a ChangeNotifier model, a provider in the tree, or a widget that watches one."
---

# provider patterns

## Model shape

Private fields, public getters, one method per user action, and `notifyListeners()` after each change. The first load starts in `create`, never during a build.

```dart
class OrderModel extends ChangeNotifier {
  OrderModel(this._repository);

  final OrderRepository _repository;
  Order? _order;

  Order? get order => _order;

  Future<void> load() async {
    _order = await _repository.fetchCurrent();
    notifyListeners();
  }
}
```

## Rebuild scope

- `context.select((OrderModel m) => m.order?.total)` rebuilds only when that value changes.
- Wrap only the subtree that changes in `Consumer` or `Selector`, and pass static parts as `child`.

## Dependencies between providers

```dart
MultiProvider(
  providers: [
    Provider(create: (_) => OrderRepository()),
    ChangeNotifierProxyProvider<OrderRepository, OrderModel>(
      create: (context) => OrderModel(context.read<OrderRepository>())..load(),
      update: (context, repository, model) => model!,
    ),
  ],
  child: const OrderPage(),
)
```

## Disposal

- `ChangeNotifierProvider(create: ...)` disposes the model when it leaves the tree.
- `.value` never disposes; the owner of the object does.
- Read a provider that may be absent as nullable (`context.watch<OrderModel?>()`) instead of catching the error.
