# P5 plan: shared knowledge

Tier: large (new commands, an MCP server, a new profile key, a guard rule change, catalog changes to `learn` and the chain). Idea and trade-offs: `docs/ideas/knowledge-tracker-multirepo.md`, section P5.

## Goal

Each AI agent on the team draws on one shared, reviewed body of product knowledge. "How does checkout work?" gets an answer from a curated page whose code pointers the agent re-reads before answering. That knowledge grows from the retro at the end of each task, through human review, never by unreviewed writes.

Today the closest thing is per-machine agent memory. It is private, never reviewed, and invisible to the rest of the team.

## Design

**The knowledge repo** is a separate git repo, one per team:

```
<knowledge repo>/
  README.md               generated index (the "knowledge book"), rendered by GitHub
  features/<slug>.md      purpose, entry points per repo, state, endpoints, decisions and why, gotchas
  repos/<repo>.md         facts about one repo: setup, conventions not written elsewhere, traps
  inbox/<date>-<slug>.md  retro proposals waiting for review
```

- A page has front matter (`title`, `repos`, `tags`) and a body.
- Every fact paragraph ends with `<!-- verified <repo>@<short sha> <yyyy-mm-dd> -->`.
- Code pointers are written as `` `<repo>:<path>:<line>` `` so tools can find them.

**On each member's machine:**

- Profile key `knowledge: { repo: <git url or path>, name: <short name> }`.
- The local clone lives at `~/.dashcrew/knowledge/<name>`.
- `sync` links `.ai/knowledge` to that clone. The link is git-ignored, and on Windows `sync` copies the clone instead.
- `sync` never touches the network. `dashcrew knowledge pull` clones the repo or fast-forwards it.

**For agents:**

- They read and grep `.ai/knowledge` as plain files.
- They write only `.ai/knowledge/inbox/`. The guard enforces this on the real path.
- The MCP server `dashcrew knowledge serve` (stdio) offers ranked search for agents and clients that cannot read files.

## Stages

### K0: format and scaffold

- `dashcrew knowledge init <dir>` writes the layout above, a short `README.md` and a page template.
- Page parser:
  - front matter;
  - headings;
  - verified markers (repo, sha, date);
  - code pointers (`repo:path:line`).
- Tests over fixture pages, including malformed markers and pages without front matter.

### K1: profile, clone and link

- Profile key `knowledge` (strict keys: `repo` required, `name` defaulting to the repo's last path segment without `.git`).
- `dashcrew knowledge pull`: clones into `~/.dashcrew/knowledge/<name>` when missing, otherwise `git fetch` and a fast-forward only merge. A diverged clone is reported, never reset.
- `sync`:
  - creates or repairs the `.ai/knowledge` link and adds it to the project's `.gitignore` block that dashcrew owns;
  - records the link target and the clone's HEAD sha in the lock;
  - warns, without failing, when the clone is missing ("run dashcrew knowledge pull");
  - on Windows without symlinks, copies the clone and marks the lock entry `copied`.
- `doctor`:
  - reports the link state and the clone's age (the time of its last pull);
  - with `--fetch`, reports how many commits the clone is behind its remote.

### K2: guard

- Every write rule resolves the real path first, following symlinks for the existing part of the path.
- A write whose real path is inside the knowledge clone is allowed only under `inbox/`, for every agent and capability. Reading is never blocked.
- Tests:
  - through the link;
  - through the absolute clone path;
  - through `..`;
  - through a second symlink;
  - a write to `inbox/` allowed;
  - a write to `features/` blocked;
  - `apply_patch` and shell redirections (`> .ai/knowledge/features/x.md`) the same way.
- Latency stays within the G4b numbers; real-path resolution costs a few stat calls.

### K3: retro writes proposals

- `learn` (and the `continuous-learning` skill) gain a knowledge branch. When `knowledge` is configured, a lesson about the product (a flow, a contract, a trap, a decision and its reason) becomes one proposal file in `.ai/knowledge/inbox/`:
  - front matter: `kind` (`feature` or `repo`), `target` (the page slug), `repo`, `branch`, `commit`;
  - body: the fact, with code pointers.
- A lesson that is really a rule, a standing instruction for agents, is not written to knowledge. It is proposed for the project pack instead.
- Without `knowledge`, `learn` behaves as today.

### K4: review

`dashcrew knowledge review` walks `inbox/` one proposal at a time: accept, edit, skip, delete.

- Accept:
  - merges the body into the target page;
  - adds the verified marker;
  - creates the page from the template when it does not exist;
  - removes the proposal.
- Edit opens `$EDITOR` on the proposal, then offers accept again.
- After the walk, `README.md` (the index) is regenerated: pages grouped by repo and tag, with each page's first sentence and its newest verified date.
- The command never commits or pushes. It ends by printing the `git` commands for the member to run on the knowledge repo, so the team reviews the change as a PR.

### K5: staleness

`dashcrew knowledge doctor`, also run as one `doctor` check when `knowledge` is configured, reports:

- code pointers whose path no longer exists at HEAD, for repos it can resolve: the current project, and other repos found as siblings or in P7 tracks later;
- proposals older than 14 days in `inbox/`;
- pages with no verified marker newer than 180 days.

It never edits pages.

### K6: MCP server

- `dashcrew knowledge serve`: stdio MCP server with the official Go SDK (`github.com/modelcontextprotocol/go-sdk`, a release at least two weeks old).
- Tools:
  - `knowledge_search(query, repo?, limit?)`: BM25 over title, headings and body, field-weighted. It returns the page path, title, the best matching snippet and the score.
  - `knowledge_read(page)`: the page text, with its verified markers listed.
  - `knowledge_sources(page)`: the code pointers as `repo:path:line`, each marked found or missing for repos it can resolve.
- The index is in memory, built at start, and rebuilt when the clone's HEAD changes (checked on each call by reading the git HEAD file).
- `sync` registers the server for every configured agent through the existing owned-MCP merge, only when `knowledge` is set.

### K7: catalog

AGENTS.md gets one short line in the toolkit section: when `knowledge` is configured, read `.ai/knowledge` (or search it) for the feature before planning, and verify its code pointers before relying on them.

- The heaviest AGENTS.md is 22,526 bytes in the goldens, and the heaviest pack combination is 23,792 against Antigravity's 24,000. The line is paid for by cutting the same number of bytes elsewhere in the same file.
- `explorer`: search knowledge before sweeping code, and say which pages it used.
- `plan` workflow: the plan names the knowledge pages it relied on.
- README: a "Shared knowledge" section (setup, pull, review, serve, the guard rule).

## How we check

- Unit tests per stage as listed; `go vet ./...` clean, `go test ./...` passing.
- Goldens:
  - a new sample `sample-knowledge` with `knowledge` set renders the link entry in the lock, the MCP registration for each agent, the AGENTS.md line and the `learn` branch;
  - every existing golden is unchanged except for the bytes cut to pay for the AGENTS.md line.
- MCP: an in-process client from the SDK calls all three tools against a fixture knowledge repo.
- Search quality: 20 fixture questions over a fixture repo of about 40 pages; the right page must be in the top 3 for at least 17 of them.
- Live:
  1. `knowledge init` a private test repo;
  2. `pull` it in the sample app;
  3. run one task to its retro;
  4. `review` the proposal;
  5. ask the agent a question that the accepted page answers;
  6. check that it used `knowledge_search` or the files, and checked the pointer.

## Not doing

- Embeddings, a vector store, or any database. The index is derived and in memory.
- Automatic commits or pushes on the knowledge repo, and automatic accept.
- A static site. The generated `README.md` is the book.
- The personal profile (`me.yaml`) and role-trimmed skill links. They come later; P5 only adds the knowledge link.
- Multi-repo pointer resolution beyond the current project and sibling clones (P7 tracks extend it).
- Moving rules into knowledge.

## Decisions (confirmed)

1. `sync` never touches the network; `dashcrew knowledge pull` clones or fast-forwards.
2. Accept merges the proposal into the target page automatically, with a verified marker; `edit` opens `$EDITOR` on the proposal first.
3. With `knowledge` configured and no local clone, `sync` warns and continues; `doctor` reports it too.
4. The MCP server is registered for every configured agent whenever `knowledge` is set.

## Progress

- K0-K2 done.
  - Package `internal/knowledge`: a page parser (never fails, lists `Problems`), `Init`, `Pull`, `Fetch`, `Behind`, `HeadSHA` and `LastPull`.
  - Commands: `dashcrew knowledge init <dir>` (refuses a non-empty folder) and `dashcrew knowledge pull` (fast-forward only; a diverged clone exits 1 and is never reset). `doctor --fetch` reports commits behind the remote.
  - Profile `knowledge: {repo, name}`. Lock `knowledge: {name, link, state, target, head}`. guard.json `knowledge_clone: ~/.dashcrew/knowledge/<name>`.
  - New `.gitignore` block (`# dashcrew:begin` / `# dashcrew:end`) holding `/.ai/knowledge`.
  - Guard: write rules run on the real path, and only `inbox/` in the clone is writable. Shell write targets are now checked: redirections, tee, cp/mv/ln/install/rsync, rm, rmdir, touch, mkdir, truncate, `dd of=`, `sed -i`, `find -delete/-exec`, `cd`/`pushd`, and `~`/`$HOME`.
  - Blind spots, pinned in `TestKnowledgeShellBlindSpots`: interpreters (`python -c`, `perl -pi`), xargs operands, variables other than HOME, `tar -C`, `git -C`, and Codex `workdir`.
  - Latency about 4 ms.
  - Golden `sample-knowledge`; transcripts `knowledge-init`, `knowledge-pull-and-sync` and `sample-knowledge`. Only `usage.txt` changed among the old ones (the help gains `knowledge` and `doctor --fetch`).
  - Note (2 Nov): `go test ./...` took about 16 min of wall time at 6% CPU, while each package reported at most 26 s. The cause is on the machine, not in the code: one package ran in 15 min once and in 1.1 s on the next run. Later runs took about 21 s again.
- K3-K5 done.
  - K3: `learn/20-knowledge.md` and `continuous-learning/20-knowledge.md`, gated by the new template boolean `has_knowledge`.
    - A product lesson becomes `.ai/knowledge/inbox/<date>-<slug>.md` (front matter `kind`, `target`, `repo`, `branch`, `commit`). A standing instruction for agents goes to `.ai/packs/`.
    - Without `knowledge`, output is unchanged.
  - K4: `knowledge review [--clone] [-p]`.
    - Accept merges into `features|repos/<target>.md`, under the proposal's heading or `## Notes`, with the verified marker. A missing page is made from the template (front matter and headings only).
    - Malformed proposals are reported and kept.
    - The answers can be piped. The clock comes from `SOURCE_DATE_EPOCH`.
    - The README index (from `## Pages`) is grouped by repo, then tag. Git commands are printed, never run.
  - K5: `knowledge doctor` prints `pointer`, `unchecked`, `proposal` and `page` lines, and exits 1 only on a stale pointer. Paths are checked via `git ls-tree HEAD`. `dashcrew doctor` gets a warn-level `knowledge-stale` line.
  - New transcripts `knowledge-review` and `knowledge-doctor`. 21 new tests. Full suite passes in about 21 s.
- K6-K7 done.
  - `dashcrew knowledge serve` (`internal/knowledgemcp`) uses `github.com/modelcontextprotocol/go-sdk` v1.8.0. Tools: `knowledge_search`, `knowledge_read` and `knowledge_sources`. Each returns text and structured JSON, and each description says to verify pointers.
  - BM25 (title 3, headings 2, body 1; k1 1.2, b 0.75; stop words and suffix stripping) finds the right page in the top 3 for 20 of 20 fixture questions over 40 pages.
  - The index rebuilds when the clone's HEAD file changes; no git process runs. `PageFile` rejects absolute paths, `..`, backslashes, `.git/`, non-`.md` files and symlinks that escape the clone.
  - The server is registered as `dashcrew-knowledge` in `.mcp.json`, `.codex/config.toml` and `.agents/mcp_config.json`, and doctor expects it. `MergeJSONMcpServers` now removes owned servers that are no longer listed.
  - AGENTS.md: with knowledge set, the memory line becomes one combined line that also covers `.ai/knowledge`, at no byte cost. A test enforces that. The heaviest combination stays at 23,791 bytes or less.
  - `explorer` and `plan` get knowledge-gated lines. README gains a "Shared knowledge" section.
  - Only `sample-knowledge` changed among the goldens (antigravity was added to its profile). 23 new tests. The full suite passes in about 23 s.

P5 is complete in code. Left: the live check in "How we check" (`knowledge init` a private test repo, pull, retro, review, then ask the agent).
