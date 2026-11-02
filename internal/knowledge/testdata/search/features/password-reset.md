---
title: Password reset
repos: [notes-app, notes-api]
tags: [auth]
---

# Password reset

Forgot password starts from the sign in screen. The member enters an email and receives a reset email. <!-- verified notes-app@abc1234 2026-09-01 -->

## Reset token

The reset link carries a single-use token that expires after 30 minutes. Opening an expired link shows a request-new-link screen. Token code: `notes-api:src/auth/reset.go:44`. <!-- verified notes-app@abc1234 2026-09-01 -->
