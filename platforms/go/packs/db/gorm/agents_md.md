---
target: agents_md
order: 330
rules: [db-gorm.usage]
---
## Database: GORM

- Every call starts from `db.WithContext(ctx)`.
- `First`, `Take` and `Last` return `gorm.ErrRecordNotFound` when nothing matches; `Find` does not. Map it to the domain not-found error once, in the repository.
- Conditions take placeholders: `Where("email = ?", email)`; never concatenate input into a condition or `Raw`.
- `Updates` with a struct skips zero values (`false`, `0`, `""`); update those through a map or `Select`.
- Several writes that belong together run in `db.Transaction(func(tx *gorm.DB) error { ... })`. `AutoMigrate` runs only locally, never as the production migration path.
