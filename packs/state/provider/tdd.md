---
target: workflow/tdd
order: 300
rules: [tdd.provider-model-test]
---
### provider model tests

Test the `ChangeNotifier` directly with a fake repository: count notifications through `addListener`, then `dispose()` it.

```dart
test('load notifies once with the order', () async {
  final model = OrderModel(FakeOrderRepository());
  var notifications = 0;
  model.addListener(() => notifications++);

  await model.load();

  expect(model.order?.id, 'order-1');
  expect(notifications, 1);
  model.dispose();
});
```

In widget tests, pump the screen under the same providers, with fakes passed to `create`.
