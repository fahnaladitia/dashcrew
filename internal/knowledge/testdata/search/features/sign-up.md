---
title: Sign up
repos: [notes-app, notes-api]
tags: [auth]
---

# Sign up

A new account is created from the sign up form with an email, a display name and a password. <!-- verified notes-app@abc1234 2026-09-01 -->

## Email verification

The server sends a verification link; the account stays limited until the link is opened. Password rules: at least ten characters, one digit. See `notes-api:src/auth/register.go:30`. <!-- verified notes-app@abc1234 2026-09-01 -->
