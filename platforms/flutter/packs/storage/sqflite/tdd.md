---
target: workflow/tdd
order: 340
rules: [tdd.sqflite-ffi-in-memory]
---
### Database tests

Unit tests open an in-memory database through `sqflite_common_ffi` (a dev dependency), then close it.

```dart
setUpAll(sqfliteFfiInit);

test('save then find an order', () async {
  final db = await databaseFactoryFfi.openDatabase(inMemoryDatabasePath);
  await db.execute('CREATE TABLE orders (id TEXT PRIMARY KEY, total INTEGER)');
  final repository = OrderRepository(db);

  await repository.save(order);

  expect(await repository.find('order-1'), order);
  await db.close();
});
```
