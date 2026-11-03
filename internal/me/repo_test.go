package me

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseRepo(t *testing.T) {
	r, err := ParseRepo("role: lead\nlevel: new\n")
	if err != nil || !reflect.DeepEqual(r, &RepoProfile{Role: "lead", Level: "new"}) {
		t.Fatalf("got %+v %v", r, err)
	}
	if r, err := ParseRepo("# only comments\n"); err != nil || !reflect.DeepEqual(r, &RepoProfile{}) {
		t.Fatalf("empty: %+v %v", r, err)
	}
	back, err := ParseRepo((&RepoProfile{Level: "new"}).Text())
	if err != nil || !reflect.DeepEqual(back, &RepoProfile{Level: "new"}) {
		t.Fatalf("round trip: %+v %v", back, err)
	}
	for text, want := range map[string]string{
		"stacks: [go]\n":      ".ai/me.yaml stacks: belongs in ~/.dashcrew/me.yaml",
		"chat_language: id\n": ".ai/me.yaml chat_language: belongs in ~/.dashcrew/me.yaml",
		"team: x\n":           ".ai/me.yaml team: unknown key",
		"role: boss\n":        ".ai/me.yaml role: expected one of",
		"level: senior\n":     ".ai/me.yaml level: expected one of",
		"- a\n":               ".ai/me.yaml: expected a map",
	} {
		if _, err := ParseRepo(text); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want %q", text, err, want)
		}
	}
}

func TestLoadAndSaveRepo(t *testing.T) {
	dir := t.TempDir()
	if r, err := LoadRepo(dir); r != nil || err != nil {
		t.Fatalf("missing: %+v %v", r, err)
	}
	if err := (&RepoProfile{Role: "lead"}).Save(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".ai", "me.yaml")); err != nil {
		t.Fatal(err)
	}
	if r, err := LoadRepo(dir); err != nil || !reflect.DeepEqual(r, &RepoProfile{Role: "lead"}) {
		t.Fatalf("loaded %+v %v", r, err)
	}
}

func TestEffectiveIn(t *testing.T) {
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	if got := EffectiveIn(home, nil).Describe(); got != "role mobile (home), level regular (home)" {
		t.Errorf("no repo: %s", got)
	}
	e := EffectiveIn(home, &RepoProfile{Role: "lead"})
	if e.Describe() != "role lead (repo), level regular (home)" || home.Role != "mobile" || !reflect.DeepEqual(e.Stacks, home.Stacks) {
		t.Errorf("repo role: %s, home %+v", e.Describe(), home)
	}
	if got := EffectiveIn(home, &RepoProfile{Level: "new"}).Describe(); got != "role mobile (home), level new (repo)" {
		t.Errorf("repo level: %s", got)
	}
}

func TestSyncPerRepo(t *testing.T) {
	pl := places(t)
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: Agents}
	lead, fresh := t.TempDir(), t.TempDir()
	if err := (&RepoProfile{Role: "lead"}).Save(lead); err != nil {
		t.Fatal(err)
	}
	if err := (&RepoProfile{Level: "new"}).Save(fresh); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{lead, fresh, lead} {
		if err := Register(pl, dir); err != nil {
			t.Fatal(err)
		}
	}
	if repos, _ := Repos(pl); !reflect.DeepEqual(repos, []string{lead, fresh}) {
		t.Fatalf("registry: %v", repos)
	}
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}
	homeRule := read(t, filepath.Join(pl.ClaudeDir, "rules", ruleFile))
	if !strings.Contains(homeRule, "A repository can set its own role and level") || strings.Contains(homeRule, "## Lead") {
		t.Errorf("home rule:\n%s", homeRule)
	}
	leadRule := read(t, filepath.Join(lead, ".claude", "rules", ruleFile))
	for _, want := range []string{marker, "# Personal profile in this repository (dashcrew)", "sets role `lead` and level `regular`", "## Lead", "`dashcrew-lead` skill"} {
		if !strings.Contains(leadRule, want) {
			t.Errorf("lead rule lacks %q:\n%s", want, leadRule)
		}
	}
	if strings.Contains(leadRule, "## New to the team") || strings.Contains(leadRule, "Outside the member's stacks") {
		t.Errorf("lead rule repeats home sections:\n%s", leadRule)
	}
	if !strings.HasPrefix(read(t, filepath.Join(fresh, ".agents", "rules", ruleFile)), "---\ntrigger: always_on") {
		t.Error("antigravity repo rule lacks front matter")
	}
	if !strings.Contains(read(t, filepath.Join(fresh, ".claude", "rules", ruleFile)), "`dashcrew-onboarding` skill") {
		t.Error("new repo rule does not name onboarding")
	}
	codex := read(t, filepath.Join(pl.CodexDir, "AGENTS.md"))
	for _, want := range []string{"## In the repository at `" + filepath.ToSlash(lead) + "`", "### Lead", "## In the repository at `" + filepath.ToSlash(fresh) + "`", "### New to the team"} {
		if !strings.Contains(codex, want) {
			t.Errorf("codex block lacks %q:\n%s", want, codex)
		}
	}
	for _, name := range Skills {
		if ours, _ := linkState(pl, name); !ours {
			t.Errorf("%s not linked", name)
		}
	}
	if stale, err := Status(home, pl, personal(t)); err != nil || len(stale) != 0 {
		t.Fatalf("status: %v %v", stale, err)
	}

	if _, err := RemoveRepo(home, pl, personal(t), lead); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(lead, ".claude", "rules", ruleFile)); !os.IsNotExist(err) {
		t.Error("lead repo rule left")
	}
	if ours, _ := linkState(pl, "dashcrew-lead"); ours {
		t.Error("lead skill still linked")
	}
	if strings.Contains(read(t, filepath.Join(pl.CodexDir, "AGENTS.md")), filepath.ToSlash(lead)) {
		t.Error("codex section for the removed repo left")
	}

	if err := os.Remove(filepath.Join(fresh, ".ai", "me.yaml")); err != nil {
		t.Fatal(err)
	}
	freshRule := filepath.Join(fresh, ".claude", "rules", ruleFile)
	stale, _ := Status(home, pl, personal(t))
	if !containsChange(stale, Change{freshRule, "left over", false}) || !containsChange(stale, Change{fresh, "no .ai/me.yaml there", false}) {
		t.Errorf("status with a gone repo file: %v", stale)
	}
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(freshRule); !os.IsNotExist(err) {
		t.Error("the rule of a repo without .ai/me.yaml was left")
	}
	if gone, err := Prune(pl); err != nil || !reflect.DeepEqual(gone, []string{fresh}) {
		t.Fatalf("prune: %v %v", gone, err)
	}
	if _, err := Remove(pl); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(registryPath(pl)); !os.IsNotExist(err) {
		t.Error("registry left after remove")
	}
}

func TestRepoRuleTurnsHomeSectionsOff(t *testing.T) {
	home := &Profile{Role: "lead", Stacks: []string{"go"}, Level: "new", Agents: []string{"claude-code"}}
	text, err := renderRepo(home, &RepoProfile{Role: "be", Level: "regular"}, "/r", personal(t), "claude-code", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"The Lead section of the home profile does not apply here.", "The New to the team section of the home profile does not apply here."} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
	if os.Getenv("DUMP") != "" {
		t.Log("\n" + text)
	}
}

func containsChange(changes []Change, want Change) bool {
	for _, c := range changes {
		if c == want {
			return true
		}
	}
	return false
}
