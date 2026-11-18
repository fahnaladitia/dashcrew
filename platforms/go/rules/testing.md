---
target: agents_md
order: 80
rules: [go.tests-table-driven, go.tests-fakes, go.tests-feature-needs-tests, testing.no-test-later, go.tests-scoped, testing.batch-runs, go.tests-race, testing.no-coverage-threshold]
---
## Testing

- Tests sit next to the code (`x_test.go`) and are table-driven: one slice of cases, `t.Run(name, ...)` per case.
- Dependencies are faked by hand-written types behind the consumer's small interface; no mocking library unless the repository already uses one.
- Every feature ships service and repository tests in the same task, written first. A touched service or repository with no test is a high-severity finding.
- Add a test now, or report it as skipped in the summary; "later" is not an option.
- Scope runs to the diff: `go vet ./<pkg>/...` and `go test ./<pkg>/... -run <Test>`. Run `go test ./...` only when the user asks or a release is cut. Say which scope ran.
- Run all test files of a stage in one call, RED once and GREEN once per stage; aim for at most 6 test runs per task.
- Code that starts goroutines is tested with `-race`.
- There is no coverage threshold; never invent one.
