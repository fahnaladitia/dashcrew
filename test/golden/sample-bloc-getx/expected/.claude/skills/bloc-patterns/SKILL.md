---
name: bloc-patterns
description: "Bloc and Cubit patterns for this project - choosing between them, state shape, rebuild scope, side effects, lifecycle and tests."
when_to_use: "When adding or changing a Bloc or Cubit, its states or events, or the widgets that listen to it."
---

# Bloc patterns

## Bloc or Cubit

Use a Cubit when a state change is triggered by a direct method call and its origin does not matter.

```dart
class OrderFilterCubit extends Cubit<OrderFilterState> {
  OrderFilterCubit() : super(const OrderFilterState());

  void selectStatus(OrderStatus status) => emit(state.copyWith(status: status));
}
```

Use a Bloc only when events must be told apart, ordered, debounced or transformed.

```dart
class OrderSearchBloc extends Bloc<OrderSearchEvent, OrderSearchState> {
  OrderSearchBloc() : super(const OrderSearchState()) {
    on<OrderSearchQueryChanged>(_onQueryChanged, transformer: restartable());
  }
}
```

A Bloc with a single event that maps to a single method is a Cubit.

## State shape

- States and events extend `Equatable`.
- `props` lists every field that affects the UI. A field missing from `props` does not trigger a rebuild when it changes.

```dart
class OrderLoaded extends OrderState {
  const OrderLoaded(this.order);

  final Order order;

  @override
  List<Object?> get props => [order];
}
```

## Rebuild scope

- `BlocSelector` is the default for large states; it rebuilds only when the selected value changes.
- Use `buildWhen` when a `BlocBuilder` needs most of the state but not every transition.
- Never put a plain `BlocBuilder` at the screen root; scope it to the subtree that changes.

```dart
BlocSelector<OrderCubit, OrderState, int>(
  selector: (state) => state is OrderLoaded ? state.order.total : 0,
  builder: (context, total) => Text('$total'),
)
```

## Side effects

- Navigation, snackbars, dialogs and analytics run in `BlocListener` or in callbacks, never in a builder.
- Use `listenWhen` so a side effect fires once per transition, not on every emit.

## Lifecycle

- `BlocProvider(create: ...)` closes its bloc when the provider leaves the tree.
- `BlocProvider.value(...)` never closes the bloc; its creator does.
- A bloc created by hand (outside a provider) is closed in the owning widget's `dispose()`.
- Do not call `setState` in a widget whose state a bloc already manages.

## Blocs talking to each other

- A bloc never depends on another bloc directly.
- Share data through a repository both blocs read, or react in the UI with a `BlocListener` that adds an event to the second bloc.

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
