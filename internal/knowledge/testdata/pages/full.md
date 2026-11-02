---
title: Note sharing
repos: [notes-api, notes-app]
tags: [sharing, permissions]
---

# Note sharing

## Purpose

A note owner shares a note by link; readers open it without an account. <!-- verified notes-api@3f2a9c1 2026-10-30 -->

## Entry points

- `notes-api:internal/share/handler.go:42` creates the link.
- `notes-app:lib/share/share_sheet.dart:17` opens the sheet. <!-- verified notes-app@a1b2c3d4e5f6 2026-10-29 -->

```text
# not a heading
`notes-api:ignored/in/fence.go:1`
<!-- verified notes-api@ffffff0 2026-01-01 -->
```

### Gotchas ###

Links expire after 7 days; see `notes-api:internal/share/expiry.go:9`. <!-- verified notes-api@3f2a9c1 2026-10-30 -->
