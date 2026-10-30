---
target: role/qa-engineer
order: 370
rules: [role.qa-engineer.identifier-audit, qa.tests-find-by-key, qa.update-registry]
---
### Identifier audit

- On the diff, check every identifier: `key` and `identifier` carry the same string, the name follows the convention, it holds no dynamic value, and no identifier present before the diff is lost.
- Tests find actions with `find.byKey(const ValueKey('<identifier>'))`, not `find.text`. Integration journeys target the same identifiers.
- If the project keeps a central identifier registry or an automation coverage document and the screen appears in either, update both.
