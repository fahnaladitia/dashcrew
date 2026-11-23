---
target: workflow/tdd
order: 320
rules: [tdd.chopper-mock-client]
---
### Network tests

Build the `ChopperClient` on a `MockClient` from `package:http/testing.dart` and test the repository against canned responses, a non-2xx one included.

```dart
final chopper = ChopperClient(
  client: MockClient((request) async => http.Response('{"id": "order-1"}', 200)),
  services: [OrderService.create()],
  converter: const JsonConverter(),
);
final repository = OrderRepository(chopper.getService<OrderService>());
```
