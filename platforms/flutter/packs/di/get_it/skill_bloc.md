---
target: skill/bloc-patterns
order: 150
rules: [bloc.test-fake-via-locator]
when: { stack.state.use: bloc }
---
## Dependencies and tests

The constructor stays empty. Dependencies come from the container in fields; dynamic values arrive on events or method arguments.

```dart
class OrderCubit extends Cubit<OrderState> {
  OrderCubit() : super(const OrderInitial());

  final OrderRepository repository = {{pack.di.locator}}.get<OrderRepository>();

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
  create: (_) => {{pack.di.locator}}.get<OrderCubit>()..load(orderId),
  child: const OrderView(),
)
```

Tests never inject through the constructor. Register the fake in the container before the bloc is created, and unregister it in `tearDown`.

```dart
setUp(() {
  {{pack.di.locator}}.registerFactory<OrderRepository>(() => FakeOrderRepository());
});

tearDown(() {
  {{pack.di.locator}}.unregister<OrderRepository>();
});
```
