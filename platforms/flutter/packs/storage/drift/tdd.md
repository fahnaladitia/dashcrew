---
target: workflow/tdd
order: 340
rules: [tdd.drift-in-memory, tdd.drift-migration-steps]
---
### Database tests

Unit tests open `{{pack.storage.database_class}}` on an in-memory database, then close it. The class takes its `QueryExecutor` through the constructor for this.

```dart
late {{pack.storage.database_class}} db;

setUp(() => db = {{pack.storage.database_class}}(NativeDatabase.memory()));
tearDown(() => db.close());

test('save then find an order', () async {
  final repository = OrderRepository(db);

  await repository.save(order);

  expect(await repository.find('order-1'), order);
});
```

With `drift_dev make-migrations`, every new schema version gets a test of its step through the generated schema verifier.
