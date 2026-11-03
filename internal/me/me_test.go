package me

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	embedded "github.com/fahnaladitia/dashcrew"
)

func personal(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(embedded.PersonalFS, "personal")
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

func places(t *testing.T) Places {
	t.Helper()
	home := t.TempDir()
	p, err := PlacesFrom(map[string]string{"HOME": home})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseDefaultsAndRoundTrip(t *testing.T) {
	p, err := Parse("role: mobile\nstacks: [flutter, flutter]\n")
	if err != nil {
		t.Fatal(err)
	}
	want := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("got %+v", p)
	}
	full := &Profile{Role: "lead", Stacks: []string{"go", "flutter"}, Level: "new", ChatLanguage: "id", Agents: []string{"codex", "antigravity"}}
	back, err := Parse(full.Text())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, full) {
		t.Fatalf("round trip: %+v", back)
	}
}

func TestParseErrors(t *testing.T) {
	for text, want := range map[string]string{
		"role: mobile\nstacks: [flutter]\nteam: x\n":          "me.yaml team: unknown key",
		"stacks: [flutter]\n":                                 "me.yaml role: expected one of",
		"role: boss\nstacks: [flutter]\n":                     "me.yaml role: expected one of",
		"role: be\n":                                          "me.yaml stacks: required",
		"role: be\nstacks: [Go]\n":                            "me.yaml stacks: Go is not valid",
		"role: be\nstacks: go\n":                              "me.yaml stacks: expected a list",
		"role: be\nstacks: [go]\nlevel: senior\n":             "me.yaml level: expected one of",
		"role: be\nstacks: [go]\nchat_language: indonesian\n": "me.yaml chat_language:",
		"role: be\nstacks: [go]\nagents: [cursor]\n":          "me.yaml agents: cursor is not valid",
		"- a\n": "me.yaml: expected a map",
	} {
		if _, err := Parse(text); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want %q", text, err, want)
		}
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load(t.TempDir()); !os.IsNotExist(err) {
		t.Fatalf("got %v", err)
	}
}

func TestPlacesFromEnv(t *testing.T) {
	p, err := PlacesFrom(map[string]string{"USERPROFILE": "/u", "CLAUDE_CONFIG_DIR": "/c", "CODEX_HOME": "/x"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Home != "/u" || p.ClaudeDir != "/c" || p.CodexDir != "/x" || p.GeminiDir != filepath.Join("/u", ".gemini") {
		t.Fatalf("got %+v", p)
	}
	if _, err := PlacesFrom(map[string]string{}); err == nil {
		t.Fatal("want an error without a home")
	}
}

func TestInstructions(t *testing.T) {
	regular := &Profile{Role: "mobile", Stacks: []string{"flutter"}, Level: "regular", Agents: []string{"claude-code"}}
	text, err := Instructions(regular, personal(t), "claude-code", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"trigger:", "## Chat language", "## New to the team", "## Lead", "{{"} {
		if strings.Contains(text, absent) {
			t.Errorf("regular profile has %q:\n%s", absent, text)
		}
	}
	if !strings.Contains(text, "Member: role `mobile`, stacks `flutter`.") || !strings.Contains(text, marker) {
		t.Errorf("member line or marker missing:\n%s", text)
	}

	lead := &Profile{Role: "lead", Stacks: []string{"go", "flutter"}, Level: "new", ChatLanguage: "id", Agents: Agents}
	text, err = Instructions(lead, personal(t), "antigravity", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(text, "---\ntrigger: always_on\n") {
		t.Errorf("antigravity front matter missing:\n%s", text)
	}
	for _, want := range []string{"stacks `go`, `flutter`, new to the team.", "language `id`", "read `.ai/knowledge/README.md`", "dashcrew check --ports"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
	text, err = Instructions(lead, personal(t), "claude-code", map[string]bool{"dashcrew-onboarding": true, "dashcrew-lead": true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "follow the `dashcrew-onboarding` skill") || !strings.Contains(text, "follow the `dashcrew-lead` skill") || strings.Contains(text, "dashcrew check --ports") {
		t.Errorf("skills not referenced:\n%s", text)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSyncWritesEachAgentAndRemoves(t *testing.T) {
	pl := places(t)
	codex := filepath.Join(pl.CodexDir, "AGENTS.md")
	if err := os.MkdirAll(pl.CodexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codex, []byte("# Mine\n\nKeep this.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Profile{Role: "lead", Stacks: []string{"go"}, Level: "new", Agents: Agents}
	changes, err := Sync(p, pl, personal(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 5 {
		t.Fatalf("changes: %v", changes)
	}
	claudeRule := filepath.Join(pl.ClaudeDir, "rules", ruleFile)
	if !strings.Contains(read(t, claudeRule), "`dashcrew-lead` skill") {
		t.Error("claude rule does not name the linked skill")
	}
	for _, name := range Skills {
		if !strings.Contains(read(t, filepath.Join(pl.ClaudeDir, "skills", name, "SKILL.md")), "name: "+name) {
			t.Errorf("skill %s not reachable", name)
		}
	}
	file := read(t, codex)
	if !strings.HasPrefix(file, "# Mine\n\nKeep this.\n\n"+codexBegin+"\n") || !strings.HasSuffix(file, codexEnd+"\n") {
		t.Errorf("codex file:\n%s", file)
	}
	if !strings.HasPrefix(read(t, filepath.Join(pl.GeminiDir, "config", "rules", ruleFile)), "---\ntrigger: always_on") {
		t.Error("antigravity rule lacks front matter")
	}
	if stale, err := Status(p, pl, personal(t)); err != nil || len(stale) != 0 {
		t.Fatalf("status after sync: %v %v", stale, err)
	}

	again, err := Sync(p, pl, personal(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range again {
		if c.What != "unchanged" {
			t.Errorf("second sync: %v", c)
		}
	}
	if read(t, codex) != file {
		t.Error("second sync changed the codex file")
	}

	regular := &Profile{Role: "be", Stacks: []string{"go"}, Level: "regular", Agents: []string{"codex"}}
	if stale, _ := Status(regular, pl, personal(t)); len(stale) != 5 {
		t.Errorf("status for a narrower profile: %v", stale)
	}
	if _, err := Sync(regular, pl, personal(t)); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{claudeRule, filepath.Join(pl.ClaudeDir, "skills", "dashcrew-lead")} {
		if _, err := os.Lstat(gone); !os.IsNotExist(err) {
			t.Errorf("%s left over", gone)
		}
	}
	if !strings.Contains(read(t, codex), "role `be`") {
		t.Error("codex block not updated")
	}

	if _, err := Remove(pl); err != nil {
		t.Fatal(err)
	}
	if got := read(t, codex); got != "# Mine\n\nKeep this.\n" {
		t.Errorf("codex after remove: %q", got)
	}
}

func TestSyncKeepsForeignFiles(t *testing.T) {
	pl := places(t)
	claudeRule := filepath.Join(pl.ClaudeDir, "rules", ruleFile)
	foreignSkill := filepath.Join(pl.ClaudeDir, "skills", "dashcrew-onboarding")
	for path, text := range map[string]string{
		claudeRule:                              "my own rule\n",
		filepath.Join(foreignSkill, "SKILL.md"): "mine\n",
		filepath.Join(pl.CodexDir, "AGENTS.md"): "x\n" + codexBegin + "\nno end\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := &Profile{Role: "fe", Stacks: []string{"react"}, Level: "new", Agents: []string{"claude-code", "codex"}}
	changes, err := Sync(p, pl, personal(t))
	if err != nil {
		t.Fatal(err)
	}
	kept := 0
	for _, c := range changes {
		if c.Kept {
			kept++
		}
	}
	if kept != 3 {
		t.Fatalf("changes: %v", changes)
	}
	if read(t, claudeRule) != "my own rule\n" || read(t, filepath.Join(foreignSkill, "SKILL.md")) != "mine\n" {
		t.Error("a foreign file was overwritten")
	}
	if _, err := Remove(pl); err != nil {
		t.Fatal(err)
	}
	if read(t, claudeRule) != "my own rule\n" {
		t.Error("remove took a foreign file")
	}
}

func TestRepoPlatformAndPropose(t *testing.T) {
	dir := t.TempDir()
	if RepoPlatform(dir) != "" {
		t.Error("no project.yaml should give no platform")
	}
	if err := os.MkdirAll(filepath.Join(dir, ".ai"), 0o755); err != nil {
		t.Fatal(err)
	}
	for text, want := range map[string]string{"toolkit: 1\n": "flutter", "toolkit: 1\nplatform: go\n": "go", "platform: [x]\n": ""} {
		if err := os.WriteFile(filepath.Join(dir, ".ai", "project.yaml"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := RepoPlatform(dir); got != want {
			t.Errorf("%q: got %q", text, got)
		}
	}

	pl := places(t)
	if err := os.MkdirAll(pl.CodexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fresh := Propose(nil, "go", pl)
	if !reflect.DeepEqual(fresh, &Profile{Role: "be", Stacks: []string{"go"}, Level: "regular", Agents: []string{"codex"}}) {
		t.Errorf("fresh: %+v", fresh)
	}
	if got := Propose(nil, "", places(t)); got.Role != "fullstack" || got.Stacks != nil || !reflect.DeepEqual(got.Agents, []string{"claude-code"}) {
		t.Errorf("no platform: %+v", got)
	}
	existing := &Profile{Role: "lead", Stacks: []string{"go"}, Level: "new", Agents: []string{"claude-code"}}
	merged := Propose(existing, "flutter", pl)
	if !reflect.DeepEqual(merged.Stacks, []string{"go", "flutter"}) || merged.Role != "lead" || len(existing.Stacks) != 1 {
		t.Errorf("merged: %+v, existing %+v", merged, existing)
	}
}

func TestSyncCopyFallbackIsStable(t *testing.T) {
	pl := places(t)
	p := &Profile{Role: "lead", Stacks: []string{"go"}, Level: "regular", Agents: []string{"claude-code"}}
	link := filepath.Join(pl.ClaudeDir, "skills", "dashcrew-lead")
	data, err := fs.ReadFile(personal(t), "skills/dashcrew-lead/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := copySkill(data, link); err != nil {
		t.Fatal(err)
	}
	changes, err := Sync(p, pl, personal(t))
	if err != nil {
		t.Fatal(err)
	}
	if changes[0].What != "unchanged" || changes[0].Path != link {
		t.Fatalf("changes: %v", changes)
	}
	if !strings.Contains(read(t, filepath.Join(pl.ClaudeDir, "rules", ruleFile)), "`dashcrew-lead` skill") {
		t.Error("a copied skill is not referenced")
	}
	if _, err := Remove(pl); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(link); !os.IsNotExist(err) {
		t.Error("remove left the copy")
	}
}
