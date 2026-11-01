---
target: workflow/tdd
order: 310
---
### Writing a fake

A fake implements the interface, records calls, and returns canned data that the test sets.

```dart
class FakeOrderRepository implements OrderRepository {
  Order? nextOrder;
  Object? nextError;
  final fetchedIds = <String>[];

  @override
  Future<Order> fetch(String orderId) async {
    fetchedIds.add(orderId);
    final error = nextError;
    if (error != null) throw error;
    return nextOrder ?? (throw StateError('nextOrder not set'));
  }
}
```
