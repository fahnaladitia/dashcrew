---
target: workflow/tdd
order: 300
rules: [tdd.riverpod-container-test]
---
### Riverpod tests

Build a `ProviderContainer.test()` with the repository overridden by a fake; it disposes itself when the test ends. Await `.future` for an async provider.

```dart
test('build loads the current order', () async {
  final container = ProviderContainer.test(
    overrides: [orderRepositoryProvider.overrideWithValue(FakeOrderRepository())],
  );

  final order = await container.read(orderProvider.future);

  expect(order.id, 'order-1');
});
```

In widget tests, pump the screen under `ProviderScope(overrides: [...])` and read state through `tester.container()`.
