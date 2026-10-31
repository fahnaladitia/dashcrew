---
name: data-layer-tdd
description: "Test cases and patterns for data-layer TDD in Flutter - mappers, repositories, models and state holders."
when_to_use: "When writing or changing a mapper, repository, model or state holder, before the implementation."
---

# Data-layer TDD

Every feature ships data-layer tests in the same task, not later.
The cycle (RED, GREEN, REFACTOR, run budget) is in the `tdd` workflow.
The test-double style (hand-written fakes or a mocking library) is set by the selected test pack; follow AGENTS.md.

## Fixtures

- Use the real JSON fixtures in the project's fixture folder.
- Never invent a response shape. A test that passes on a wrong assumption is worse than no test.

## Mapper

The mapper is the one place where the backend shape becomes the domain model, so it is tested the hardest.

Required cases:

- full response, every field present
- optional fields that are `null` map to their defaults
- fields missing from the JSON entirely
- empty lists
- numbers sent as strings

```dart
void main() {
  group('OrderMapper', () {
    test('maps full response', () {
      final json = jsonDecode(
        File('<fixture folder>/order_detail.json').readAsStringSync(),
      ) as Map<String, dynamic>;

      final result = OrderMapper.map(OrderResponse.fromJson(json));

      expect(result.id, '123');
      expect(result.total, 150000);
    });

    test('maps missing discount to zero', () {
      final result = OrderMapper.map(OrderResponse.fromJson({'id': '1'}));

      expect(result.discount, 0);
    });
  });
}
```

## Repository

Use a test double for the client. Required cases: success, client error, empty response.

```dart
test('returns mapped order on success', () async {
  final result = await repository.fetch('123');

  expect(result.id, '123');
});

test('propagates client error', () async {
  expect(() => failingRepository.fetch('123'), throwsException);
});
```

## Model

A model with value equality (for example one extending `Equatable`) gets two tests that prove its props are complete:

```dart
test('equal when fields are equal', () {
  expect(const Order(id: '1'), const Order(id: '1'));
});

test('not equal when id differs', () {
  expect(const Order(id: '1'), isNot(const Order(id: '2')));
});
```

This is not a trivial test. Props missing a field are why a UI does not rebuild when the data changed.

## State holder

Test the sequence of emitted states. Always cancel the subscription and close the state holder at the end of the test; leaks make other tests flaky.

## Do not test

- getters that only pass a field through
- the Flutter framework
- generated code (`*.g.dart`)
- anything that passes whatever the implementation is
