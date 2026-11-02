---
title: Two-factor authentication
repos: [notes-app, notes-api]
tags: [auth]
---

# Two-factor authentication

Members can turn on two-factor authentication with a TOTP authenticator app. <!-- verified notes-app@abc1234 2026-09-01 -->

## Recovery codes

Ten single-use recovery codes are shown once when two-factor is turned on. Each code works once in place of a TOTP code; generating new codes voids the old ones. `notes-api:src/auth/totp.go:8` <!-- verified notes-app@abc1234 2026-09-01 -->
