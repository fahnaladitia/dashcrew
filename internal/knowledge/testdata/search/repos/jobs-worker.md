---
title: jobs-worker
repos: [jobs-worker]
tags: [repo]
---

# jobs-worker

The background worker runs scheduled jobs: it purges notes that stayed in the trash past their limit and sends due reminders. A failed job is retried with exponential backoff up to five times. <!-- verified notes-app@abc1234 2026-09-01 -->
