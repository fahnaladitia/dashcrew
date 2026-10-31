---
target: agents_md
order: 330
rules: [tdd.mockito-generated, tdd.mockito-then-answer, tdd.mockito-prefer-fakes, tdd.mockito-named-matchers]
---
## Tests: mockito

- Mocks are generated: `@GenerateNiceMocks([MockSpec<OrderRepository>()])` in the test file, then `dart run build_runner build`. Never edit `*.mocks.dart`.
- A `Future` or `Stream` is stubbed with `thenAnswer`; `thenReturn` throws for them.
- Prefer real objects, then fakes, over mocks. A mock holds no behavior of its own; stubs live in the test.
- Named arguments need named matchers (`anyNamed('limit')`); never pass a bare `null` next to matchers.
