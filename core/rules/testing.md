---
target: agents_md
order: 80
rules: [testing.locations, testing.flat-test-dir, testing.feature-needs-data-layer-tests, testing.no-test-later, testing.scoped-runs, testing.state-scope, testing.batch-runs, testing.no-coverage-threshold]
---
## Testing

- Unit and state-holder tests follow the location and naming of the existing suite, kept flat unless it already uses subfolders.
- Every feature ships data-layer tests (mapper, repository, model, state holder) in the same task, written first. A touched data-layer unit with no test is a high-severity finding.
- Add a test now, or report it as skipped in the summary; "later" is not an option.
- Scope runs to the diff: `{{platform.commands.analyze}} <changed paths>` and `{{platform.commands.test_scoped}} <covering test files>`. Run the whole repo only when the user asks or a release is cut.
- Say which scope ran. A scoped green run says nothing about the rest of the suite.
- Run all test files of a stage in one call, RED once and GREEN once per stage; aim for at most 6 test runs per task.
- There is no coverage threshold; never invent one.
