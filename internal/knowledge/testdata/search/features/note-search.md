---
title: Note search
repos: [notes-app, notes-api]
tags: [notes, search]
---

# Note search

Full text search across all notes runs on the server index; results highlight the matched words. Search on the device covers titles only while offline. `notes-api:src/search/index.go:52` <!-- verified notes-app@abc1234 2026-09-01 -->
