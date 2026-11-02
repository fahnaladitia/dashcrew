---
title: Card payments
repos: [shop-web, shop-api]
tags: [shop, payments]
---

# Card payments

Card payments go through the payment provider. <!-- verified shop-web@def5678 2026-09-01 -->

## 3-D Secure

When the card issuer asks for 3-D Secure, the shopper confirms in a bank challenge window; the payment is captured only after the provider webhook reports success. `shop-api:src/payments/webhook.go:27` <!-- verified shop-web@def5678 2026-09-01 -->
