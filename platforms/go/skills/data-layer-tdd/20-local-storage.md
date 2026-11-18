---
target: skill/data-layer-tdd
order: 20
rules: [go.store-tests, go.store-transactions]
---
## Repository against a database

- Repository tests run against a real database of the same engine (a container or a local instance) when the repository already has such tests; otherwise the change says how the queries were checked.
- Each test creates the rows it needs and cleans up, or runs inside a transaction that is rolled back.
- A change that needs several writes to succeed together uses one transaction, and the test proves that a failure in the second write leaves the first rolled back.
