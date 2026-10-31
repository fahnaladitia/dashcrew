---
name: bloc-patterns
description: "Cubit patterns for this team - state shape, rebuild scope and tests."
when_to_use: "When adding or changing a Cubit or the widgets that listen to it."
---

# Cubit patterns

- States extend `Equatable`; a screen with more than one async load uses one status enum, never several booleans.
- Scope rebuilds with `BlocSelector` or `buildWhen`.

## Dependencies and tests

The constructor stays empty. Dependencies come from the container in fields; dynamic values arrive on events or method arguments.

```dart
class OrderCubit extends Cubit<OrderState> {
  OrderCubit() : super(const OrderInitial());

  final OrderRepository repository = getIt.get<OrderRepository>();

  Future<void> load(String orderId) async {
    emit(const OrderLoading());
    final order = await repository.fetch(orderId);
    emit(OrderLoaded(order));
  }
}
```

The screen provides it from the container:

```dart
BlocProvider(
  create: (_) => getIt.get<OrderCubit>()..load(orderId),
  child: const OrderView(),
)
```

Tests never inject through the constructor. Register the fake in the container before the bloc is created, and unregister it in `tearDown`.

```dart
setUp(() {
  getIt.registerFactory<OrderRepository>(() => FakeOrderRepository());
});

tearDown(() {
  getIt.unregister<OrderRepository>();
});
```
