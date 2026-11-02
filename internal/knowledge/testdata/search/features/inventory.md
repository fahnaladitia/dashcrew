---
title: Inventory reservation
repos: [shop-api]
tags: [shop, orders]
---

# Inventory reservation

Stock is reserved when checkout starts. A reservation is released after 15 minutes if the payment did not complete. `shop-api:src/stock/reserve.go:40` <!-- verified shop-web@def5678 2026-09-01 -->
