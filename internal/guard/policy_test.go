package guard

import (
	"reflect"
	stdsync "sync"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

var (
	shippedOnce    stdsync.Once
	shippedCatalog *catalog.Catalog
)

func compile(t *testing.T, yaml string) Policy {
	t.Helper()
	shippedOnce.Do(func() {
		var err error
		if shippedCatalog, err = catalog.LoadEmbedded(); err != nil {
			t.Fatal(err)
		}
	})
	p, err := profile.Parse(yaml, shippedCatalog.KnownOptions())
	if err != nil {
		t.Fatal(err)
	}
	return CompilePolicy(p, shippedCatalog.ResolvedFor(p.Platform))
}

func TestDefaultsBlockCommitAndFormattersWithPlansUnderAi(t *testing.T) {
	policy := compile(t, "toolkit: 1\nagents: [claude-code]\n")

	for _, sub := range []string{"commit", "push", "stash"} {
		if !policy.BlockedGit[sub] {
			t.Errorf("%s not blocked", sub)
		}
	}
	if !policy.BlockFormatters || policy.PlansDir != ".ai/plans" || len(policy.ForbiddenImports) != 0 {
		t.Errorf("policy %+v", policy)
	}
}

func TestRoleCapabilitiesComeFromRoleYaml(t *testing.T) {
	policy := compile(t, "toolkit: 1\nagents: [claude-code]\n")

	if want := map[string]string{"explorer": "read", "reviewer": "read"}; !reflect.DeepEqual(policy.RoleCapability, want) {
		t.Errorf("roles %v", policy.RoleCapability)
	}
}

func TestAgentCommitAndFormatterRelaxOnlyTheirOwnRules(t *testing.T) {
	policy := compile(t, "toolkit: 1\nagents: [claude-code]\npolicy: { agent_commit: true, formatter: true }\n")

	if policy.BlockedGit["commit"] || !policy.BlockedGit["push"] || policy.BlockFormatters {
		t.Errorf("policy %+v", policy)
	}
}

func TestAnalyticsVendorImportAndLocalPackagesFlowIntoThePolicy(t *testing.T) {
	policy := compile(t, "toolkit: 1\nagents: [claude-code]\nstack: { analytics: posthog }\nproject: { local_packages: [core] }\npaths: { plans: docs/plans }\n")

	if !reflect.DeepEqual(policy.ForbiddenImports, []string{"package:posthog_flutter"}) ||
		!reflect.DeepEqual(policy.LocalPackages, []string{"core"}) || policy.PlansDir != "docs/plans" {
		t.Errorf("policy %+v", policy)
	}
}

func TestPolicyJSONIsSortedAndReadsBack(t *testing.T) {
	policy := compile(t, "toolkit: 1\nagents: [claude-code]\n")

	text, err := dartjson.EncodeIndent(policy.ToJSON(), "  ")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "schema": 1,
  "blocked_git": [
    "cherry-pick",
    "commit",
    "merge",
    "push",
    "rebase",
    "reset",
    "revert",
    "stash",
    "tag"
  ],
  "block_formatters": true,
  "plans_dir": ".ai/plans",
  "role_capability": {
    "explorer": "read",
    "reviewer": "read"
  },
  "forbidden_imports": [],
  "local_packages": []
}`
	if text != want {
		t.Errorf("got %s", text)
	}
	back, err := PolicyFromJSON(text)
	if err != nil || !reflect.DeepEqual(back, policy) {
		t.Errorf("read back %+v, %v", back, err)
	}
}

func TestPolicyFromJSONRejectsMissingOrMistypedKeys(t *testing.T) {
	for _, text := range []string{
		`[]`,
		`{"blocked_git": [], "block_formatters": true, "plans_dir": ".ai/plans", "role_capability": {}, "forbidden_imports": []}`,
		`{"blocked_git": [1], "block_formatters": true, "plans_dir": ".ai/plans", "role_capability": {}, "forbidden_imports": [], "local_packages": []}`,
		`{"blocked_git": [], "block_formatters": "yes", "plans_dir": ".ai/plans", "role_capability": {}, "forbidden_imports": [], "local_packages": []}`,
		`not json`,
	} {
		if _, err := PolicyFromJSON(text); err == nil {
			t.Errorf("accepted %s", text)
		}
	}
}
