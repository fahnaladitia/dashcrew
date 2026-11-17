package guard

import (
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
)

// pushPolicy is testPolicy with agent commit and agent push on, develop and main protected.
func pushPolicy() Policy {
	p := testPolicy(true, false)
	delete(p.BlockedGit, "push")
	p.Push = &PushRule{Protected: []string{"develop", "main"}}
	return p
}

func pushDecision(t *testing.T, command string, p Policy) Decision {
	t.Helper()
	decision, err := EvaluateAll(allFromHook(t, bashHook(t, command)), p)
	if err != nil {
		t.Fatal(err)
	}
	return decision
}

// wrapped runs command bare and behind the wrappers and nesting the guard already unwraps.
func wrapped(command string) []string {
	return []string{
		command,
		"sh -c '" + command + "'",
		"env FOO=1 " + command,
		"cd app && " + command,
		strings.Replace(command, "git ", "git -C app ", 1),
	}
}

func TestAgentPushAllowsATaskBranchNamedOutright(t *testing.T) {
	for _, command := range []string{
		"git push -u origin feat/login",
		"git push --set-upstream origin feat/login",
		"git push origin feat/login",
		"git push origin HEAD:feat/login",
		"git push origin HEAD:refs/heads/feat/login",
		"git push --dry-run -q origin feat/login",
		"git push -o ci.skip origin feat/login",
	} {
		for _, form := range wrapped(command) {
			if decision := pushDecision(t, form, pushPolicy()); decision.Blocked {
				t.Errorf("%q blocked: %s", form, decision.Reason)
			}
		}
	}
}

func TestAgentPushStillBlocksForceDeleteEverythingAndProtectedBranches(t *testing.T) {
	for command, reason := range map[string]string{
		"git push --force origin feat/x":                   "git push --force",
		"git push -f origin feat/x":                        "git push --force",
		"git push -uf origin feat/x":                       "git push --force",
		"git push --force-with-lease origin feat/x":        "git push --force",
		"git push --force-with-lease=feat/x origin feat/x": "git push --force",
		"git push --force-if-includes origin feat/x":       "git push --force",
		"git push origin +feat/x":                          "git push --force",
		"git push origin --delete feat/x":                  "deleting a remote branch",
		"git push -d origin feat/x":                        "deleting a remote branch",
		"git push origin :feat/x":                          "deleting a remote branch",
		"git push --all origin":                            "git push --all",
		"git push --mirror origin":                         "git push --mirror",
		"git push --tags origin":                           "git push --tags",
		"git push origin refs/tags/v1.0.0":                 "only a branch",
		"git push origin 'feat/*'":                         "only a branch",
		"git push origin main":                             "pushing to main",
		"git push origin refs/heads/main":                  "pushing to main",
		"git push origin HEAD:main":                        "pushing to main",
		"git push origin feat/x:develop":                   "pushing to develop",
		"git push origin feat/x main":                      "pushing to main",
		"git push":                                         "name the task branch",
		"git push origin":                                  "name the task branch",
		"git push origin HEAD":                             "name the task branch",
		"git push origin @":                                "name the task branch",
	} {
		for _, form := range wrapped(command) {
			decision := pushDecision(t, form, pushPolicy())
			if !decision.Blocked || !strings.Contains(decision.Reason, reason) {
				t.Errorf("%q: blocked %v, reason %q, want it to contain %q", form, decision.Blocked, decision.Reason, reason)
			}
		}
	}
}

func TestWithoutAgentPushEveryPushStaysBlocked(t *testing.T) {
	if decision := pushDecision(t, "git push -u origin feat/login", testPolicy(true, false)); !decision.Blocked {
		t.Error("push allowed without the push rule")
	}
}

func TestCompiledAgentPushPolicyDropsPushAndProtectsTheBaseBranches(t *testing.T) {
	policy := compile(t, "toolkit: 1\nagents: [claude-code]\nproject: { base_branches: [develop, main] }\npolicy: { agent_commit: true, agent_push: true }\n")

	if policy.BlockedGit["push"] || policy.BlockedGit["commit"] {
		t.Errorf("blocked %v", policy.BlockedGit)
	}
	if policy.Push == nil || strings.Join(policy.Push.Protected, ",") != "develop,main" {
		t.Fatalf("push %+v", policy.Push)
	}
	if policy.Minimum() != "0.7.0" {
		t.Errorf("minimum %s", policy.Minimum())
	}
	text := encodePolicy(t, policy)
	if !strings.Contains(text, `"push":{"protected":["develop","main"]}`) {
		t.Errorf("guard.json %s", text)
	}
	for name, read := range map[string]func(string) (Policy, error){"PolicyFromJSON": PolicyFromJSON, "LoadPolicy": loadPolicyText} {
		loaded, err := read(text)
		if err != nil || loaded.Push == nil || strings.Join(loaded.Push.Protected, ",") != "develop,main" || loaded.BlockedGit["push"] {
			t.Errorf("%s: %+v %v", name, loaded.Push, err)
		}
	}
}

func TestWithoutAgentPushThePolicyHasNoPushKey(t *testing.T) {
	policy := compile(t, "toolkit: 1\nagents: [claude-code]\npolicy: { agent_commit: true }\n")

	if !policy.BlockedGit["push"] || policy.Push != nil || strings.Contains(encodePolicy(t, policy), `"push":`) {
		t.Errorf("blocked %v, push %+v", policy.BlockedGit, policy.Push)
	}
}

func TestAMalformedPushKeyIsAnError(t *testing.T) {
	base := `{"schema":1,"blocked_git":["commit"],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{},"forbidden_imports":[],"local_packages":[],"push":`
	for _, push := range []string{`true`, `{"protected":"main"}`, `{"protected":[1]}`} {
		for name, read := range map[string]func(string) (Policy, error){"PolicyFromJSON": PolicyFromJSON, "LoadPolicy": loadPolicyText} {
			if _, err := read(base + push + "}"); err == nil {
				t.Errorf("%s accepted push %s", name, push)
			}
		}
	}
}

func encodePolicy(t *testing.T, p Policy) string {
	t.Helper()
	text, err := dartjson.Encode(p.ToJSON())
	if err != nil {
		t.Fatal(err)
	}
	return text
}

// loadPolicyText is the hook's reader (LoadPolicy) on a guard.json text.
func loadPolicyText(text string) (Policy, error) {
	value, err := dartjson.Decode(text)
	if err != nil {
		return Policy{}, err
	}
	return LoadPolicy(value)
}
