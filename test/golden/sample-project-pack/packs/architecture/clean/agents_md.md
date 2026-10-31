---
target: agents_md
order: 380
rules: [team.use-case-layer]
---
## Architecture: use cases

- State holders call use cases in `{{pack.architecture.domain_dir}}`, never repositories directly. A use case does one thing and is named after it.
