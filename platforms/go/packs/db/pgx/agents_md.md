---
target: agents_md
order: 330
rules: [db-pgx.usage]
---
## Database: pgx

- Share one `*pgxpool.Pool`, created at startup; pass the request context to every call.
- `QueryRow(...).Scan` returns `pgx.ErrNoRows` when nothing matches: map it to the domain not-found error once, in the repository.
- Close rows (`defer rows.Close()`) or use `pgx.CollectRows`; check `rows.Err()`.
- Transactions: `tx, err := pool.Begin(ctx)`, `defer tx.Rollback(ctx)`, then `tx.Commit(ctx)`; the deferred rollback is a no-op after commit.
