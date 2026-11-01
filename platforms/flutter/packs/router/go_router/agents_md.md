---
target: agents_md
order: 315
rules: [router.go-router-one-config, router.go-vs-push, router.ids-in-path-not-extra, router.redirect-for-gates, router.shell-route-for-bars]
---
## Navigation: go_router

- Routes are declared in one `GoRouter` in `{{pack.router.router_file}}`, given to `MaterialApp.router`.
- `context.go` replaces the stack (tab roots, deep-link targets, after sign-in); `context.push` adds a page and may await a result that the page returns with `context.pop(value)`.
- Pass ids in the path (`/orders/:id`) or query, not objects in `extra`: `extra` is serialized for browser history and dropped without a codec.
- Sign-in and onboarding gates live in `redirect`, never in a page's `initState`.
- A bar that stays visible across pages is a `ShellRoute`.
