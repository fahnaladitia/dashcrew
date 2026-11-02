---
title: Recurring tasks
repos: [notes-app, notes-api]
tags: [todos]
---

# Recurring tasks

A todo item can repeat daily, weekly or monthly. <!-- verified notes-app@abc1234 2026-09-01 -->

## Next occurrence

The next occurrence is created only when the current one is completed, so missed items do not pile up. `notes-api:src/todos/recurrence.go:61` <!-- verified notes-app@abc1234 2026-09-01 -->
