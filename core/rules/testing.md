---
target: agents_md
order: 80
rules: [testing.locations, testing.flat-test-dir, testing.integration-journeys, testing.feature-needs-data-layer-tests, testing.no-test-later, testing.scoped-runs, testing.state-scope, testing.batch-runs, testing.expanded-reporter, testing.no-coverage-threshold]
---
## Testing

- Unit and state-holder tests live in `test/`, named `<subject>_test.dart` and kept flat unless the existing suite already uses subfolders. Integration journeys live in `integration_test/` and follow `flutter-add-integration-test`.
- Every feature ships data-layer tests (mapper, repository, model, state holder) in the same task, written first. A touched data-layer unit with no test is a high-severity finding.
- Add a test now, or report it as skipped in the summary; "later" is not an option.
- Scope runs to the diff: `flutter analyze <changed paths>` and `flutter test <covering test files>`. Run the whole repo only when the user asks or a release is cut.
- Say which scope ran. A scoped green run says nothing about the rest of the suite.
- Run all test files of a stage in one call, RED once and GREEN once per stage; aim for at most 6 test runs per task.
- Run `flutter test --reporter expanded` so test names are visible.
- There is no coverage threshold; never invent one.
