# Shared knowledge and specs

A team can keep its product knowledge (how a flow works, a contract, a trap, a decision and its reason) in one reviewed git repo that every member's agents read. Agents propose; people review.

**Layout.** `README.md` (the generated index), `features/<slug>.md`, `repos/<repo>.md`, and `inbox/` for proposals. A page has front matter (`title`, `repos`, `tags`); each fact ends with `<!-- verified <repo>@<sha> <yyyy-mm-dd> -->`, and code pointers are written `` `<repo>:<path>:<line>` ``.

**Setup.**

1. `dashcrew knowledge init <dir>` writes that layout into a new folder. Commit it to a new repo and push it.
2. In each project, set `knowledge: { repo: <git url or path> }` in `.ai/project.yaml` (`name` defaults to the repo's last path segment).
3. Each member runs `dashcrew knowledge pull`: it clones the repo into `~/.dashcrew/knowledge/<name>`, or fetches and fast-forwards it. A diverged clone is reported, never reset.
4. `dashcrew sync` links `.ai/knowledge` to the clone (a copy on Windows), git-ignores the link, and registers the knowledge MCP server for every agent in the profile. `sync` never touches the network; with no clone it warns and carries on.

**Guard rule.** Agents read the whole clone but write only `.ai/knowledge/inbox/`. The guard checks the real path, so a write through the link, the absolute clone path, `..` or another symlink is blocked the same way, for edits, patches and shell redirections.

**Retro and review.** With `knowledge` set, `learn` turns each product lesson into one proposal in `inbox/` (a standing instruction for agents goes to `.ai/packs/` instead). `dashcrew knowledge review` walks the inbox oldest first: accept merges the proposal into its page with a verified marker (making the page when needed), edit opens `$EDITOR`, skip or delete. It then regenerates `README.md` and prints the `git` commands to open a pull request on the knowledge repo. A page it creates keeps only the sections a proposal filled, and a `target` written as the page path (`features/orders.md`) is read as its slug.

**Team flow.** Two members accepting into the same page conflict on its lines and on `README.md`. Keep page edits in one place:

1. Members open pull requests on the knowledge repo that add only `inbox/` files. Each file has its own dated name, so these never conflict.
2. One member at a time (a lead, or a rota) runs `dashcrew knowledge pull` and `dashcrew knowledge review` on the main branch and opens one pull request with the merged pages, where duplicate facts also show.

`knowledge review` fetches first and stops when the clone is behind its upstream (`--no-fetch` skips it); offline, or on a branch with no upstream, it only warns. On a conflict anyway: take either side of `README.md` and run `knowledge review` with an empty inbox to rebuild the index; merge a page by hand, keeping both sides' verified markers, then run `knowledge doctor`.

**Staleness.** `dashcrew knowledge doctor` reports code pointers whose path is gone at HEAD (for the current project and sibling folders), proposals waiting over 14 days, and pages with no verified marker in 180 days. It edits nothing, and exits 1 only on a stale pointer. `dashcrew doctor` runs it as the `knowledge-stale` check.

**MCP server.** `dashcrew knowledge serve [--clone <dir>]` is a stdio MCP server with three tools: `knowledge_search` (BM25 over title, headings and body of the pages and the spec files; page, kind, title, snippet, score, with an optional `kind` filter: `spec`, `feature`, `repo` or `other`, from the page's folder), `knowledge_read` (the page and its verified markers) and `knowledge_sources` (each code pointer as found, missing or unchecked). Its index lives in memory and is rebuilt when the clone's HEAD moves. `sync` registers it as `dashcrew-knowledge` in `.mcp.json`, `.codex/config.toml` and `.agents/mcp_config.json`, and removes it when `knowledge` is dropped. AGENTS.md, the `explorer` and the `plan` workflow tell agents to read the knowledge before planning and to check a page's code pointers before relying on it.

dashcrew never commits, pushes or resets the knowledge repo; every change to it goes through a member's own pull request.

## Specs

With `knowledge` set, a large feature gets a PRD and a TRD in the knowledge repo before its tasks start, and every task says which revision it builds.

**Format.** `specs/<release>/<feature>/prd.md` (problem, users, goals, scope, Not doing, acceptance criteria) and `trd.md` (impact per repo, contracts, data, rollout order, risks, decisions, open questions). `<release>` is a version, or `unreleased`. Both files carry front matter: `title`, `feature`, `release`, `status` (`draft`, `review`, `approved` or `superseded`), `rev` (from 1), `repos`, and `owners` as roles, never names or emails.

**IDs.** `AC-n` (criteria), `C-n` (contracts) and `D-n` (decisions) start a list item or heading. They are never renumbered; a dropped one stays as `~~AC-3~~ dropped: <why>`. Every open question ends with `owner: <role>`.

**Commands.**

| Command | |
|---|---|
| `spec new <feature> [--release <v>] [--clone <dir>]` | writes both files from the repo's templates, `status: draft`, `rev: 1`; the release defaults to the branch's recorded one, else `unreleased`. Refuses an existing folder |
| `spec lint [<path>] [--against <ref>]` | front matter, duplicate IDs, gaps in a new spec, unowned questions in an approved one; with `--against`, a vanished or renumbered ID and an approved file changed without a higher `rev`. Exit 1 on any problem. `knowledge doctor` runs it over `specs/` |
| `spec revise <path>` | run by a member: moves an approved spec back to `review` with `rev` one higher in both files, and prints the `git` commands for the pull request |
| `spec status [-p <dir>] [--clone <dir>]` | the spec the branch records against the clone's current `rev` and status, the ACs this repo's plans claim (plans under the plans path that name the spec folder), the ACs none claims, and the spec's other repos as not checked |

**Guard rule.** Under `specs/`, agents write only new files and files whose status is `draft` or `review` (read from the first 2 KB). Approved and superseded files are blocked with a pointer to `dashcrew spec revise`, which agents may not run. A patch that moves or deletes a file is checked at the old path too, so a page or spec cannot be moved into the inbox. Approval is the merged knowledge-repo pull request in which a member set `status: approved`; there is no approve command.

**Workflow.** The `spec` workflow (main session, outside the chain): one round of brainstorm questions, the PRD with `acceptance-criteria`, the TRD with `feature-architecture` and the platform's TRD sections, `spec lint`, then `status: review` and the `git` commands for the member.

**Tasks follow the spec.** Step 0 gains a `Spec` row (`specs/<release>/<feature>@rev<n> | none`); with a spec, the success criteria are its ACs by ID. The `branching` skill records it with `git config branch.<name>.dashcrew-spec <value>`. The `plan` names the spec and revision, lists the ACs it covers, and states each departure from the TRD as `departs from D-n: <what and why>`; an undeclared one makes it NO-GO, and the `reviewer` reports a change against a decision or contract the plan did not declare as HIGH. An accepted departure is followed by a `spec revise` pull request, named in the summary.

**Drift.** `doctor` warns, and never fails, when the recorded `rev` is older than the clone's ("spec moved to rev 4; re-read it") or when the spec is still `draft` while the branch has commits of its own (commits no other branch holds). After the feature ships, the knowledge-gated `learn` proposes its `features/<slug>.md` page through the inbox from the PRD's goals and the TRD's final decisions, with code pointers; a spec becomes `superseded` only when a newer spec replaces it.

Not done: an approve command, specs outside the knowledge repo, syncing specs to a tracker, generating code or tests from a spec.
