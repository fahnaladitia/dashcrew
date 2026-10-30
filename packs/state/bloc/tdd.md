---
target: workflow/tdd
order: 300
rules: [tdd.bloc-state-sequence]
---
### Bloc and Cubit tests

Assert the emitted state sequence, then cancel the subscription and close the bloc. A leaked subscription makes other tests flaky.

```dart
test('load emits loading then loaded', () async {
  final cubit = OrderCubit();
  final states = <OrderState>[];
  final subscription = cubit.stream.listen(states.add);

  await cubit.load('order-1');

  expect(states, [const OrderLoading(), isA<OrderLoaded>()]);
  await subscription.cancel();
  await cubit.close();
});
```
