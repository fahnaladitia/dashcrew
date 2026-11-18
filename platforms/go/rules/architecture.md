---
target: agents_md
order: 70
rules: [go.layers, go.layer-folders, go.small-interfaces, go.no-sql-in-handlers, go.config-once]
---
## Architecture

- Requests flow handler -> service -> repository -> store. Handlers decode, validate and map errors to responses; services hold the business rules; repositories own every query. Rename these layers to the names this repository uses.
- New code follows the existing package layout, one package per layer or per feature as the repository already does; never invent a new layout.
- A consumer declares the small interface it needs (one to three methods), next to itself. Constructors return concrete types.
- No SQL, ORM call or connection in a handler or service; only repositories touch the store.
- Configuration is read once at startup into a struct and passed down; no `os.Getenv` below `main`.
