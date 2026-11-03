package me

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestSkillsParseInBothLayers(t *testing.T) {
	p, err := Parse("role: mobile\nstacks: [flutter]\nskills: [ui-slop, ui-slop, checks]\n")
	if err != nil || !reflect.DeepEqual(p.Skills, []string{"ui-slop", "checks"}) {
		t.Fatalf("home %+v %v", p, err)
	}
	if back, err := Parse(p.Text()); err != nil || !reflect.DeepEqual(back, p) {
		t.Fatalf("home round trip %+v %v", back, err)
	}
	r, err := ParseRepo("skills: [ui-slop]\n")
	if err != nil || !reflect.DeepEqual(r, &RepoProfile{Skills: []string{"ui-slop"}}) {
		t.Fatalf("repo %+v %v", r, err)
	}
	if back, err := ParseRepo((&RepoProfile{Role: "lead", Skills: []string{"ui-slop"}}).Text()); err != nil || !reflect.DeepEqual(back, &RepoProfile{Role: "lead", Skills: []string{"ui-slop"}}) {
		t.Fatalf("repo round trip %+v %v", back, err)
	}
	for text, want := range map[string]string{
		"role: mobile\nstacks: [flutter]\nskills: [UI]\n":            "me.yaml skills: UI is not valid",
		"role: mobile\nstacks: [flutter]\nskills: [dashcrew-lead]\n": "me.yaml skills: dashcrew-lead is not valid",
		"role: mobile\nstacks: [flutter]\nskills: ui-slop\n":         "me.yaml skills: expected a list",
	} {
		if _, err := Parse(text); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want %q", text, err, want)
		}
	}
	if _, err := ParseRepo("skills: [Bad]\n"); err == nil || !strings.Contains(err.Error(), ".ai/me.yaml skills: Bad is not valid") {
		t.Errorf("repo bad name: %v", err)
	}
}

func TestSyncLinksTheMembersOwnSkillsForClaudeCode(t *testing.T) {
	pl := places(t)
	writeFile(t, filepath.Join(pl.Home, ".dashcrew", "skills", "ui-slop", "SKILL.md"), "---\nname: ui-slop\ndescription: Catch generic UI.\n---\n# UI slop\n")
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code", "codex"}, Skills: []string{"ui-slop", "gone"}}

	changes, err := Sync(home, pl, personal(t))
	if err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(pl.ClaudeDir, "skills", "ui-slop")
	if !exists(filepath.Join(link, "SKILL.md")) {
		t.Fatal("ui-slop is not linked for Claude Code")
	}
	if rules := read(t, homeInstructionPath(pl, "claude-code")); !strings.Contains(rules, "- `ui-slop`: Catch generic UI.\n") {
		t.Errorf("Claude rules lack the own skill:\n%s", rules)
	}
	if rules := read(t, homeInstructionPath(pl, "codex")); strings.Contains(rules, "ui-slop") {
		t.Error("Codex rules name a skill Codex does not have")
	}
	missing := filepath.Join(pl.Home, ".dashcrew", "skills", "gone", "SKILL.md")
	if !containsChange(changes, Change{missing, "missing; skill gone not linked", true}) {
		t.Errorf("no change for the missing skill: %v", changes)
	}

	home.Skills = nil
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}
	if exists(link) {
		t.Error("ui-slop is still linked after it left the profile")
	}
	if !exists(filepath.Join(pl.Home, ".dashcrew", "skills", "ui-slop", "SKILL.md")) {
		t.Error("me sync deleted the member's own skill")
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	return dir
}

func TestSyncWritesTurnedOnOptionalProjectSkillsIntoThatCloneOnly(t *testing.T) {
	pl := places(t)
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code", "codex"}}
	repo := gitRepo(t)
	writeFile(t, filepath.Join(repo, ".ai", "skills", "ui-slop", "meta.yaml"), "description: Catch generic UI.\nwhen_to_use: When a screen is built.\noptional: true\n")
	writeFile(t, filepath.Join(repo, ".ai", "skills", "ui-slop", "SKILL.md"), "# UI slop\n")
	writeFile(t, filepath.Join(repo, ".ai", "skills", "api-style", "meta.yaml"), "description: Team API style.\n")
	writeFile(t, filepath.Join(repo, ".ai", "skills", "api-style", "SKILL.md"), "# API style\n")
	writeFile(t, filepath.Join(repo, ".claude", "skills", "team-made", "SKILL.md"), "# a skill the team wrote by hand\n")
	if err := (&RepoProfile{Skills: []string{"ui-slop", "api-style", "nope"}}).Save(repo); err != nil {
		t.Fatal(err)
	}
	if err := Register(pl, repo); err != nil {
		t.Fatal(err)
	}

	changes, err := Sync(home, pl, personal(t))
	if err != nil {
		t.Fatal(err)
	}

	claudeSkill := filepath.Join(repo, ".claude", "skills", "ui-slop", "SKILL.md")
	agentsSkill := filepath.Join(repo, ".agents", "skills", "ui-slop", "SKILL.md")
	for _, path := range []string{claudeSkill, agentsSkill} {
		if text := read(t, path); !strings.Contains(text, marker) || !strings.HasSuffix(text, "# UI slop\n") || !strings.Contains(text, "Catch generic UI.") {
			t.Errorf("%s:\n%s", path, text)
		}
	}
	if exists(filepath.Join(repo, ".claude", "skills", "api-style")) {
		t.Error("me sync wrote the required api-style, which sync owns")
	}
	for _, want := range []Change{
		{repo, "skill api-style: required in this repo, on for everyone", false},
		{repo, "skill nope: no optional skill of that name in .ai/skills", true},
	} {
		if !containsChange(changes, want) {
			t.Errorf("missing change %v in %v", want, changes)
		}
	}
	exclude := read(t, filepath.Join(repo, ".git", "info", "exclude"))
	for _, entry := range []string{"/.claude/skills/ui-slop/", "/.agents/skills/ui-slop/"} {
		if !strings.Contains(exclude, entry+"\n") {
			t.Errorf("exclude lacks %s:\n%s", entry, exclude)
		}
	}
	if rules := read(t, repoInstructionPath(repo, "claude-code")); !strings.Contains(rules, "- `ui-slop`: When a screen is built.\n") {
		t.Errorf("repo rules lack ui-slop:\n%s", rules)
	}

	if err := (&RepoProfile{}).Save(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Dir(claudeSkill), filepath.Dir(agentsSkill)} {
		if exists(path) {
			t.Errorf("%s stays after the member turned ui-slop off", path)
		}
	}
	if !exists(filepath.Join(repo, ".claude", "skills", "team-made", "SKILL.md")) {
		t.Error("me sync removed a skill it did not write")
	}
}

func TestSyncLeavesARepoAloneWhenItsAiFolderDoesNotLoad(t *testing.T) {
	pl := places(t)
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	repo := gitRepo(t)
	writeFile(t, filepath.Join(repo, ".ai", "skills", "ui-slop", "meta.yaml"), "description: d\noptional: true\n")
	writeFile(t, filepath.Join(repo, ".ai", "skills", "ui-slop", "SKILL.md"), "# UI slop\n")
	if err := (&RepoProfile{Skills: []string{"ui-slop"}}).Save(repo); err != nil {
		t.Fatal(err)
	}
	if err := Register(pl, repo); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(repo, ".claude", "skills", "ui-slop", "SKILL.md")
	writeFile(t, filepath.Join(repo, ".ai", "packs", "broken", "x", "rules.md"), "---\ntarget: agents_md\n---\nx\n")

	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}

	if !exists(skill) {
		t.Error("a broken .ai/ folder made me sync delete the member's turned-on skill")
	}
}

func TestStatusMatchesSyncWhenAnOwnSkillFolderIsForeign(t *testing.T) {
	pl := places(t)
	writeFile(t, filepath.Join(pl.Home, ".dashcrew", "skills", "foo", "SKILL.md"), "---\ndescription: Foo.\n---\n# Foo\n")
	writeFile(t, filepath.Join(pl.ClaudeDir, "skills", "foo", "SKILL.md"), "# made by hand\n")
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}, Skills: []string{"foo"}}

	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}
	changes, err := Status(home, pl, personal(t))
	if err != nil {
		t.Fatal(err)
	}

	for _, change := range changes {
		if change.What == "stale" {
			t.Errorf("status right after sync: %v", change)
		}
	}
}

func TestTurningASkillOffDropsItsExcludeLines(t *testing.T) {
	pl := places(t)
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	repo := gitRepo(t)
	writeFile(t, filepath.Join(repo, ".ai", "skills", "ui-slop", "meta.yaml"), "description: d\noptional: true\n")
	writeFile(t, filepath.Join(repo, ".ai", "skills", "ui-slop", "SKILL.md"), "# UI slop\n")
	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "# mine\n/build/\n")
	if err := (&RepoProfile{Skills: []string{"ui-slop"}}).Save(repo); err != nil {
		t.Fatal(err)
	}
	if err := Register(pl, repo); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(repo, ".ai", "skills", "ui-slop", "meta.yaml"), "description: d\n")
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}

	if exclude := read(t, filepath.Join(repo, ".git", "info", "exclude")); strings.Contains(exclude, "ui-slop") || !strings.Contains(exclude, "# mine\n/build/\n") {
		t.Errorf("exclude after the skill became required:\n%s", exclude)
	}
}

func TestSyncUnlinksOnlyOwnSkillsItLinked(t *testing.T) {
	pl := places(t)
	source := filepath.Join(pl.Home, ".dashcrew", "skills", "manual")
	writeFile(t, filepath.Join(source, "SKILL.md"), "---\ndescription: M.\n---\n# M\n")
	link := filepath.Join(pl.ClaudeDir, "skills", "manual")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, link); err != nil {
		t.Skip("no symlinks here")
	}
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}

	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}

	if !exists(link) {
		t.Error("me sync removed a link the member made by hand")
	}
}
