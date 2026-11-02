---
title: Shared lists
repos: [notes-app, notes-api]
tags: [todos, sharing]
---

# Shared lists

A todo list can be shared with members by invite. <!-- verified notes-app@abc1234 2026-09-01 -->

## Roles

Members of a shared list have one of three roles: owner, editor or viewer. Only the owner can invite or remove members. `notes-api:src/lists/members.go:19` <!-- verified notes-app@abc1234 2026-09-01 -->
