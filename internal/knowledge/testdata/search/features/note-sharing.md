---
title: Note sharing
repos: [notes-app, notes-api]
tags: [notes, sharing]
---

# Note sharing

A note can be shared by link with read-only or edit permission. <!-- verified notes-app@abc1234 2026-09-01 -->

## Link expiry

The owner may set the share link to expire after one day, one week or never. An expired link shows an access-ended page. `notes-api:src/share/link.go:15` <!-- verified notes-app@abc1234 2026-09-01 -->
