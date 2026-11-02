# Offline sync

Edits made offline are queued and replayed in order. <!-- verified notes-app@0badc0de 2026-09-01 -->

## Conflicts

The server copy wins; see `notes-api:internal/sync/merge.go:120`.
