package me

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
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
	writeFile(t, filepath.Join(repo, ".ai", "skills", "broken", "SKILL.md"), "# no meta.yaml next to it\n")

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

func TestAnOptionalSkillCarriesItsFilesAndTakesOnlyThoseAway(t *testing.T) {
	pl := places(t)
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	repo := gitRepo(t)
	source := filepath.Join(repo, ".ai", "skills", "ui-slop")
	writeFile(t, filepath.Join(source, "meta.yaml"), "description: d\noptional: true\n")
	writeFile(t, filepath.Join(source, "SKILL.md"), "# UI slop\n")
	writeFile(t, filepath.Join(source, "references", "checklist.md"), "- one\n")
	writeFile(t, filepath.Join(source, "scripts", "run.sh"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(source, "scripts", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&RepoProfile{Skills: []string{"ui-slop"}}).Save(repo); err != nil {
		t.Fatal(err)
	}
	if err := Register(pl, repo); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}

	folder := filepath.Join(repo, ".claude", "skills", "ui-slop")
	if read(t, filepath.Join(folder, "references", "checklist.md")) != "- one\n" {
		t.Error("references/checklist.md not written")
	}
	if info, err := os.Stat(filepath.Join(folder, "scripts", "run.sh")); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0) {
		t.Errorf("scripts/run.sh not written executable: %v", err)
	}

	writeFile(t, filepath.Join(source, "references", "checklist.md"), "- two\n")
	if changes, err := Status(home, pl, personal(t)); err != nil || !containsChange(changes, Change{filepath.Join(folder, "references", "checklist.md"), "stale", false}) {
		t.Errorf("status misses the changed file: %v %v", changes, err)
	}
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(folder, "references", "checklist.md")) != "- two\n" {
		t.Error("a changed supporting file is not updated")
	}

	writeFile(t, filepath.Join(folder, "notes.md"), "mine\n")
	if err := (&RepoProfile{}).Save(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"SKILL.md", "references", "scripts", ".dashcrew-me"} {
		if exists(filepath.Join(folder, gone)) {
			t.Errorf("%s stays after the skill was turned off", gone)
		}
	}
	if !exists(filepath.Join(folder, "notes.md")) {
		t.Error("me sync removed a file it did not write")
	}
}

func TestAManifestNeverReachesOutsideItsSkillFolder(t *testing.T) {
	pl := places(t)
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	repo := gitRepo(t)
	victim := filepath.Join(repo, "victim.txt")
	writeFile(t, victim, "keep me\n")
	folder := filepath.Join(repo, ".claude", "skills", "evil")
	writeFile(t, filepath.Join(folder, "SKILL.md"), skillMarker+"\n")
	writeFile(t, filepath.Join(folder, skillManifest), "../../../victim.txt\n"+victim+"\n")
	if err := (&RepoProfile{}).Save(repo); err != nil {
		t.Fatal(err)
	}
	if err := Register(pl, repo); err != nil {
		t.Fatal(err)
	}

	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}

	if !exists(victim) {
		t.Fatal("a manifest path outside the skill folder was removed")
	}
}

func TestStatusAndSyncFollowRemovedFilesAndExecuteBits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no execute bits on Windows")
	}
	pl := places(t)
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	repo := gitRepo(t)
	source := filepath.Join(repo, ".ai", "skills", "ui-slop")
	writeFile(t, filepath.Join(source, "meta.yaml"), "description: d\noptional: true\n")
	writeFile(t, filepath.Join(source, "SKILL.md"), "# UI slop\n")
	for _, name := range []string{"b.md", "a.md", "run.sh"} {
		writeFile(t, filepath.Join(source, name), name+"\n")
	}
	if err := os.Chmod(filepath.Join(source, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&RepoProfile{Skills: []string{"ui-slop"}}).Save(repo); err != nil {
		t.Fatal(err)
	}
	if err := Register(pl, repo); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(home, pl, personal(t)); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(repo, ".claude", "skills", "ui-slop")

	for _, name := range []string{"a.md", "b.md"} {
		if err := os.Remove(filepath.Join(source, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(source, "run.sh"), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, err := Status(home, pl, personal(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []Change{
		{filepath.Join(folder, "a.md"), "left over", false},
		{filepath.Join(folder, "run.sh"), "stale", false},
	} {
		if !containsChange(changes, want) {
			t.Errorf("status misses %v: %v", want, changes)
		}
	}

	changes, err = Sync(home, pl, personal(t))
	if err != nil {
		t.Fatal(err)
	}
	var removed []string
	for _, change := range changes {
		if change.What == "removed" {
			removed = append(removed, filepath.Base(change.Path))
		}
	}
	if !reflect.DeepEqual(removed, []string{"a.md", "b.md"}) {
		t.Errorf("removed %v, want a.md then b.md", removed)
	}
	if info, err := os.Stat(filepath.Join(folder, "run.sh")); err != nil || info.Mode().Perm()&0o111 != 0 {
		t.Errorf("run.sh keeps its execute bits after the source lost them: %v", err)
	}
}

func TestABrokenProjectPackNoLongerStopsOptionalSkills(t *testing.T) {
	pl := places(t)
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	repo := gitRepo(t)
	writeFile(t, filepath.Join(repo, ".ai", "packs", "broken", "x", "rules.md"), "---\ntarget: agents_md\n---\nx\n")
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

	if !exists(filepath.Join(repo, ".claude", "skills", "ui-slop", "SKILL.md")) {
		t.Error("a broken project pack, which me sync does not need, kept the optional skill out")
	}
}

func TestAnOptionalSkillNamedLikeAProjectPackSkillIsNotWritten(t *testing.T) {
	pl := places(t)
	home := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	repo := gitRepo(t)
	writeFile(t, filepath.Join(repo, ".ai", "packs", "ui", "mine", "skills", "ui-slop.yaml"), "description: d\n")
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

	if exists(filepath.Join(repo, ".claude", "skills", "ui-slop")) {
		t.Error("me sync wrote a skill whose name a project pack skill takes, which sync refuses")
	}
}
