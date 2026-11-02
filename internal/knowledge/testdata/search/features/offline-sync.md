---
title: Offline sync
repos: [notes-app]
tags: [notes, sync]
---

# Offline sync

The app works offline on a local database. Edits made offline wait in a sync queue and are sent when the connection returns. <!-- verified notes-app@abc1234 2026-09-01 -->

## Conflicts

When the same note changed on two devices, the server keeps the last write and stores the other version as a conflict copy. `notes-app:lib/sync/queue.dart:77` <!-- verified notes-app@abc1234 2026-09-01 -->
