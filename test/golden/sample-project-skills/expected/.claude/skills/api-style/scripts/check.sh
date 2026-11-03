#!/bin/sh
# Lists DTO fields that are not snake_case.
grep -rn "@JsonKey" lib/ || true
