---
target: role/qa-engineer
order: 320
rules: [router.getx-sheet-test-harness]
---
### GetX sheets in widget tests

- A widget test of a sheet or dialog opened through a GetX helper (`Get.bottomSheet`, `Get.dialog`, `Get.back()`) pumps it under `GetMaterialApp`. Under a plain `MaterialApp`, `Get.back()` throws.
