---
title: Shopping cart
repos: [shop-web, shop-api]
tags: [shop]
---

# Shopping cart

Shoppers add products to a cart and change the quantity of each line. <!-- verified shop-web@def5678 2026-09-01 -->

## Cart lifetime

The cart is saved per account and kept for 7 days without activity; after that it expires and is emptied. `shop-api:src/cart/expiry.go:10` <!-- verified shop-web@def5678 2026-09-01 -->
