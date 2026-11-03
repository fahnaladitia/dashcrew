package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/knowledge"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

// The CLI transcript test runs the README's commands through the built binary in temp folders and compares, per
// step, the exit code, stdout, stderr and the files the step added, changed or removed with a recorded transcript
// in testdata/transcripts/. Only the temp root is normalized, to <root>. The transcripts were recorded while the
// output was byte-identical to the Dart engine (G8), so they pin the help text, messages and exit codes.
//
// Rewrite them after an intended change with `DASHCREW_UPDATE_GOLDENS=1 go test ./cmd/dashcrew -run Transcript`,
// then review the diff.

// cliEnvironment is the binary under test and the PATH folder it runs with.
type cliEnvironment struct {
	binary, bin string
}

// step is one CLI call; dir is relative to the side root ("w" when empty, "." for the root itself).
type step struct {
	args   []string
	dir    string
	stdin  string
	env    map[string]string
	before func(t *testing.T, side side)
}

type cliCase struct {
	name  string
	setup func(t *testing.T, side side)
	steps []step
}

// side is the temp folder a case runs in.
type side struct {
	root string
	env  []string
}

func (s side) path(relative string) string {
	return filepath.Join(s.root, filepath.FromSlash(relative))
}

func (s side) write(t *testing.T, relative, content string) {
	t.Helper()
	writeFile(t, s.path(relative), content)
}

func (s side) git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = s.path(dir)
	cmd.Env = s.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func setUpCLI(t *testing.T) cliEnvironment {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the transcripts use POSIX sh stubs (gh, $EDITOR) and Unix paths")
	}
	repo := repoRoot(t)
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := cliEnvironment{binary: filepath.Join(work, "dashcrew"), bin: filepath.Join(work, "bin")}
	// -buildvcs=false keeps the module version unstamped, so the binary reports and locks version.Unstamped.
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-buildvcs=false", "-o", e.binary, "./cmd/dashcrew")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	// PATH holds git and the system tools only: no dashcrew, flutter or fvm. A runner may ship gh in /usr/bin (Ubuntu
	// does), so the bin folder, first on PATH, holds a gh stub that fails: every OS then sees gh missing or failing.
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(e.bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(gitPath, filepath.Join(e.bin, "git")); err != nil {
		t.Fatal(err)
	}
	ghStub := "#!/bin/sh\necho 'gh stub: the transcripts run without gh' >&2\nexit 127\n"
	if err := os.WriteFile(filepath.Join(e.bin, "gh"), []byte(ghStub), 0o755); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e cliEnvironment) newSide(t *testing.T, root string) side {
	t.Helper()
	for _, dir := range []string{"home", "w"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	const date = "2026-01-01T00:00:00Z"
	return side{root: root, env: []string{
		"PATH=" + e.bin + ":/usr/bin:/bin",
		"HOME=" + filepath.Join(root, "home"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_AUTHOR_DATE=" + date,
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com", "GIT_COMMITTER_DATE=" + date,
	}}
}

type outcome struct {
	code           int
	stdout, stderr string
}

func (e cliEnvironment) run(t *testing.T, s side, st step) outcome {
	t.Helper()
	cmd := exec.Command(e.binary, st.args...)
	dir := st.dir
	if dir == "" {
		dir = "w"
	}
	cmd.Dir = s.path(dir)
	cmd.Env = append([]string{}, s.env...)
	for key, value := range st.env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stdin = strings.NewReader(st.stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return outcome{code, normalize(stdout.String(), s.root), normalize(stderr.String(), s.root)}
}

func normalize(text, root string) string { return strings.ReplaceAll(text, root, "<root>") }

// keptGitFile names the files under a .git directory the comparison reads: config, the exclude file, the
// watch state and the verify record; objects, logs and the index differ by timestamps and are skipped.
func keptGitFile(relative string) bool {
	_, inGit, ok := strings.Cut(relative, ".git/")
	if !ok {
		return true
	}
	// A knowledge clone under home is made by a step, so its whole git folder is new: git's initial content.
	if strings.HasPrefix(relative, "home/") {
		return false
	}
	return inGit == "config" || inGit == "info/exclude" || inGit == "dashcrew-watch/state.json" || inGit == "dashcrew/last-verify"
}

// snapshot maps every file, link and folder under the side root to a normalized description.
func snapshot(t *testing.T, s side) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(s.root, path)
		relative = filepath.ToSlash(relative)
		if relative == "." {
			return nil
		}
		if relative == "origin.git" {
			return filepath.SkipDir
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			out[relative] = "link -> " + normalize(target, s.root)
		case info.IsDir():
			if strings.Contains(relative+"/", ".git/") {
				if _, inGit, _ := strings.Cut(relative+"/", ".git/"); inGit != "" && inGit != "info/" && inGit != "dashcrew-watch/" && inGit != "dashcrew/" {
					return filepath.SkipDir
				}
				return nil
			}
			out[relative] = "dir"
		default:
			if !keptGitFile(relative) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[relative] = fmt.Sprintf("%v %s", info.Mode().Perm(), normalize(string(data), s.root))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(from, path)
		target := filepath.Join(to, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
}

// ---- fixtures ----

func golden(t *testing.T, parts ...string) string {
	return filepath.Join(append([]string{repoRoot(t), "test", "golden"}, parts...)...)
}

// sample is a golden sample's profile with its project packs and platforms, as the conformance test sets it up.
func sample(name string) func(t *testing.T, s side) {
	return func(t *testing.T, s side) {
		data, err := os.ReadFile(golden(t, name, "project.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		s.write(t, "w/.ai/project.yaml", string(data))
		for _, folder := range []string{"packs", "platforms"} {
			if _, err := os.Stat(golden(t, name, folder)); err == nil {
				copyDir(t, golden(t, name, folder), s.path("w/.ai/"+folder))
			}
		}
	}
}

func initApp(t *testing.T, s side) {
	copyDir(t, golden(t, "init-app", "input"), s.path("w"))
	s.git(t, "w", "init", "-q", "-b", "main")
	s.git(t, "w", "add", "-A")
	s.git(t, "w", "commit", "-q", "-m", "init")
	for _, branch := range []string{"main", "develop", "feature/x"} {
		s.git(t, "w", "update-ref", "refs/remotes/origin/"+branch, "HEAD")
	}
}

func noPlatform(t *testing.T, s side) { s.write(t, "w/Makefile", "lint:\n") }

const goPlatformYaml = "detect: [go.mod]\ncommands: { analyze: go vet ./..., test: go test ./..., test_scoped: go test, deps: go mod download }\n"

func projectPlatform(t *testing.T, s side) {
	s.write(t, "w/go.mod", "module example.com/app\n")
	s.write(t, "w/.ai/platforms/go/platform.yaml", goPlatformYaml)
	s.write(t, "w/.ai/platforms/go/packs/router/chi/pack.yaml", "description: chi\n")
}

func twoPlatforms(t *testing.T, s side) {
	projectPlatform(t, s)
	s.write(t, "w/pubspec.yaml", "name: app\ndependencies:\n  flutter_bloc: ^9.0.0\n  dio: ^5.0.0\n  http: ^1.0.0\n")
}

// repo is internal/worktree's fixture: origin.git, a clone in w with main and develop pushed, and the
// git-ignored toolkit written in the main checkout.
func repo(profileYaml string) func(t *testing.T, s side) {
	return func(t *testing.T, s side) {
		if err := os.Remove(s.path("w")); err != nil {
			t.Fatal(err)
		}
		s.git(t, ".", "init", "-q", "--bare", "-b", "main", "origin.git")
		s.git(t, ".", "clone", "-q", s.path("origin.git"), "w")
		s.write(t, "w/.gitignore", "AGENTS.md\nCLAUDE.md\n.claude/\n.codex/\n.agents/\n.ai/bin/\ndocs/plans/\n")
		s.write(t, "w/.ai/project.yaml", profileYaml)
		s.write(t, "w/docs/readme.md", "docs\n")
		s.write(t, "w/lib/a.dart", "void a() {}\n")
		s.write(t, "w/Makefile", "deps:\n")
		s.git(t, "w", "add", "-A")
		s.git(t, "w", "commit", "-q", "-m", "init")
		s.git(t, "w", "push", "-q", "origin", "main")
		s.git(t, "w", "push", "-q", "origin", "main:develop")
		s.git(t, "w", "fetch", "-q", "origin")
		s.write(t, "w/AGENTS.md", "rules\n")
		s.write(t, "w/CLAUDE.md", "@AGENTS.md\n")
		s.write(t, "w/.claude/settings.json", "{}\n")
		s.write(t, "w/.codex/hooks.json", "{}\n")
		s.write(t, "w/.agents/skills/plan/SKILL.md", "plan\n")
		s.write(t, "w/.ai/bin/guard", "binary\n")
		s.write(t, "w/docs/plans/x/plan.md", "plan\n")
	}
}

const repoProfile = "toolkit: 1\nagents: [claude-code]\npaths: { plans: docs/plans }\n"

const knowledgeProfile = "toolkit: 1\nagents: [claude-code]\nknowledge: { repo: ../origin.git, name: notes }\n"

// knowledgeRepo is a knowledge repo laid out by `dashcrew knowledge init`, pushed to origin.git, with its working
// copy in seed; w holds a profile whose knowledge is origin.git.
func knowledgeRepo(t *testing.T, s side) {
	s.write(t, "seed/README.md", "# Knowledge\n")
	s.write(t, "seed/features/share.md", "---\ntitle: Note sharing\nrepos: [notes-api]\ntags: [sharing]\n---\n\n# Note sharing\n")
	s.write(t, "seed/inbox/.gitkeep", "")
	s.git(t, "seed", "init", "-q", "-b", "main")
	s.git(t, "seed", "add", "-A")
	s.git(t, "seed", "commit", "-q", "-m", "init")
	s.git(t, ".", "clone", "-q", "--bare", s.path("seed"), "origin.git")
	s.git(t, "seed", "remote", "add", "origin", s.path("origin.git"))
	s.git(t, "seed", "fetch", "-q", "origin")
	s.write(t, "w/.ai/project.yaml", knowledgeProfile)
}

// recorded is the clock of the knowledge review and doctor steps: 2026-03-01T00:00:00Z.
var recorded = map[string]string{"SOURCE_DATE_EPOCH": "1772323200"}

// knowledgeProposal is an inbox proposal about notes-api with body below the front matter.
func knowledgeProposal(kind, target, body string) string {
	return "---\nkind: " + kind + "\ntarget: " + target + "\nrepo: notes-api\nbranch: feat/notes\ncommit: 1a2b3c4d5e6f\n---\n\n" + body
}

const profileClone = "home/.dashcrew/knowledge/notes/"

// knowledgeInbox is the profile's clone laid out by `knowledge init` with one page and six proposals, and a fake
// editor in the root that fixes a typo in the file it is given.
func knowledgeInbox(t *testing.T, s side) {
	s.write(t, "w/.ai/project.yaml", knowledgeProfile)
	if _, err := knowledge.Init(s.path(profileClone)); err != nil {
		t.Fatal(err)
	}
	s.write(t, profileClone+"features/share.md", "---\ntitle: Note sharing\nrepos: [notes-api, notes-app]\ntags: [sharing]\n---\n\n"+
		"# Note sharing\n\n## Purpose\n\nShare a note with a link. <!-- verified notes-api@abcdef1 2026-01-10 -->\n\n## Gotchas\n")
	s.write(t, profileClone+"inbox/2026-02-01-expiry.md", knowledgeProposal("feature", "share",
		"## Gotchas\n\nShare links expire after 7 days; the API refuses older tokens. `notes-api:src/share/token.go:42`\n"))
	s.write(t, profileClone+"inbox/2026-02-02-tags.md", knowledgeProposal("feature", "tags", "## Purpose\n\nTags group notes across folders.\n"))
	s.write(t, profileClone+"inbox/2026-02-03-broken.md", "---\nkind: flow\ntarget: share\n---\n\nNo repo.\n")
	s.write(t, profileClone+"inbox/2026-02-04-setup.md", knowledgeProposal("repo", "notes-api", "Run the API with `make devv`.\n"))
	s.write(t, profileClone+"inbox/2026-02-05-later.md", knowledgeProposal("feature", "share", "Maybe later.\n"))
	s.write(t, profileClone+"inbox/2026-02-06-noise.md", knowledgeProposal("feature", "share", "Noise.\n"))
	s.write(t, "editor.sh", "#!/bin/sh\nsed 's/devv/dev/' \"$1\" > \"$1.tmp\" && mv \"$1.tmp\" \"$1\"\n")
	if err := os.Chmod(s.path("editor.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// knowledgeStale is a project w whose origin is notes-app, a sibling clone notes-api, and the profile's clone with
// a stale pointer, an unchecked repo, an old proposal and two unverified pages.
func knowledgeStale(t *testing.T, s side) {
	s.write(t, "w/.ai/project.yaml", knowledgeProfile)
	s.write(t, "w/src/app.go", "package app\n")
	s.git(t, "w", "init", "-q", "-b", "main")
	s.git(t, "w", "remote", "add", "origin", "git@example.com:notes-team/notes-app.git")
	s.git(t, "w", "add", "src")
	s.git(t, "w", "commit", "-q", "-m", "app")
	s.write(t, "notes-api/src/share/token.go", "package share\n")
	s.git(t, "notes-api", "init", "-q", "-b", "main")
	s.git(t, "notes-api", "add", "-A")
	s.git(t, "notes-api", "commit", "-q", "-m", "api")
	if _, err := knowledge.Init(s.path(profileClone)); err != nil {
		t.Fatal(err)
	}
	s.write(t, profileClone+"features/share.md", "---\ntitle: Note sharing\nrepos: [notes-api, notes-app]\n---\n\n"+
		"The app opens `notes-app:src/app.go:1` and `notes-app:src/share.go:3`; the API checks `notes-api:src/share/token.go:4`.\n"+
		"The web client is `notes-web:src/share.ts:9`. <!-- verified notes-app@abcdef1 2026-02-01 -->\n")
	s.write(t, profileClone+"repos/notes-app.md", "---\ntitle: notes-app\nrepos: [notes-app]\n---\n\nBuilt with make. <!-- verified notes-app@abcdef1 2025-06-01 -->\n")
	s.write(t, profileClone+"features/tags.md", "---\ntitle: Tags\nrepos: [notes-app]\n---\n\nTags group notes.\n")
	s.write(t, profileClone+"inbox/2026-02-01-old.md", "old\n")
	s.write(t, profileClone+"inbox/2026-02-25-new.md", "new\n")
}

// specProject is w on branch feat/notes-sync, which records release 1.4.0, with a knowledge profile and the
// profile's clone laid out by knowledge init.
func specProject(t *testing.T, s side) {
	s.write(t, "w/.ai/project.yaml", knowledgeProfile)
	s.git(t, "w", "init", "-q", "-b", "main")
	s.git(t, "w", "add", "-A")
	s.git(t, "w", "commit", "-q", "-m", "chore: init")
	s.git(t, "w", "checkout", "-q", "-b", "feat/notes-sync")
	s.git(t, "w", "config", "branch.feat/notes-sync.dashcrew-release", "1.4.0")
	if _, err := knowledge.Init(s.path(profileClone)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.path("kb"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// specFile is a spec file of the notes app with the given status, rev and body.
func specFile(feature, status, rev, body string) string {
	return "---\ntitle: Notes sync\nfeature: " + feature + "\nrelease: \"1.4.0\"\nstatus: " + status + "\nrev: " + rev +
		"\nrepos: [notes-api, notes-app]\nowners: [product]\n---\n\n" + body
}

const (
	specPRD = "# Notes sync: PRD\n\n## Acceptance criteria\n\n- AC-1 Given two devices, when a note changes on one, then the other shows it.\n" +
		"- ~~AC-2~~ dropped: offline merge moves to a later release\n- AC-3 Given no network, when the user edits, then the edit is kept.\n"
	specTRD = "# Notes sync: TRD\n\n## Contracts\n\n- C-1 `GET /notes/changes?since=<cursor>`: a page of changes.\n\n" +
		"## Decisions\n\n- D-1 Sync by cursor, not by timestamp: clocks drift.\n\n## Open questions\n\n- How long are cursors kept? owner: backend\n"
)

// specClone is the profile's clone as a git repo on main with an approved spec of notes-sync committed.
func specClone(t *testing.T, s side) {
	s.write(t, "w/.ai/project.yaml", knowledgeProfile)
	if _, err := knowledge.Init(s.path(profileClone)); err != nil {
		t.Fatal(err)
	}
	s.write(t, profileClone+"specs/1.4.0/notes-sync/prd.md", specFile("notes-sync", "approved", "1", specPRD))
	s.write(t, profileClone+"specs/1.4.0/notes-sync/trd.md", specFile("notes-sync", "approved", "1", specTRD))
	s.git(t, profileClone, "init", "-q", "-b", "main")
	s.git(t, profileClone, "add", "-A")
	s.git(t, profileClone, "commit", "-q", "-m", "spec: notes-sync")
}

// specStatusProject is specClone with w a git repo whose origin is notes-app, on branch feat/sync-client, and a plan
// that claims AC-1 of the notes-sync spec.
func specStatusProject(t *testing.T, s side) {
	specClone(t, s)
	s.write(t, "w/.ai/plans/1.4.0/sync-client/plan.md", "# Sync client\n\nSpec: specs/1.4.0/notes-sync@rev1\n\n"+
		"Covers AC-1; offline edits are another task.\n\n## Verification map\n\nAC-1  test/sync_test  pulls changes by cursor\n")
	s.write(t, "w/.ai/plans/notes-search/plan.md", "# Search\n\nSpec: none\n\nAC-1  test/search_test  finds a note\n")
	s.git(t, "w", "init", "-q", "-b", "main")
	s.git(t, "w", "remote", "add", "origin", "git@example.com:notes-team/notes-app.git")
	s.git(t, "w", "add", "-A")
	s.git(t, "w", "commit", "-q", "-m", "chore: init")
	s.git(t, "w", "checkout", "-q", "-b", "feat/sync-client")
}

// pushPage commits a page in seed and pushes it to origin.git.
func pushPage(file string) func(t *testing.T, s side) {
	return func(t *testing.T, s side) {
		s.write(t, "seed/"+file, file+"\n")
		s.git(t, "seed", "add", "-A")
		s.git(t, "seed", "commit", "-q", "-m", file)
		s.git(t, "seed", "push", "-q", "origin", "main")
	}
}

// guardWrite is a Claude Code Write hook payload for path.
func guardWrite(path string) step {
	return step{args: []string{"guard"}, stdin: `{"tool_name":"Write","tool_input":{"file_path":"` + path + `","content":"x"}}`}
}

const commitRepoProfile = "toolkit: 1\nagents: [claude-code]\nproject:\n  commit:\n" +
	"    scopes: { 'lib/features/cart/**': cart, 'lib/core/**': core }\n    ticket: { from_branch: '([A-Z]+-[0-9]+)' }\n"

// commitRepo is a repo in w on branch fix/ABC-123-cart-total, with one staged file, one unstaged edit, an edit to a
// file whose name needs quotes, and two untracked files.
func commitRepo(t *testing.T, s side) {
	s.write(t, "w/.ai/project.yaml", commitRepoProfile)
	s.write(t, "w/lib/features/cart/cart_cubit.dart", "class CartCubit {}\n")
	s.write(t, "w/lib/features/cart/it's $1.dart", "x\n")
	s.write(t, "w/README.md", "readme\n")
	s.git(t, "w", "init", "-q", "-b", "main")
	s.git(t, "w", "add", "-A")
	s.git(t, "w", "commit", "-q", "-m", "chore: init")
	s.git(t, "w", "checkout", "-q", "-b", "fix/ABC-123-cart-total")
	s.write(t, "w/lib/features/cart/cart_page.dart", "class CartPage {}\n")
	s.git(t, "w", "add", "lib/features/cart/cart_page.dart")
	s.write(t, "w/lib/features/cart/cart_cubit.dart", "class CartCubit { int total = 0; }\n")
	s.write(t, "w/lib/features/cart/it's $1.dart", "y\n")
	s.write(t, "w/test/cart_cubit_test.dart", "void main() {}\n")
	s.write(t, "w/notes.md", "draft\n")
}

// commitOnBranch writes file and commits it with message in w.
func commitOnBranch(file, message string) func(t *testing.T, s side) {
	return func(t *testing.T, s side) {
		s.write(t, "w/"+file, message+"\n")
		s.git(t, "w", "add", "-A")
		s.git(t, "w", "commit", "-q", "-m", message)
	}
}

const linesProfile = "toolkit: 1\nagents: [claude-code]\nproject:\n" +
	"  release_lines: { main: develop, release: 'release/(?P<version>[0-9.]+)', hotfix: 'hotfix/(?P<version>[0-9.]+)' }\n" +
	"  branch_prefix:\n    feat: feat/\n    hotfix: hotfix/\n    fix: fix/\n"

// linesRepo is w on develop with the release lines profile committed.
func linesRepo(t *testing.T, s side) {
	s.write(t, "w/.ai/project.yaml", linesProfile)
	s.write(t, "w/README.md", "readme\n")
	s.git(t, "w", "init", "-q", "-b", "develop")
	s.git(t, "w", "add", "-A")
	s.git(t, "w", "commit", "-q", "-m", "chore: init")
}

// releaseLinesRepo adds release/2.4.0 with a fix, hotfix/2.4.5 cut from it, release/2.5.0 only on the remote,
// a branch that records release 2.4.5 and one cut from release/2.4.0 that records nothing.
func releaseLinesRepo(t *testing.T, s side) {
	linesRepo(t, s)
	s.git(t, "w", "checkout", "-q", "-b", "release/2.4.0")
	commitOnBranch("fix.txt", "fix: totals")(t, s)
	s.git(t, "w", "checkout", "-q", "-b", "hotfix/2.4.5")
	commitOnBranch("hot.txt", "fix: crash")(t, s)
	s.git(t, "w", "checkout", "-q", "develop")
	commitOnBranch("feature.txt", "feat: next")(t, s)
	s.git(t, "w", "update-ref", "refs/remotes/origin/release/2.5.0", "HEAD")
	s.git(t, "w", "branch", "fix/recorded", "develop")
	s.git(t, "w", "config", "branch.fix/recorded.dashcrew-release", "2.4.5")
	s.git(t, "w", "checkout", "-q", "-b", "fix/guess", "release/2.4.0")
	commitOnBranch("guess.txt", "fix: guess")(t, s)
	s.git(t, "w", "checkout", "-q", "develop")
}

// portsRepo holds release/2.4.0 with five fixes, a feature and a chore marked Port: yes; hotfix/2.4.5 cut from it
// with one fix; and release/2.5.0 cut from develop, where one fix is ported by trailer, one by cherry-pick -x, one by
// patch-id and one ported then reverted. A task of 2.4.5 has a plan folder, a recorded branch and a PR description.
func portsRepo(t *testing.T, s side) {
	linesRepo(t, s)
	s.git(t, "w", "checkout", "-q", "-b", "release/2.4.0")
	for _, c := range [][2]string{
		{"m.txt", "fix: totals round down"}, {"t.txt", "fix: empty cart crash"}, {"c.txt", "fix: wrong currency"},
		{"p.txt", "fix(cart): stale badge"}, {"l.txt", "fix: double charge"}, {"n.txt", "feat: gift cards"},
	} {
		commitOnBranch(c[0], c[1])(t, s)
	}
	commitOnBranch("y.txt", "chore: rotate the signing key\n\nPort: yes")(t, s)
	short := func(rev string) string { return s.git(t, "w", "rev-parse", "--short=7", rev) }
	trailer, pick, lost := short("HEAD~5"), short("HEAD~4"), short("HEAD~2")
	s.git(t, "w", "checkout", "-q", "-b", "hotfix/2.4.5")
	commitOnBranch("h.txt", "fix: login loop")(t, s)
	s.git(t, "w", "branch", "fix/login-loop")
	s.git(t, "w", "config", "branch.fix/login-loop.dashcrew-release", "2.4.5")
	s.git(t, "w", "checkout", "-q", "develop")
	commitOnBranch("b.txt", "feat: next")(t, s)
	s.git(t, "w", "checkout", "-q", "-b", "release/2.5.0")
	s.write(t, "w/t.txt", "fix: empty cart crash, adapted\n")
	s.git(t, "w", "add", "-A")
	s.git(t, "w", "commit", "-q", "-m", "fix: empty cart crash\n\nPorted-from: "+trailer)
	s.git(t, "w", "cherry-pick", "-x", pick)
	s.write(t, "w/p.txt", "fix(cart): stale badge\n")
	s.git(t, "w", "add", "-A")
	s.git(t, "w", "commit", "-q", "-m", "fix: badge count after checkout")
	s.git(t, "w", "cherry-pick", lost)
	s.git(t, "w", "revert", "--no-edit", "HEAD")
	s.git(t, "w", "checkout", "-q", "develop")
	s.write(t, "w/.ai/plans/2.4.5/login-loop/plan.md", "plan\n")
	s.write(t, "w/.ai/plans/2.4.5/session-timeout/plan.md", "plan\n")
	s.write(t, "w/.ai/pr/fix/login-loop.md", "# fix: login loop\n")
	s.write(t, "w/.git/info/exclude", ".ai/plans/\n.ai/pr/\n")
}

// olderLineToolkit makes w's lock read as written by dashcrew 0.0.0, whose catalog rendered AGENTS.md differently,
// and commits it.
func olderLineToolkit(t *testing.T, s side) {
	data, err := os.ReadFile(s.path("w/.ai/lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	agents, err := os.ReadFile(s.path("w/AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	older := strings.Replace(string(agents), "\n", "\nOlder catalog text.\n", 1)
	s.write(t, "w/AGENTS.md", older)
	lock := strings.Replace(string(data), `"dashcrew_version": "0.0.1"`, `"dashcrew_version": "0.0.0"`, 1)
	lock = strings.Replace(lock, sync.FileHash("AGENTS.md", string(agents)), sync.FileHash("AGENTS.md", older), 1)
	s.write(t, "w/.ai/lock.json", lock)
	s.git(t, "w", "add", "-A")
	s.git(t, "w", "commit", "-q", "-m", "chore: toolkit 0.0.0")
}

// editProfile replaces from with to in w's profile.
func editProfile(t *testing.T, s side, from, to string) {
	data, err := os.ReadFile(s.path("w/.ai/project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s.write(t, "w/.ai/project.yaml", strings.Replace(string(data), from, to, 1))
}

func cli(args ...string) step { return step{args: args} }

func usageSteps() []step {
	lines := [][]string{
		{}, {"--help"}, {"-h"}, {"help"}, {"help", "help"}, {"help", "init"}, {"help", "sync"}, {"help", "doctor"},
		{"help", "check"}, {"help", "worktree"}, {"help", "worktree", "create"}, {"help", "guard"}, {"help", "guard", "install"},
		{"--version"}, {"--version", "sync"}, {"sync", "--version"}, {"help", "--version"}, {"--version=1"}, {"--no-version"},
		{"help", "nope"}, {"help", "sync", "extra"}, {"help", "worktree", "nope"}, {"help", "-h"},
		{"init", "--help"}, {"sync", "-h"}, {"doctor", "--help"}, {"check", "--help"}, {"worktree", "--help"},
		{"worktree", "create", "--help"}, {"worktree", "list", "--help"}, {"worktree", "link", "--help"},
		{"worktree", "remove", "--help"}, {"worktree", "watch", "--help"}, {"guard", "--help"}, {"guard", "-h"},
		{"--help", "sync"}, {"-h", "worktree", "create"}, {"worktree", "-h", "create"}, {"sync", "--help", "extra"},
		{"nope"}, {"snyc"}, {"worktre"}, {"sink"}, {"chek"}, {"worktree"}, {"worktree", "nope"}, {"worktree", "craete"},
		{"sync", "--bogus"}, {"sync", "-x"}, {"sync", "-xy"}, {"init", "-yp"}, {"init", "-yz"}, {"init", "-yfoo"},
		{"init", "--agents", "bogus"}, {"init", "--agents", "codex,bogus"}, {"init", "--agents"}, {"init", "--no-yes"},
		{"sync", "--no-project"}, {"sync", "--no-bogus"}, {"doctor", "-p"}, {"sync", "--force=1"}, {"sync", "--help=1"},
		{"--", "sync"}, {"foo", "sync"}, {"help", "--project", "x"}, {"--project", "x"}, {"---x"}, {"--"}, {"-"},
		{"worktree", "create"}, {"worktree", "create", "-p"}, {"worktree", "list"},
		{"worktree", "watch", "--interval", "0"}, {"check", "--pr-body"}, {"check", "--pr-body", "missing.md"},
		{"check", "--pr-body", ""}, {"check", "--pr-body", "."},
	}
	steps := make([]step, len(lines))
	for i, args := range lines {
		steps[i] = cli(args...)
	}
	return steps
}

// toolkitSteps are the README's sync, doctor and check calls on a synced project, a PR body missing a section and
// one with every section, and the GitHub Actions annotations.
func toolkitSteps() []step {
	return []step{
		cli("sync"),
		cli("sync", "-p", "."),
		{args: []string{"sync", "--project", "w"}, dir: "."},
		cli("sync", "--compile-guard"),
		cli("sync", "--no-compile-guard"),
		cli("doctor"),
		{args: []string{"doctor", "-p", "w"}, dir: "."},
		cli("check"),
		{args: []string{"check", "--pr-body", "../body.md"}, before: func(t *testing.T, s side) { s.write(t, "body.md", "## Summary\n") }},
		{args: []string{"check", "--pr-body", "../body.md"}, env: map[string]string{"GITHUB_ACTIONS": "true"}},
		{args: []string{"check", "-p", ".", "--pr-body", "../full.md"}, before: func(t *testing.T, s side) {
			data, err := os.ReadFile(s.path("w/.github/pull_request_template.md"))
			if err != nil {
				t.Fatal(err)
			}
			s.write(t, "full.md", string(data))
		}},
	}
}

func appendTo(path, text string) func(t *testing.T, s side) {
	return func(t *testing.T, s side) {
		data, err := os.ReadFile(s.path(path))
		if err != nil {
			t.Fatal(err)
		}
		s.write(t, path, string(data)+text)
	}
}

func cliCases() []cliCase {
	cases := []cliCase{
		{name: "usage", steps: usageSteps()},
		{name: "init-app yes, refused, forced with agents", setup: initApp, steps: []step{
			cli("init", "--yes"), cli("init", "--yes"),
			cli("init", "-y", "--force", "--agents", "codex", "--agents", "antigravity"),
			cli("init", "-y", "--force", "--agents=codex,antigravity"),
		}},
		{name: "init-app from the parent folder", setup: initApp, steps: []step{
			{args: []string{"init", "-p", "w", "-y"}, dir: "."},
			{args: []string{"init", "--project=w/", "--yes", "--force"}, dir: "."},
		}},
		{name: "init-app interactive", setup: initApp, steps: []step{
			{args: []string{"init"}, stdin: "\n\n\ndio\n-\nnope\nother chopper\n\n\n\n\n\n\n\n\n"},
			{args: []string{"init", "--force"}, stdin: "go\n\r\nflutter\r\n"},
			{args: []string{"init", "--force"}},
		}},
		{name: "init-app synced", setup: initApp, steps: append([]step{cli("init", "--yes")}, toolkitSteps()...)},
		{name: "init without a platform", setup: noPlatform, steps: []step{
			cli("init", "--yes"),
			{args: []string{"init"}, stdin: "\nMakefile\n"},
			{args: []string{"init"}, stdin: "flutter\n"},
			{args: []string{"init"}, stdin: "nope\n\nMakefile  go.mod\nmake lint\n\nmake test\nmake one\nmake deps\nmake fmt\n"},
			cli("sync"),
			cli("doctor"),
		}},
		{name: "init with a project platform", setup: projectPlatform, steps: []step{
			cli("init", "--yes", "--agents", "codex"),
			{args: []string{"init", "--force"}, stdin: "\nchi\n\n\n"},
		}},
		{name: "init with two platforms", setup: twoPlatforms, steps: []step{
			cli("init", "-y"),
			{args: []string{"init"}},
			{args: []string{"init"}, stdin: "\ngo\n"},
			{args: []string{"init", "--force"}, stdin: "flutter\n"},
		}},
		{name: "no profile", steps: []step{
			cli("sync"), cli("doctor"), cli("check"), cli("worktree", "list"),
			cli("worktree", "create", "feat/a", "main"),
		}},
		{name: "adopt and force", setup: func(t *testing.T, s side) {
			s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code, codex]\n")
			s.write(t, "w/AGENTS.md", "# Team rules\n\nKeep it short.\n")
		}, steps: []step{
			cli("sync"),
			cli("sync", "--adopt"),
			{args: []string{"sync"}, before: appendTo("w/CLAUDE.md", "edited\n")},
			cli("doctor"),
			cli("sync", "--force"),
			{args: []string{"sync"}, before: func(t *testing.T, s side) {
				s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\npolicy: { plan_gate: true }\n")
			}},
			{args: []string{"sync"}, before: func(t *testing.T, s side) {
				s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [nope]\n")
			}},
			cli("doctor"),
			cli("check"),
		}},
		{name: "migrate p3", setup: func(t *testing.T, s side) { copyDir(t, golden(t, "sample-migrate-p3", "p3"), s.path("w")) },
			steps: []step{cli("doctor"), cli("sync"), cli("sync"), cli("doctor")}},
		{name: "worktree create, list, link, remove", setup: repo(repoProfile), steps: []step{
			cli("worktree", "list"),
			cli("worktree", "create", "feat/a", "origin/develop", "--no-pub"),
			cli("worktree", "list"),
			cli("worktree", "create", "feat/b", "main", "--name", "b-dir"),
			cli("worktree", "create", "feat/a", "main", "--no-pub"),
			cli("worktree", "create", "feat/c", "main", "--no-pub", "--name", "b-dir"),
			cli("worktree", "create", "bad..name", "main"),
			cli("worktree", "create", "feat/c", "origin/nope"),
			cli("worktree", "create", "feat/c"),
			{args: []string{"worktree", "create", "feat/c", "main", "-p", "w.worktrees/feat-a", "--no-pub"}, dir: "."},
			{args: []string{"worktree", "list", "-p", "lib"}, before: func(t *testing.T, s side) {
				os.Remove(s.path("w.worktrees/feat-a/CLAUDE.md"))
				s.write(t, "w.worktrees/feat-a/lib/a.dart", "void a() { }\n")
				s.git(t, "w.worktrees/b-dir", "branch", "--set-upstream-to=origin/main")
			}},
			cli("worktree", "link", "feat-a"),
			cli("worktree", "link", "b-dir"),
			cli("worktree", "link", "feat-a"),
			cli("worktree", "link"),
			cli("worktree", "link", "nope"),
			cli("worktree", "remove", "feat-a"),
			cli("worktree", "remove", "b-dir"),
			cli("worktree", "remove", "nope"),
			cli("worktree", "list"),
		}},
		{name: "worktree deps through the platform command", setup: repo(
			"toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { detect: [Makefile], commands: { analyze: make lint, test: make test, test_scoped: make one, deps: touch deps-ran, format: null } }\n"),
			steps: []step{
				cli("worktree", "create", "feat/a", "main"),
				{args: []string{"worktree", "create", "feat/b", "main", "--pub"}, before: func(t *testing.T, s side) {
					s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { detect: [lib/a.dart], commands: { analyze: a, test: b, test_scoped: c, deps: exit 3 } }\n")
				}},
				{args: []string{"worktree", "create", "feat/c", "main"}, before: func(t *testing.T, s side) {
					s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\nstack: { state: nope }\n")
				}},
				{args: []string{"worktree", "create", "feat/d", "main"}, before: func(t *testing.T, s side) {
					s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\nplatform: go\n")
					s.write(t, "w/.ai/platforms/go/platform.yaml", goPlatformYaml)
				}},
				{args: []string{"worktree", "create", "feat/e", "main"}, before: func(t *testing.T, s side) {
					os.Remove(s.path("w/.ai/project.yaml"))
					s.write(t, "w/pubspec.yaml", "name: app\n")
					s.git(t, "w", "add", "pubspec.yaml")
					s.git(t, "w", "commit", "-q", "-m", "pubspec")
				}},
			}},
		{name: "knowledge init", setup: func(t *testing.T, s side) {
			s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\n")
			if err := os.MkdirAll(s.path("w/empty"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, steps: []step{
			cli("knowledge", "init", "kb"),
			cli("knowledge", "init", "kb"),
			cli("knowledge", "init", "empty"),
			cli("knowledge", "init", "kb/README.md"),
			cli("knowledge", "init"),
			cli("knowledge"),
			cli("help", "knowledge", "pull"),
			cli("knowledge", "pull"),
		}},
		{name: "knowledge pull and sync", setup: knowledgeRepo, steps: []step{
			cli("sync"),
			cli("doctor"),
			guardWrite(".ai/knowledge/features/x.md"),
			cli("knowledge", "pull"),
			cli("sync"),
			cli("sync"),
			cli("knowledge", "pull"),
			guardWrite(".ai/knowledge/features/x.md"),
			guardWrite(".ai/knowledge/inbox/2026-01-01-share.md"),
			{args: []string{"guard"}, stdin: `{"tool_name":"Bash","tool_input":{"command":"cd .ai/knowledge && echo x >> features/share.md"}}`},
			{args: []string{"guard"}, stdin: `{"tool_name":"Bash","tool_input":{"command":"cp draft.md .ai/knowledge/inbox/"}}`},
			{args: []string{"knowledge", "pull", "-p", "w"}, dir: ".", before: pushPage("repos/notes-api.md")},
			cli("sync"),
			{args: []string{"doctor", "--fetch"}, before: pushPage("features/tags.md")},
			{args: []string{"knowledge", "pull"}, before: func(t *testing.T, s side) {
				s.write(t, "home/.dashcrew/knowledge/notes/inbox/2026-01-01-tags.md", "proposal\n")
				s.git(t, "home/.dashcrew/knowledge/notes", "add", "-A")
				s.git(t, "home/.dashcrew/knowledge/notes", "commit", "-q", "-m", "proposal")
				pushPage("features/export.md")(t, s)
			}},
			{args: []string{"sync"}, before: func(t *testing.T, s side) {
				s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\n")
			}},
		}},
		{name: "knowledge review", setup: knowledgeInbox, steps: []step{
			cli("help", "knowledge", "review"),
			{args: []string{"knowledge", "review"}, stdin: "a\nx\naccept\ne\na\ns\nd\n", env: map[string]string{
				"SOURCE_DATE_EPOCH": recorded["SOURCE_DATE_EPOCH"], "EDITOR": "../editor.sh"}},
			{args: []string{"knowledge", "review"}, env: recorded},
			{args: []string{"knowledge", "review"}, stdin: "e\nskip\n", env: recorded},
			{args: []string{"knowledge", "review", "--clone", "../" + profileClone}, stdin: "delete\n", env: recorded},
			{args: []string{"knowledge", "review", "--clone", "../nope"}, env: recorded},
		}},
		{name: "knowledge doctor", setup: knowledgeStale, steps: []step{
			cli("help", "knowledge", "doctor"),
			{args: []string{"knowledge", "doctor"}, env: recorded},
			{args: []string{"sync"}},
			{args: []string{"doctor"}, env: recorded},
			{args: []string{"knowledge", "doctor", "-p", "w", "--clone", profileClone}, dir: ".", env: recorded, before: func(t *testing.T, s side) {
				s.write(t, "w/src/share.go", "package app\n")
				s.git(t, "w", "add", "src")
				s.git(t, "w", "commit", "-q", "-m", "share")
			}},
			{args: []string{"knowledge", "doctor", "--clone", "../nope"}, env: recorded},
		}},
		{name: "spec new", setup: specProject, steps: []step{
			cli("help", "spec", "new"),
			cli("spec", "new", "notes-sync"),
			cli("spec", "new", "notes-sync"),
			cli("spec", "new", "shop-cart", "--release", "2.0.0", "--clone", "../kb"),
			{args: []string{"spec", "new", "todo", "--clone", profileClone}, dir: "."},
			cli("spec", "new", "Shop_Cart"),
			cli("spec", "new", "todo", "--release", "../x"),
			cli("spec", "new"),
			cli("spec", "new", "todo", "--clone", "../nope"),
		}},
		{name: "spec lint", setup: specClone, steps: []step{
			cli("help", "spec", "lint"),
			cli("spec", "lint"),
			{args: []string{"spec", "lint", "--against", "main"}, before: func(t *testing.T, s side) {
				s.write(t, profileClone+"specs/1.4.0/notes-sync/prd.md", specFile("notes-sync", "approved", "1",
					strings.Replace(strings.Replace(specPRD, "- AC-3 Given", "- AC-4 Given", 1), "- ~~AC-2~~ dropped: offline merge moves to a later release",
						"- AC-2 Given a conflict, when both devices edit, then both versions stay.", 1)))
			}},
			{args: []string{"spec", "lint", "../" + profileClone + "specs/1.5.0"}, before: func(t *testing.T, s side) {
				s.write(t, profileClone+"specs/1.5.0/shop-cart/prd.md", "---\ntitle: Shop cart\nfeature: shop-cart\nstatus: done\nrev: 1\n"+
					"repos: [shop-app]\nowners: [ana@example.com]\n---\n\n- AC-1 Given a cart, then a total.\n- AC-1 Given a coupon, then a discount.\n- AC-4 Given no stock, then a notice.\n- ~~AC-2~~ gone\n")
				s.write(t, profileClone+"specs/1.5.0/shop-cart/trd.md", specFile("shop-cart", "approved", "1",
					"## Open questions\n\n- Which payment provider?\n- Tax rules per country? owner: backend\n"))
			}},
			cli("spec", "lint", "../"+profileClone+"specs/1.5.0/shop-cart/trd.md"),
			{args: []string{"knowledge", "doctor"}, env: recorded},
			cli("spec", "lint", "--against", "nope"),
			cli("spec", "lint", "../missing"),
			cli("spec", "lint", "a", "b"),
		}},
		{name: "spec revise", setup: specClone, steps: []step{
			cli("help", "spec", "revise"),
			cli("spec", "revise", "../"+profileClone+"specs/1.4.0/notes-sync"),
			cli("spec", "lint", "--against", "main"),
			cli("spec", "revise", "../"+profileClone+"specs/1.4.0/notes-sync/prd.md"),
			cli("spec", "revise", "../"+profileClone+"features"),
			cli("spec", "revise"),
		}},
		{name: "spec status", setup: specStatusProject, steps: []step{
			cli("help", "spec", "status"),
			cli("spec", "status"),
			{args: []string{"spec", "status"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "config", "branch.feat/sync-client.dashcrew-spec", "specs/1.4.0/notes-sync@rev1")
			}},
			cli("spec", "revise", "../"+profileClone+"specs/1.4.0/notes-sync"),
			cli("spec", "status"),
			cli("sync"),
			{args: []string{"doctor"}, env: recorded},
			{args: []string{"spec", "status", "-p", "w"}, dir: ".", before: func(t *testing.T, s side) {
				s.git(t, "w", "config", "branch.feat/sync-client.dashcrew-spec", "specs/1.4.0/notes-sync@rev2")
				s.write(t, "w/.ai/plans/1.4.0/sync-offline/plan.md", "Spec: specs/1.4.0/notes-sync@rev2\n\nAC-3 and AC-7.\n")
			}},
			{args: []string{"spec", "status"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "config", "branch.feat/sync-client.dashcrew-spec", "notes-sync")
			}},
			{args: []string{"spec", "status"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "config", "branch.feat/sync-client.dashcrew-spec", "specs/1.4.0/gone@rev1")
			}},
			cli("spec", "status", "extra"),
		}},
		{name: "commit suggest", setup: commitRepo, steps: []step{
			cli("help", "commit"),
			cli("commit", "suggest", "--help"),
			cli("commit", "suggest"),
			cli("commit", "suggest", "--subject", "keep the $total `right` when it's \"0\" \u00e9", "--body", "Line one.\nLine two."),
			cli("commit", "suggest", "--json"),
			cli("verify", "record"),
			cli("commit", "suggest"),
			{args: []string{"commit", "suggest"}, before: func(t *testing.T, s side) {
				s.write(t, "w/lib/core/net.dart", "edited by hand\n")
				s.git(t, "w", "add", "lib/core/net.dart")
				s.write(t, "w/lib/features/cart/cart_cubit.dart", "class CartCubit { int total = 1; }\n")
			}},
			{args: []string{"commit", "suggest", "--scope", "", "--type", "docs", "--subject", "explain the cart"}, dir: "w/lib/features"},
			cli("commit", "suggest", "--type", "wip"),
			cli("commit", "suggest", "--subject", "Keep the total right on the cart page even when the count reaches zero"),
			{args: []string{"commit", "suggest"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "add", "-A")
				s.git(t, "w", "commit", "-q", "-m", "fix(cart): keep the total")
				s.git(t, "w", "checkout", "-q", "--detach")
			}},
			{args: []string{"commit", "suggest"}, dir: "."},
		}},
		{name: "commit lint", setup: func(t *testing.T, s side) {
			s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\nproject: { commit: { body: required, trailers: [Ported-from] } }\n")
			s.git(t, "w", "init", "-q", "-b", "main")
			commitOnBranch("a.txt", "chore: init\n\nFirst commit.")(t, s)
			s.git(t, "w", "checkout", "-q", "-b", "feat/x")
			commitOnBranch("b.txt", "feat(share): add links\n\nWhy.\n\nPorted-from: abc1234")(t, s)
			commitOnBranch("c.txt", "wip")(t, s)
			commitOnBranch("d.txt", "fix: ok\n\nWhy.\n\nCo-Authored-By: Claude <noreply@anthropic.com>")(t, s)
		}, steps: []step{
			cli("help", "commit", "lint"),
			cli("commit", "lint", "fix(cart): keep the total\n\nThe total reset on reload."),
			cli("commit", "lint", "Fix the cart"),
			cli("commit", "lint", "chore: x\n\n\U0001F916 Generated with an AI tool\n\nChange-Id: 1"),
			{args: []string{"commit", "lint", "-"}, stdin: "docs: explain lint\n\nBody.\n"},
			cli("commit", "lint", "fix: reply faster\n\nAsk ops@example.com, not @ana; see assets/icon@2x.png.\n\nSigned-off-by: Ana Lima <ana@example.com>"),
			cli("commit", "lint"),
			cli("commit", "lint", "a", "--range", "main..HEAD"),
			cli("commit", "lint", "--range", "main..HEAD"),
			cli("commit", "lint", "--range", "HEAD~1..HEAD"),
			cli("commit", "lint", "--range", "nope..HEAD"),
			{args: []string{"commit", "lint", "docs: no body"}, dir: "."},
			{args: []string{"commit", "lint", "--range", "main..HEAD"}, dir: "."},
		}},
		{name: "me", setup: func(t *testing.T, s side) {
			s.write(t, "w/.ai/project.yaml", "toolkit: 1\nplatform: go\n")
			s.write(t, "home/.codex/AGENTS.md", "# Mine\n")
		}, steps: []step{
			cli("me", "show"),
			cli("me", "sync"),
			{args: []string{"me", "init"}, stdin: "boss\n"},
			{args: []string{"me", "init"}, stdin: "lead\n\nnew\nid\nclaude-code codex\n"},
			cli("me", "show"),
			cli("me", "sync"),
			cli("me", "sync"),
			cli("me", "init", "--yes", "-p", "."),
			cli("me", "sync", "--remove"),
			cli("me", "show"),
		}},
		{name: "check commits", setup: func(t *testing.T, s side) {
			s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\n")
			s.git(t, "w", "init", "-q", "-b", "main")
		}, steps: []step{
			cli("sync"),
			{args: []string{"check", "--base", "main"}, before: func(t *testing.T, s side) {
				commitOnBranch("README.md", "chore: toolkit")(t, s)
				s.git(t, "w", "checkout", "-q", "-b", "feat/share")
				commitOnBranch("src/a.txt", "feat(share): add links")(t, s)
			}},
			{args: []string{"check", "--base", "main"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "checkout", "-q", "main")
				commitOnBranch("src/base.txt", "base moved")(t, s)
				s.git(t, "w", "checkout", "-q", "feat/share")
				s.git(t, "w", "merge", "-q", "--no-edit", "main")
				commitOnBranch("src/b.txt", "WIP sharing")(t, s)
			}},
			{args: []string{"check"}, env: map[string]string{"GITHUB_EVENT_PATH": "../event.json", "GITHUB_ACTIONS": "true"},
				before: func(t *testing.T, s side) {
					s.write(t, "event.json", `{"pull_request":{"body":"","base":{"sha":"`+s.git(t, "w", "rev-parse", "main")+
						`"},"head":{"sha":"`+s.git(t, "w", "rev-parse", "HEAD")+`"}}}`)
				}},
			cli("check", "--base", "nope"),
			cli("help", "check"),
		}},
		{name: "release lines", setup: releaseLinesRepo, steps: []step{
			cli("help", "release"),
			cli("release", "lines", "--help"),
			cli("release", "lines"),
			{args: []string{"release", "lines"}, before: func(t *testing.T, s side) { s.git(t, "w", "checkout", "-q", "release/2.4.0") }},
			{args: []string{"release", "lines"}, before: func(t *testing.T, s side) { s.git(t, "w", "checkout", "-q", "fix/recorded") }},
			{args: []string{"release", "lines", "-p", "w"}, dir: ".", before: func(t *testing.T, s side) { s.git(t, "w", "checkout", "-q", "fix/guess") }},
			{args: []string{"release", "lines"}, before: func(t *testing.T, s side) { s.git(t, "w", "checkout", "-q", "--detach") }},
			{args: []string{"release", "lines"}, before: func(t *testing.T, s side) {
				s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\n")
			}},
			{args: []string{"release", "lines"}, before: func(t *testing.T, s side) {
				s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\nproject: { release_lines: { main: develop, release: 'release/(.+)' } }\n")
			}},
		}},
		{name: "release status and ports", setup: portsRepo, steps: []step{
			cli("release", "status", "--help"),
			cli("check", "--help"),
			cli("check", "--ports"),
			cli("check", "--ports", "--from", "2.4.0", "--to", "2.5.0"),
			cli("check", "--ports", "--from", "release/2.4.0", "--to", "hotfix/2.4.5"),
			cli("check", "--ports", "--to", "main"),
			cli("check", "--ports", "--from", "main"),
			cli("check", "--ports", "--from", "9.9.9"),
			cli("check", "--ports", "--base", "develop"),
			cli("check", "--from", "2.4.0"),
			cli("release", "status", "2.4.5"),
			cli("release", "status", "2.5.0"),
			cli("release", "status", "9.9.9"),
			cli("release", "status"),
			cli("commit", "lint", "fix: totals round down\n\nPorted-from: 1a2b3c4\nPort: yes"),
			cli("commit", "lint", "fix: totals round down\n\nPorted-from: release/2.4.0\nPort: true"),
			{args: []string{"commit", "suggest", "--subject", "totals round down", "--ported-from", "1a2b3c4"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "checkout", "-q", "-b", "fix/port-totals-2.5.0", "release/2.5.0")
				s.write(t, "w/README.md", "readme, totals round down\n")
			}},
			cli("commit", "suggest", "--ported-from", "HEAD~1"),
		}},
		{name: "sync on a release line", setup: linesRepo, steps: []step{
			cli("sync"),
			{args: []string{"sync"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "add", "-A")
				s.git(t, "w", "commit", "-q", "-m", "chore: toolkit")
				s.git(t, "w", "checkout", "-q", "-b", "release/2.4.0")
				olderLineToolkit(t, s)
			}},
			cli("doctor"),
			{args: []string{"sync"}, before: func(t *testing.T, s side) { editProfile(t, s, "feat: feat/", "feat: feature/") }},
			cli("sync", "--upgrade"),
			cli("sync"),
			{args: []string{"sync"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "checkout", "-q", "-f", "develop")
				s.git(t, "w", "checkout", "-q", "-b", "fix/totals")
				s.git(t, "w", "config", "branch.fix/totals.dashcrew-release", "2.4.0")
				olderLineToolkit(t, s)
			}},
			{args: []string{"sync"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "checkout", "-q", "develop")
				olderLineToolkit(t, s)
			}},
		}},
		{name: "sync after a merge between lines", setup: linesRepo, steps: []step{
			cli("sync"),
			{args: []string{"sync"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "add", "-A")
				s.git(t, "w", "commit", "-q", "-m", "chore: toolkit")
				s.git(t, "w", "branch", "release/2.5.0")
				s.git(t, "w", "checkout", "-q", "-b", "release/2.4.0")
				editProfile(t, s, "feat: feat/", "feat: feature/")
				s.write(t, "w/README.md", "readme 2.4.0\n")
			}},
			{args: []string{"sync"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "add", "-A")
				s.git(t, "w", "commit", "-q", "-m", "chore: feature prefix")
				s.git(t, "w", "checkout", "-q", "release/2.5.0")
				editProfile(t, s, "fix: fix/", "fix: bugfix/")
				s.write(t, "w/README.md", "readme 2.5.0\n")
			}},
			{args: []string{"check"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "add", "-A")
				s.git(t, "w", "commit", "-q", "-m", "chore: bugfix prefix")
				merge := exec.Command("git", "merge", "-q", "--no-edit", "release/2.4.0")
				merge.Dir = s.path("w")
				merge.Env = s.env
				if out, err := merge.CombinedOutput(); err == nil {
					t.Fatalf("the merge did not conflict:\n%s", out)
				}
			}},
			cli("sync"),
			cli("sync", "--after-merge"),
			cli("check"),
			cli("sync"),
		}},
		{name: "worktree watch", setup: repo(repoProfile), steps: []step{
			cli("worktree", "watch", "--once"),
			cli("worktree", "create", "feat/a", "origin/main", "--no-pub"),
			cli("worktree", "create", "feat/b", "main", "--no-pub"),
			cli("worktree", "watch", "--once"),
			cli("worktree", "watch", "--once", "feat-x"),
			cli("worktree", "watch", "--interval", "abc", "--once"),
			{args: []string{"worktree", "watch", "--once", "--interval", " 0x10 "}, before: func(t *testing.T, s side) {
				s.write(t, "w.worktrees/feat-a/lib/b.dart", "\n")
				s.git(t, "w.worktrees/feat-a", "add", "-A")
				s.git(t, "w.worktrees/feat-a", "commit", "-q", "-m", "b")
			}},
			{args: []string{"worktree", "watch", "--once", "feat-a"}, before: func(t *testing.T, s side) {
				s.git(t, "w.worktrees/feat-a", "push", "-q", "origin", "feat/a")
			}},
			{args: []string{"worktree", "watch", "--once"}, before: func(t *testing.T, s side) {
				s.write(t, "w.worktrees/feat-a/lib/a.dart", "void a() { 1; }\n")
				s.write(t, "w.worktrees/feat-b/lib/a.dart", "void a() { 2; }\n")
			}},
			{args: []string{"worktree", "watch", "--once", "--state-dir", "../state"}, before: func(t *testing.T, s side) {
				s.write(t, "w/lib/a.dart", "void a() { /* base */ }\n")
				s.git(t, "w", "commit", "-q", "-am", "base")
				s.git(t, "w", "push", "-q", "origin", "main")
			}},
			{args: []string{"worktree", "watch", "--once"}, before: func(t *testing.T, s side) {
				s.git(t, "w.worktrees/feat-a", "commit", "-q", "-am", "branch")
				s.write(t, "w/lib/c.dart", "\n")
				s.git(t, "w", "add", "-A")
				s.git(t, "w", "commit", "-q", "-m", "base 2")
				s.git(t, "w", "push", "-q", "origin", "main")
			}},
			{args: []string{"worktree", "watch", "--once", "feat-a", "feat-b"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "push", "-q", "origin", "--delete", "develop")
				s.git(t, "w.worktrees/feat-b", "checkout", "-q", "--", ".")
			}},
			{args: []string{"worktree", "watch", "--once"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "worktree", "remove", "--force", s.path("w.worktrees/feat-a"))
				s.git(t, "w", "worktree", "remove", "--force", s.path("w.worktrees/feat-b"))
			}},
		}},
	}
	for _, name := range []string{"sample-bloc-getx", "sample-project-pack", "sample-riverpod-gorouter", "sample-codex",
		"sample-antigravity", "sample-other", "sample-project-platform", "sample-knowledge", "sample-release-lines", "sample-links"} {
		cases = append(cases, cliCase{name: name, setup: sample(name), steps: toolkitSteps()})
	}
	return cases
}

// transcriptPath is the recorded transcript of a case.
func transcriptPath(name string) string {
	slug := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, strings.ToLower(name))
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return filepath.Join("testdata", "transcripts", strings.Trim(slug, "-")+".txt")
}

// describeStep is the transcript header of a step: the arguments, then the folder, stdin and environment it ran with.
func describeStep(i int, st step) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== step %d: dashcrew", i+1)
	for _, arg := range st.args {
		b.WriteString(" " + strconv.Quote(arg))
	}
	b.WriteString("\n")
	if st.dir != "" {
		fmt.Fprintf(&b, "dir: %s\n", st.dir)
	}
	if st.stdin != "" {
		fmt.Fprintf(&b, "stdin: %s\n", strconv.Quote(st.stdin))
	}
	keys := make([]string, 0, len(st.env))
	for key := range st.env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "env: %s=%s\n", key, st.env[key])
	}
	if st.before != nil {
		b.WriteString("(after the case's setup for this step)\n")
	}
	return b.String()
}

// block is a labelled stream, each line prefixed with "| " so empty lines and missing final newlines stay visible.
func block(label, text string) string {
	if text == "" {
		return label + ": (empty)\n"
	}
	var b strings.Builder
	b.WriteString(label + ":\n")
	body := strings.TrimSuffix(text, "\n")
	for _, line := range strings.Split(body, "\n") {
		b.WriteString("| " + line + "\n")
	}
	if !strings.HasSuffix(text, "\n") {
		b.WriteString("(no final newline)\n")
	}
	return b.String()
}

// entry describes one snapshot value briefly: a folder, a link with its target, or a file's mode and content hash.
func entry(value string) string {
	if value == "dir" || strings.HasPrefix(value, "link -> ") {
		return value
	}
	mode, content, _ := strings.Cut(value, " ")
	return fmt.Sprintf("%s sha256:%x", mode, sha256.Sum256([]byte(content)))[:len(mode)+24]
}

// lineChanges lists the lines a step removed from (-) and added to (+) a file, as a multiset difference in file
// order. Files under .git use it instead of a hash, because git's own initial content (the default config, the
// exclude template) differs between git versions and platforms; the lines the step changed do not.
func lineChanges(old, value string) string {
	_, oldContent, _ := strings.Cut(old, " ")
	_, content, _ := strings.Cut(value, " ")
	lines := func(text string) []string {
		if text == "" {
			return nil
		}
		return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}
	count := func(text string) map[string]int {
		out := map[string]int{}
		for _, line := range lines(text) {
			out[line]++
		}
		return out
	}
	var b strings.Builder
	added, removed := count(content), count(oldContent)
	for _, line := range lines(oldContent) {
		if added[line] > 0 {
			added[line]--
			continue
		}
		fmt.Fprintf(&b, "    - %q\n", line)
	}
	for _, line := range lines(content) {
		if removed[line] > 0 {
			removed[line]--
			continue
		}
		fmt.Fprintf(&b, "    + %q\n", line)
	}
	return b.String()
}

// treeChanges lists what a step added (+), changed (~) and removed (-), in path order.
func treeChanges(before, after map[string]string) string {
	paths := map[string]bool{}
	for path := range before {
		paths[path] = true
	}
	for path := range after {
		paths[path] = true
	}
	sorted := make([]string, 0, len(paths))
	for path := range paths {
		sorted = append(sorted, path)
	}
	sort.Strings(sorted)
	var b strings.Builder
	for _, path := range sorted {
		old, inBefore := before[path]
		value, inAfter := after[path]
		switch {
		case !inBefore && inGitDir(path) && value != "dir":
			fmt.Fprintf(&b, "+ %s\n%s", path, lineChanges(" ", value))
		case !inBefore:
			fmt.Fprintf(&b, "+ %s  %s\n", path, entry(value))
		case !inAfter:
			fmt.Fprintf(&b, "- %s\n", path)
		case old != value && inGitDir(path) && value != "dir":
			fmt.Fprintf(&b, "~ %s\n%s", path, lineChanges(old, value))
		case old != value:
			fmt.Fprintf(&b, "~ %s  %s\n", path, entry(value))
		}
	}
	if b.Len() == 0 {
		return "files: (unchanged)\n"
	}
	return "files:\n" + b.String()
}

// inGitDir is true for a path inside a .git folder.
func inGitDir(path string) bool { return strings.Contains(path, ".git/") }

// transcript runs every step of c in a fresh side and returns what the steps printed and changed.
func (e cliEnvironment) transcript(t *testing.T, c cliCase) string {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := e.newSide(t, root)
	if c.setup != nil {
		c.setup(t, s)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", c.name)
	for i, st := range c.steps {
		if st.before != nil {
			st.before(t, s)
		}
		before := snapshot(t, s)
		got := e.run(t, s, st)
		b.WriteString("\n" + describeStep(i, st))
		fmt.Fprintf(&b, "exit: %d\n", got.code)
		b.WriteString(block("stdout", got.stdout))
		b.WriteString(block("stderr", got.stderr))
		b.WriteString(treeChanges(before, snapshot(t, s)))
	}
	return b.String()
}

func TestCLITranscripts(t *testing.T) {
	e := setUpCLI(t)
	update := os.Getenv("DASHCREW_UPDATE_GOLDENS") == "1"
	calls := 0
	seen := map[string]string{}
	for _, c := range cliCases() {
		calls += len(c.steps)
		path := transcriptPath(c.name)
		if other, ok := seen[path]; ok {
			t.Fatalf("cases %q and %q share %s", other, c.name, path)
		}
		seen[path] = c.name
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := e.transcript(t, c)
			if update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v; record it with DASHCREW_UPDATE_GOLDENS=1 go test ./cmd/dashcrew -run Transcript", err)
			}
			if got != string(want) {
				t.Errorf("transcript differs from %s; if the change is intended, rewrite it with "+
					"DASHCREW_UPDATE_GOLDENS=1 go test ./cmd/dashcrew -run Transcript and review the diff\n%s",
					path, firstDifference(string(want), got))
			}
		})
	}
	t.Logf("%d cases, %d calls", len(cliCases()), calls)
}

// firstDifference shows the first differing line of two transcripts with a few lines of context.
func firstDifference(want, got string) string {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			start := max(0, i-6)
			context := strings.Join(wantLines[start:min(i, len(wantLines))], "\n")
			return fmt.Sprintf("line %d, after:\n%s\n--- recorded\n%s\n--- now\n%s", i+1, context, w, g)
		}
	}
	return "(no line differs)"
}
