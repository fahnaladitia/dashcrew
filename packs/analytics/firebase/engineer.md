---
target: role/flutter-engineer
order: 350
rules: [analytics.api-usage, analytics.new-event-checklist]
---
### Analytics calls

Only after the user approved the event:

- Buttons: `{{pack.analytics.facade}}.{{pack.analytics.tap_api}}(...)`.
- Custom and funnel events: `{{pack.analytics.facade}}.{{pack.analytics.event_api}}(name, parameters: {...})`.
- Screens are tracked by the navigator observer; call `{{pack.analytics.facade}}.{{pack.analytics.screen_api}}(...)` only for a screen the observer cannot see (a sheet, a tab).
- A new event or property value adds the constant to `{{pack.analytics.constants_file}}`, adds or extends a test in the `{{pack.analytics.package}}` package, and documents it in `{{pack.analytics.catalog_doc}}`.
