---
target: agents_md
order: 330
rules: [tdd.mocktail-mocks, tdd.mocktail-closures, tdd.mocktail-fallback-values, tdd.mocktail-no-extension-stubs]
---
## Tests: mocktail

- Mocks are `class MockOrderRepository extends Mock implements OrderRepository {}`. When a test needs state rather than interaction checks, write a fake (`extends Fake implements ...`).
- Stub and verify with closures: `when(() => repository.fetch(any())).thenAnswer((_) async => order)`; async results always use `thenAnswer`.
- A custom type matched with `any()` needs `registerFallbackValue` in `setUpAll`; generic methods get explicit type arguments.
- Never stub or verify an extension method; stub the members it calls.
