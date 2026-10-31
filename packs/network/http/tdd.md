---
target: workflow/tdd
order: 320
rules: [tdd.http-mock-client]
---
### HTTP tests

Pass a `MockClient` from `package:http/testing.dart` to the class under test instead of the real client.

```dart
final client = MockClient((request) async => http.Response('{"id": "order-1"}', 200));
final repository = OrderRepository(client: client);
```
