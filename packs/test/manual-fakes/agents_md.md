---
target: agents_md
order: 330
rules: [tdd.manual-fakes]
---
## Tests: manual fakes

- Tests use plain `flutter_test` with hand-written fakes (`implements`, or `extends` for a large interface). Never add mocktail, mockito or bloc_test.
