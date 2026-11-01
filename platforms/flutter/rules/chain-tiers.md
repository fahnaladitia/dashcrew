---
target: agents_md
order: 11
rules: [tier.state-in-step0, tier.user-override, tier.highest-signal-wins, tier.definitions, tier.tie-breaker-minimum-standard]
---
### Size tier

The step 0 `Tier:` line gives evidence, and the user may override it. Take the highest tier any signal reaches.

| Tier | Files touched | Contract or dependency | Ambiguity |
|---|---|---|---|
| trivial | 1 file, a few lines, one layer | none | none |
| small | 1-2 files in one layer, or one existing widget or state holder | none | clear after reading the code |
| standard | 2-5 files across 2+ layers | maybe a new response, model or mapper | one real choice |
| large | cross-feature, new screen or state holder, DI setup, local storage | new backend contract, new package, or a design file | several open questions |

Scope that touches a security-review trigger, a backend contract, local storage or a design file is at least standard, whatever the file count.
