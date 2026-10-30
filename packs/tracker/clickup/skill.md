---
target: skill/tracker-clickup
order: 100
rules: [clickup.read-tools, clickup.my-tasks-script, clickup.rest-fallback, clickup.exact-status-names, clickup.pr-comment-after-step7, clickup.notify-all-off, clickup.comment-format, clickup.no-pr-no-comment]
---
# ClickUp tracker

## Reading

- One task: `get_task_details(task_id)`. Its thread: `get_task_comments(task_id)`.
- "My tasks" has no MCP tool: `get_tasks` needs a list id and cannot filter by assignee. Query the team task endpoint filtered by assignee, or ask the user for the list id.{{#pack.tracker.my_tasks_script}} The project helper `{{.}}` does that query and discovers the user and team ids itself; prefer it.{{/pack.tracker.my_tasks_script}}
- When the MCP tools are missing from the session, fall back to the REST API with the same rules: read with `GET /api/v2/task/<id>`, write status with `PUT /api/v2/task/<id>` and body `{"status": "<exact name>"}`.

## Token

- The API token lives in `{{pack.tracker.token_env}}`. Let the tool read it from the environment.
- Never print the token, paste it into a command line, write it to a file, or include it in output.

## Status names

- Read the valid names from the task's list (`GET /api/v2/list/<list_id>`, field `statuses`) and use the exact string. Never guess.
- If `{{pack.tracker.status_start}}` or `{{pack.tracker.status_review}}` does not exist in that list, stop and ask.

## PR comment

- Post the PR link only after the PR description file exists, and only when asked or per the status flow. It never replaces the PR description.
- If no PR exists yet, say so and stop. Never invent or guess a URL.
- Post with `notify_all` set to false unless the user asks for the ping.
- Exactly three lines:

```
PR: <url printed by the PR tool>
Branch: <branch name>
<one line: what changed>
```
