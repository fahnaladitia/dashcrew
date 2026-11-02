---
title: Note editor
repos: [notes-app]
tags: [notes]
---

# Note editor

The note editor supports rich text with markdown shortcuts such as # for a heading and - for a list. <!-- verified notes-app@abc1234 2026-09-01 -->

## Autosave

Changes are saved automatically every 5 seconds while typing and when the editor closes. A failed save keeps the draft locally. `notes-app:lib/editor/autosave.dart:21` <!-- verified notes-app@abc1234 2026-09-01 -->
