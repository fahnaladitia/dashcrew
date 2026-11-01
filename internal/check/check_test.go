package check

import (
	"os"
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
