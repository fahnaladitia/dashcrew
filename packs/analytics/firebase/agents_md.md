---
target: agents_md
order: 350
rules: [analytics.flag-and-ask, analytics.no-unprompted-wiring, analytics.read-catalog-first, analytics.facade-only, analytics.constants-no-literals, analytics.never-rename-in-use, analytics.fire-from-callbacks, workflow.polish.analytics-awareness, workflow.summarize-analytics, pr.section.analytics]
---
## Analytics: Firebase

- On every feature change, flag whether it adds, renames or removes a user action, screen or funnel step, then ask the user whether to track it.
- Never wire an event without that decision. Roles only list candidates; the final review flags wiring without a decision.
- The step 0 prompt names the candidate actions and screens. The summary and the PR description's `## Analytics` section record the decision: added, declined or not relevant.
- Read `{{pack.analytics.catalog_doc}}` (events, funnels, properties) before touching analytics.
- Call `{{pack.analytics.facade}}` from the `{{pack.analytics.package}}` package. Never import `{{pack.analytics.vendor_import}}` in `lib/`.
- Event, property, screen and element names are constants in `{{pack.analytics.constants_file}}`. No string literals.
- Never rename an event or property in use; it breaks historical funnels.
- Fire from callbacks or state listeners, never inside `build()` or a builder.
- Prefer Google's recommended events with their prescribed parameters over a custom event for the same action, and check new events in DebugView before the PR.
