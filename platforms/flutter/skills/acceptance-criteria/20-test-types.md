---
target: skill/acceptance-criteria
order: 20
rules: [ac.flutter-test-types]
---
## Test types in Flutter

- `unit`: mapper, repository, model or state holder behavior, in `test/`.
- `widget`: what one screen or component shows and does, found by QA identifier, in `test/`. Write `widget` where the generic list says `ui`.
- `integration`: a journey across screens, in `integration_test/`.
- `manual`: needs a device, a real backend or a platform feature (camera, push, deep link from outside the app).
