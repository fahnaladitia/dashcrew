---
title: Sign in
repos: [notes-app, notes-api]
tags: [auth]
---

# Sign in

Members sign in with an email address and a password. A successful sign in returns a session token that the client stores in secure storage. <!-- verified notes-app@abc1234 2026-09-01 -->

## Lockout

After five failed attempts in ten minutes the account is locked for fifteen minutes and an email explains how to unlock it. The counter lives in `notes-api:src/auth/lockout.go:12`. <!-- verified notes-app@abc1234 2026-09-01 -->
