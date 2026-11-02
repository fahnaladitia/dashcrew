package check

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	stdsync "sync"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

var (
	shippedOnce    stdsync.Once
	shippedCatalog *catalog.Catalog
)

const fullBody = "## Summary\nx\n## Changes\nx\n## Affected files\nx\n## Testing\nx\n## QA Identifiers\nNone\n## Analytics\nNone\n## Notes / not done\nNone\n"

func setup(t *testing.T, policy string) string {
	t.Helper()
	shippedOnce.Do(func() {
		var err error
		if shippedCatalog, err = catalog.LoadEmbedded(); err != nil {
			t.Fatal(err)
		}
	})
	project := t.TempDir()
	syncWith(t, project, policy)
	return project
}

func syncWith(t *testing.T, project, policy string) {
	t.Helper()
	write(t, filepath.Join(project, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\nstack: { qa: semantics-identifier, analytics: posthog }\n"+policy)
	if _, err := (sync.Sync{Catalog: shippedCatalog, ProjectDir: project}).Run(true, false); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func check(t *testing.T, project string, body *string, environment map[string]string) map[string]doctor.Check {
	t.Helper()
	checks, err := PrCheck{Catalog: shippedCatalog, ProjectDir: project, Environment: environment}.Run(body)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]doctor.Check{}
	for _, c := range checks {
		out[c.ID] = c
	}
	return out
}

func text(s string) *string { return &s }

func TestToolkitChecksRunWithoutTheMachineAndAgentChecks(t *testing.T) {
	project := setup(t, "")

	checks := check(t, project, text(fullBody), nil)

	if checks["drift"].Status != doctor.OK || checks["stale"].Status != doctor.OK {
		t.Errorf("drift %v, stale %v", checks["drift"], checks["stale"])
	}
	for _, id := range []string{"guard-binary", "hook"} {
		if _, ok := checks[id]; ok {
			t.Errorf("%s ran", id)
		}
	}
}

func TestEveryTemplateHeadingMustBeInThePrBody(t *testing.T) {
	project := setup(t, "")

	if got := check(t, project, text(fullBody), nil)["pr-sections"]; got.Status != doctor.OK || got.Detail != "7 required sections present" {
		t.Errorf("full body %v", got)
	}

	missing := check(t, project, text(strings.Replace(fullBody, "## Analytics\nNone\n", "", 1)), nil)["pr-sections"]
	if missing.Status != doctor.Fail || missing.Detail != `missing: ## Analytics (write "None" under a heading that does not apply)` {
		t.Errorf("missing %v", missing)
	}
}

func TestNoPrBodySkipsTheSectionCheckAndAMissingTemplateFailsIt(t *testing.T) {
	project := setup(t, "")

	if got := check(t, project, nil, nil)["pr-sections"]; got.Status != doctor.Skip {
		t.Errorf("no body %v", got)
	}

	if err := os.Remove(filepath.Join(project, filepath.FromSlash(sync.PrTemplatePath))); err != nil {
		t.Fatal(err)
	}
	if got := check(t, project, text(fullBody), nil)["pr-sections"]; got.Status != doctor.Fail {
		t.Errorf("no template %v", got)
	}
}

func TestAPrBodyWithNoPlanLinePassesEvenWhenTheProfileStillSetsPlanGate(t *testing.T) {
	project := setup(t, "")
	syncWith(t, project, "policy: { plan_gate: true }\n")

	checks := check(t, project, text(fullBody), nil)

	if _, ok := checks["plan-approval"]; ok {
		t.Error("plan-approval ran")
	}
	if checks["pr-sections"].Status != doctor.OK {
		t.Errorf("pr-sections %v", checks["pr-sections"])
	}
	for _, c := range checks {
		if c.Status == doctor.Fail {
			t.Errorf("%s failed: %s", c.ID, c.Detail)
		}
	}
}

func TestInAGithubActionThePrBodyComesFromTheEventFile(t *testing.T) {
	project := setup(t, "")
	event := filepath.Join(project, "event.json")
	write(t, event, `{"pull_request":{"body":`+quote(fullBody)+`}}`)

	if got := check(t, project, nil, map[string]string{"GITHUB_EVENT_PATH": event})["pr-sections"]; got.Status != doctor.OK {
		t.Errorf("event body %v", got)
	}

	write(t, event, `{"pull_request":{"body":null}}`)
	if got := check(t, project, nil, map[string]string{"GITHUB_EVENT_PATH": event})["pr-sections"]; got.Status != doctor.Fail {
		t.Errorf("empty body %v", got)
	}

	write(t, event, `{"push":{}}`)
	if got := check(t, project, nil, map[string]string{"GITHUB_EVENT_PATH": event})["pr-sections"]; got.Status != doctor.Skip {
		t.Errorf("not a pull request %v", got)
	}
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, "\n", `\n`) + `"`
}

func TestLinesSplitLikeDart(t *testing.T) {
	cases := map[string]string{
		"a\nb\n":   "a|b",
		"a\r\nb":   "a|b",
		"a\rb\n\n": "a|b|",
		"":         "",
	}
	for in, want := range cases {
		if got := strings.Join(lines(in), "|"); got != want {
			t.Errorf("lines(%q) = %q", in, got)
		}
	}
}

// gitProject is a synced project in a git repo: main holds the toolkit, branch feat/x adds commits with the given
// messages, then a merge from main.
func gitProject(t *testing.T, messages ...string) (string, func(args ...string) string) {
	t.Helper()
	project := setup(t, "")
	for _, kv := range [][2]string{
		{"GIT_CONFIG_NOSYSTEM", "1"}, {"GIT_CONFIG_GLOBAL", filepath.Join(project, ".no-global")},
		{"GIT_AUTHOR_NAME", "t"}, {"GIT_AUTHOR_EMAIL", "t@example.com"},
		{"GIT_COMMITTER_NAME", "t"}, {"GIT_COMMITTER_EMAIL", "t@example.com"},
	} {
		t.Setenv(kv[0], kv[1])
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = project
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-q", "-m", "chore: toolkit")
	git("checkout", "-q", "-b", "feat/x")
	for i, message := range messages {
		write(t, filepath.Join(project, "src", "f.txt"), strings.Repeat("x", i+1))
		git("add", "-A")
		git("commit", "-q", "-m", message)
	}
	git("checkout", "-q", "main")
	write(t, filepath.Join(project, "src", "g.txt"), "g")
	git("add", "-A")
	git("commit", "-q", "-m", "base moved without a type")
	git("checkout", "-q", "feat/x")
	git("merge", "-q", "--no-edit", "main")
	return project, git
}

func TestCommitsCheckLintsThePrCommitsAndSkipsMerges(t *testing.T) {
	project, git := gitProject(t, "feat: add sharing", "fix(api): keep the token")
	base := "main"

	got := checkWithBase(t, project, &base, nil)["commits"]
	if got.Status != doctor.OK || got.Detail != "2 commits in main..HEAD follow <type>(<scope>): <subject>" {
		t.Errorf("good %v", got)
	}

	git("commit", "-q", "--allow-empty", "-m", "wip\n\nCo-authored-by: Claude <noreply@anthropic.com>")
	short := git("rev-parse", "--short=7", "HEAD")
	bad := checkWithBase(t, project, &base, nil)["commits"]
	want := short + ` subject line "wip" does not match <type>(<scope>): <subject>; ` + short + " email address on line 3: commit messages never show one; " +
		short + " AI attribution: AI co-author trailer on line 3"
	if bad.Status != doctor.Fail || bad.Detail != want {
		t.Errorf("bad %v\nwant %s", bad, want)
	}
}

func TestCommitsCheckReadsBaseAndHeadFromThePullRequestEvent(t *testing.T) {
	project, git := gitProject(t, "Feat: capitalized type")
	baseSha, headSha := git("rev-parse", "main"), git("rev-parse", "HEAD")
	event := filepath.Join(t.TempDir(), "event.json")
	write(t, event, `{"pull_request":{"body":"","base":{"sha":"`+baseSha+`"},"head":{"sha":"`+headSha+`"}}}`)

	got := checkWithBase(t, project, nil, map[string]string{"GITHUB_EVENT_PATH": event})["commits"]
	if got.Status != doctor.Fail || !strings.Contains(got.Detail, `type "Feat" is not one of`) {
		t.Errorf("event %v", got)
	}

	write(t, event, `{"pull_request":{"body":"","base":{"sha":"`+strings.Repeat("0", 40)+`"},"head":{"sha":"`+headSha+`"}}}`)
	if got := checkWithBase(t, project, nil, map[string]string{"GITHUB_EVENT_PATH": event})["commits"]; got.Status != doctor.Fail ||
		!strings.HasPrefix(got.Detail, "cannot read 0000000.."+headSha[:7]+": fatal: ") || !strings.HasSuffix(got.Detail, "; a CI checkout needs fetch-depth: 0") {
		t.Errorf("missing base %v", got)
	}
}

func TestCommitsCheckSkipsWithoutABase(t *testing.T) {
	project := setup(t, "")
	if got := check(t, project, text(fullBody), nil)["commits"]; got.Status != doctor.Skip ||
		got.Detail != "no base; pass --base <branch> or run on a pull_request event" {
		t.Errorf("no base %v", got)
	}
	base := "main"
	if got := checkWithBase(t, project, &base, nil)["commits"]; got.Status != doctor.Fail || !strings.Contains(got.Detail, "is not inside a git repository") {
		t.Errorf("not a repo %v", got)
	}
}

func checkWithBase(t *testing.T, project string, base *string, environment map[string]string) map[string]doctor.Check {
	t.Helper()
	checks, err := PrCheck{Catalog: shippedCatalog, ProjectDir: project, Environment: environment, Base: base}.Run(text(fullBody))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]doctor.Check{}
	for _, c := range checks {
		out[c.ID] = c
	}
	return out
}
