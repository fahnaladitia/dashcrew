package doctor

import (
	"os"
	"path/filepath"
	"strings"
	stdsync "sync"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

var (
	shippedOnce    stdsync.Once
	shippedCatalog *catalog.Catalog
)

type fixture struct {
	t          *testing.T
	project    string
	userSkills string
	guardHome  string
	codexHome  string
}

func setup(t *testing.T, profile string) *fixture {
	t.Helper()
	shippedOnce.Do(func() {
		var err error
		if shippedCatalog, err = catalog.LoadEmbedded(); err != nil {
			t.Fatal(err)
		}
	})
	f := &fixture{t: t, project: t.TempDir(), userSkills: t.TempDir(), guardHome: t.TempDir(), codexHome: t.TempDir()}
	f.write(".ai/project.yaml", profile)
	return f
}

const blocProfile = "toolkit: 1\nagents: [claude-code]\nstack:\n  state: bloc\n  di: get_it\n"

func (f *fixture) write(relative, content string) {
	f.t.Helper()
	writeFile(f.t, filepath.Join(f.project, filepath.FromSlash(relative)), content)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) read(relative string) string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.project, filepath.FromSlash(relative)))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f *fixture) delete(relative string) {
	f.t.Helper()
	if err := os.Remove(filepath.Join(f.project, filepath.FromSlash(relative))); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) sync() {
	f.t.Helper()
	if _, err := (sync.Sync{Catalog: shippedCatalog, ProjectDir: f.project}).Run(false, false); err != nil {
		f.t.Fatal(err)
	}
}

func sourceHash(t *testing.T) string {
	t.Helper()
	hash, err := guard.SourceHash()
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

// installGuard puts a stand-in for the compiled guard where the shim looks: it blocks every call with exit 2.
func (f *fixture) installGuard() {
	f.t.Helper()
	path := guard.BinaryPath(f.guardHome, sourceHash(f.t))
	writeFile(f.t, path, "#!/bin/sh\ncat >/dev/null\nexit 2\n")
	if err := os.Chmod(path, 0o755); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) doctor() map[string]Check {
	f.t.Helper()
	return byID(Doctor{
		Catalog:       shippedCatalog,
		ProjectDir:    f.project,
		GuardSource:   sourceHash(f.t),
		UserSkillsDir: f.userSkills,
		GuardHome:     f.guardHome,
		CodexHome:     f.codexHome,
		MachineChecks: true,
	}.Run())
}

func byID(checks []Check) map[string]Check {
	out := map[string]Check{}
	for _, check := range checks {
		out[check.ID] = check
	}
	return out
}

func expect(t *testing.T, checks map[string]Check, id string, status Status, parts ...string) {
	t.Helper()
	check, ok := checks[id]
	if !ok {
		t.Errorf("no %s check", id)
		return
	}
	if check.Status != status {
		t.Errorf("%s: %s, want %s (%s)", id, check.Status, status, check.Detail)
	}
	for _, part := range parts {
		if !strings.Contains(check.Detail, part) {
			t.Errorf("%s detail %q misses %q", id, check.Detail, part)
		}
	}
}

func TestStatusNames(t *testing.T) {
	for status, name := range map[Status]string{OK: "ok", Warn: "warn", Fail: "fail", Skip: "skip", Info: "info"} {
		if status.String() != name {
			t.Errorf("%d is %s", status, status.String())
		}
	}
}

func TestAFreshSyncWithAnInstalledGuardPassesEveryCheck(t *testing.T) {
	f := setup(t, blocProfile)
	f.sync()
	f.installGuard()

	checks := f.doctor()

	for _, check := range checks {
		if check.ID == "skills" {
			if check.Status == Fail {
				t.Errorf("skills: %s", check.Detail)
			}
		} else if check.Status != OK {
			t.Errorf("%s: %s %s", check.ID, check.Status, check.Detail)
		}
	}
	for _, id := range []string{"profile", "lock", "drift", "stale", "guard-policy", "guard-binary", "hook"} {
		if _, ok := checks[id]; !ok {
			t.Errorf("no %s check", id)
		}
	}
}

func TestGuardBinary(t *testing.T) {
	t.Run("not installed fails", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()

		expect(t, f.doctor(), "guard-binary", Fail, "not installed on this machine", "no guard rule applies here")
	})

	t.Run("a guard that does not block fails", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		path := guard.BinaryPath(f.guardHome, sourceHash(t))
		writeFile(t, path, "#!/bin/sh\ncat >/dev/null\nexit 0\n")
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}

		expect(t, f.doctor(), "guard-binary", Fail, "did not block git push (exit 0)")
	})

	t.Run("a project that expects another guard warns", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		f.installGuard()

		checks := byID(Doctor{Catalog: shippedCatalog, ProjectDir: f.project, GuardSource: strings.Repeat("0", 64),
			UserSkillsDir: f.userSkills, GuardHome: f.guardHome, MachineChecks: true}.Run())

		expect(t, checks, "guard-binary", Warn, "different guard")
	})

	t.Run("a missing shim fails", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		f.delete(guard.ShimPath)

		expect(t, f.doctor(), "guard-binary", Fail, ".ai/bin/guard missing")
	})
}

func TestDrift(t *testing.T) {
	t.Run("a generated file edited by hand fails", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		f.write(".claude/skills/plan/SKILL.md", "edited by hand\n")

		expect(t, f.doctor(), "drift", Fail, "edited by hand: .claude/skills/plan/SKILL.md")
	})

	t.Run("a generated file deleted fails", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		f.delete(".claude/agents/reviewer.md")

		expect(t, f.doctor(), "drift", Fail, "missing: .claude/agents/reviewer.md")
	})

	t.Run("editing the AGENTS.md project block is not drift", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		f.write("AGENTS.md", strings.Replace(f.read("AGENTS.md"), sync.ProjectBegin, sync.ProjectBegin+"\nTeam rule.", 1))

		checks := f.doctor()

		expect(t, checks, "drift", OK)
		expect(t, checks, "stale", OK)
	})
}

func TestAProfileChangedAfterSyncWarnsStaleWithoutDrift(t *testing.T) {
	f := setup(t, blocProfile)
	f.sync()
	f.write(".ai/project.yaml", "toolkit: 1\nagents: [claude-code]\n")

	checks := f.doctor()

	expect(t, checks, "stale", Warn, "dashcrew sync")
	expect(t, checks, "drift", OK)
}

func TestMissingGuardJSONFailsGuardPolicy(t *testing.T) {
	f := setup(t, blocProfile)
	f.sync()
	f.delete(".ai/guard.json")

	expect(t, f.doctor(), "guard-policy", Fail)

	f.write(".ai/guard.json", `{"blocked_git": "push"}`)
	expect(t, f.doctor(), "guard-policy", Fail, ".ai/guard.json invalid: ")
}

func TestGuardHookRemovedFromSettingsFailsHook(t *testing.T) {
	f := setup(t, blocProfile)
	f.sync()
	f.write(".claude/settings.json", `{"hooks":{}}`)

	expect(t, f.doctor(), "hook", Fail, "missing")

	f.write(".claude/settings.json", `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":".ai/bin/guard"}]}]}}`)
	expect(t, f.doctor(), "hook", Fail, "differs from what sync writes")
}

func TestClaudeMdWithoutTheAgentsMdImportFails(t *testing.T) {
	f := setup(t, blocProfile)
	f.write("CLAUDE.md", "Personal notes.\n")
	f.sync()

	expect(t, f.doctor(), "claude-md", Fail)
}

func TestSkills(t *testing.T) {
	t.Run("a missing dashcrew skill fails", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		f.delete(".claude/skills/prompt-polish/SKILL.md")

		expect(t, f.doctor(), "skills", Fail, "prompt-polish")
	})

	t.Run("a skill named only in the AGENTS.md skill index is checked too", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		if !strings.Contains(f.read("AGENTS.md"), "- `work-modes`: ") {
			t.Fatal("work-modes not in the skill index")
		}
		f.delete(".claude/skills/work-modes/SKILL.md")

		expect(t, f.doctor(), "skills", Fail, "work-modes")
	})

	t.Run("an unknown external skill warns, one in the user skills dir does not", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		f.write("AGENTS.md", strings.Replace(f.read("AGENTS.md"), sync.ProjectBegin,
			sync.ProjectBegin+"\nUse the `made-up-skill` skill and skill `user-installed`.", 1))
		writeFile(t, filepath.Join(f.userSkills, "user-installed", "SKILL.md"), "---\nname: user-installed\n---\n")

		checks := f.doctor()

		expect(t, checks, "skills", Warn, "made-up-skill")
		if strings.Contains(checks["skills"].Detail, "user-installed") {
			t.Errorf("detail %s", checks["skills"].Detail)
		}
	})
}

func TestSkillIndexNames(t *testing.T) {
	agentsMd := "# Rules\n\n### Skill index\n\n- `plan`: plans.\n- `code-review` (review): reviews.\n- not `a-skill`: x\n\n## Next\n- `outside`: no\n"
	if got := strings.Join(skillIndexNames(agentsMd), ","); got != "plan,code-review" {
		t.Errorf("got %s", got)
	}
	if skillIndexNames("no index") != nil {
		t.Error("names without an index")
	}
}

func TestAiBinInGitignoreWarns(t *testing.T) {
	f := setup(t, blocProfile)
	f.sync()
	f.write(".gitignore", "build/\n.ai/bin/\n")

	expect(t, f.doctor(), "gitignore", Warn)
}

func TestNeverSyncedFailsTheLockAndSkipsWhatNeedsIt(t *testing.T) {
	f := setup(t, blocProfile)

	checks := f.doctor()

	expect(t, checks, "profile", OK)
	expect(t, checks, "lock", Fail, ".ai/lock.json missing")
	expect(t, checks, "drift", Skip)
	expect(t, checks, "stale", Skip)
	expect(t, checks, "version", Skip)
}

func TestALockThatIsNotJSONOrHasAnotherSchemaFails(t *testing.T) {
	f := setup(t, blocProfile)
	f.sync()

	f.write(sync.LockPath, "{")
	expect(t, f.doctor(), "lock", Fail, "is not JSON")

	f.write(sync.LockPath, `{"schema": 2}`)
	expect(t, f.doctor(), "lock", Fail, "unknown schema")

	f.write(sync.LockPath, `{"schema": 1}`)
	checks := f.doctor()
	expect(t, checks, "lock", OK)
	expect(t, checks, "drift", Skip)
}

func TestAProfileThatStillSetsPlanGateWarnsAndKeepsTheStaleCheck(t *testing.T) {
	f := setup(t, "toolkit: 1\nagents: [claude-code]\npolicy: { plan_gate: true }\n")
	f.sync()

	checks := f.doctor()

	expect(t, checks, "profile", Warn, "plan_gate was removed in P4; delete it from .ai/project.yaml")
	expect(t, checks, "stale", OK)
}

func TestHandoffRosters(t *testing.T) {
	t.Run("no check without a roster file left from before P4", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		f.write(".ai/plans/orders/plan.md", "# Plan\n")

		if _, ok := f.doctor()["handoff-rosters"]; ok {
			t.Error("handoff-rosters reported")
		}
	})

	t.Run("rosters under the plans path are reported as information", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [claude-code]\npaths: { plans: docs/plans }\n")
		f.sync()
		f.write("docs/plans/orders/handoff/00-roster.md", "# Roster\n")
		f.write("docs/plans/profile-page/handoff/00-roster.md", "# Roster\n")
		f.write("docs/plans/profile-page/handoff/10-product-manager.md", "# Handoff\n")
		f.write(".ai/plans/ignored/handoff/00-roster.md", "# Roster\n")

		checks := f.doctor()

		expect(t, checks, "handoff-rosters", Info, "docs/plans/orders/handoff/00-roster.md, docs/plans/profile-page/handoff/00-roster.md")
		detail := checks["handoff-rosters"].Detail
		if strings.Contains(detail, ".ai/plans/ignored") || strings.Contains(detail, "10-product-manager") {
			t.Errorf("detail %s", detail)
		}
	})
}

func TestMissingProfileFailsProfileAndSkipsStale(t *testing.T) {
	f := setup(t, blocProfile)
	f.sync()
	f.delete(".ai/project.yaml")

	checks := f.doctor()

	expect(t, checks, "profile", Fail, ".ai/project.yaml missing; run dashcrew init")
	expect(t, checks, "stale", Skip)
}

func TestAnInvalidProfileFailsWithTheProfileError(t *testing.T) {
	f := setup(t, "toolkit: 1\nagents: [claude-code]\nstak: {}\n")

	expect(t, f.doctor(), "profile", Fail, ".ai/project.yaml invalid: project.yaml stak: unknown key")
}

func TestVersion(t *testing.T) {
	setLockVersion := func(f *fixture, value string) {
		text := f.read(sync.LockPath)
		start := strings.Index(text, `"dashcrew_version"`)
		end := start + strings.Index(text[start:], "\n")
		f.write(sync.LockPath, text[:start]+value+text[end:])
	}

	t.Run("ok right after sync", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()

		expect(t, f.doctor(), "version", OK, "lock written by this dashcrew (0.0.1)")
	})

	t.Run("warns for a newer, older or missing lock version", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		for _, c := range []struct{ field, hint string }{
			{`"dashcrew_version": "999.0.0",`, "update dashcrew to 999.0.0"},
			{`"dashcrew_version": "0.0.0",`, "run dashcrew sync to move it to 0.0.1"},
			{`"no_version": "",`, "run dashcrew sync to record 0.0.1"},
		} {
			setLockVersion(f, c.field)

			expect(t, f.doctor(), "version", Warn, c.hint)
		}
	})
}

func TestProjectPacks(t *testing.T) {
	t.Run("no project packs is ok", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()

		expect(t, f.doctor(), "project-packs", OK, "no project packs in .ai/packs")
	})

	t.Run("a pack that does not parse fails", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		f.write(".ai/packs/state/bloc/rules.md", "---\ntarget: agents_md\n---\nx\n")

		checks := f.doctor()

		expect(t, checks, "project-packs", Fail, ".ai/packs/state/bloc/pack.yaml")
		expect(t, checks, "stale", Fail, "Project pack: .ai/packs/state/bloc/pack.yaml")
	})

	t.Run("a pack the profile does not select warns", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.write(".ai/packs/architecture/clean/pack.yaml", "description: clean\n")
		f.sync()

		expect(t, f.doctor(), "project-packs", Warn, "not selected in .ai/project.yaml: architecture/clean")
	})

	t.Run("a selected project pack is ok and editing it turns stale", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.write(".ai/packs/state/bloc/pack.yaml", "description: team bloc\n")
		f.write(".ai/packs/state/bloc/rules.md", "---\ntarget: agents_md\n---\nTeam Bloc.\n")
		f.sync()

		checks := f.doctor()
		expect(t, checks, "project-packs", OK, "1 project packs: state/bloc")
		expect(t, checks, "stale", OK)

		f.write(".ai/packs/state/bloc/rules.md", "---\ntarget: agents_md\n---\nTeam Bloc, edited.\n")
		checks = f.doctor()

		expect(t, checks, "stale", Warn)
		expect(t, checks, "drift", OK)
	})
}

func TestCodex(t *testing.T) {
	codexDoctor := func(f *fixture) map[string]Check {
		return byID(Doctor{Catalog: shippedCatalog, ProjectDir: f.project, GuardSource: sourceHash(f.t), UserSkillsDir: f.userSkills,
			CodexHome: f.codexHome, GuardHome: f.guardHome, MachineChecks: true}.Run())
	}
	mergeCodex := func(f *fixture) {
		if _, err := adapters.NewCodex(shippedCatalog, f.project, "flutter").Merge(map[string]bool{}, &[]string{}); err != nil {
			f.t.Fatal(err)
		}
	}
	const codexProfile = "toolkit: 1\nagents: [codex]\n"

	t.Run("only the checks of the selected agents run", func(t *testing.T) {
		f := setup(t, codexProfile)
		codexOnly := codexDoctor(f)
		for _, id := range []string{"codex-hook", "codex-mcp", "codex-skills-budget", "codex-trust"} {
			if _, ok := codexOnly[id]; !ok {
				t.Errorf("no %s", id)
			}
		}
		if _, ok := codexOnly["claude-md"]; ok {
			t.Error("claude-md for codex")
		}

		f.write(".ai/project.yaml", "toolkit: 1\nagents: [claude-code]\n")
		claudeOnly := codexDoctor(f)
		if _, ok := claudeOnly["claude-md"]; !ok {
			t.Error("no claude-md")
		}
		if _, ok := claudeOnly["codex-hook"]; ok {
			t.Error("codex-hook for claude code")
		}
	})

	t.Run("hook and mcp fail or warn until merged, then pass", func(t *testing.T) {
		f := setup(t, codexProfile)
		before := codexDoctor(f)
		expect(t, before, "codex-hook", Fail)
		expect(t, before, "codex-mcp", Warn)

		mergeCodex(f)
		after := codexDoctor(f)
		expect(t, after, "codex-hook", OK)
		expect(t, after, "codex-mcp", OK, "defined: dart-mcp-server")
	})

	t.Run("an mcp server the user defines is fine", func(t *testing.T) {
		f := setup(t, codexProfile)
		f.write(".codex/config.toml", "[mcp_servers.dart-mcp-server]\ncommand = \"fvm\"\n")

		expect(t, codexDoctor(f), "codex-mcp", OK, "user")
	})

	t.Run("a skill list over the budget warns", func(t *testing.T) {
		f := setup(t, codexProfile)
		f.write(".agents/skills/small/SKILL.md", "---\nname: small\ndescription: \"short\"\n---\n")
		expect(t, codexDoctor(f), "codex-skills-budget", OK, "1 skills, list 10 of 8000 characters")

		f.write(".agents/skills/huge/SKILL.md", "---\nname: huge\ndescription: \""+strings.Repeat("x", 8000)+"\"\n---\n")
		expect(t, codexDoctor(f), "codex-skills-budget", Warn, "over 8000")
	})

	t.Run("trust is read from the codex config of the user", func(t *testing.T) {
		f := setup(t, codexProfile)
		expect(t, codexDoctor(f), "codex-trust", Warn)

		writeFile(t, filepath.Join(f.codexHome, "config.toml"), "model = \"m\"\n\n[projects.\""+f.project+"\"]\ntrust_level = \"trusted\"\n")
		expect(t, codexDoctor(f), "codex-trust", OK)
	})

	t.Run("trust covers a project inside a trusted folder, not one next to it", func(t *testing.T) {
		f := setup(t, codexProfile)
		parent := filepath.Dir(f.project)
		writeFile(t, filepath.Join(f.codexHome, "config.toml"), "[projects.'"+parent+"']\ntrust_level = 'trusted'\n\n[projects.\""+f.project+"-other\"]\ntrust_level = \"trusted\"\n")
		expect(t, codexDoctor(f), "codex-trust", OK)

		writeFile(t, filepath.Join(f.codexHome, "config.toml"), "[projects.\""+f.project+"-other\"]\ntrust_level = \"trusted\"\n\n[projects.\""+f.project+"\"]\ntrust_level = \"untrusted\"\n")
		expect(t, codexDoctor(f), "codex-trust", Warn)
	})
}

func TestAntigravity(t *testing.T) {
	const antigravityProfile = "toolkit: 1\nagents: [antigravity]\n"

	t.Run("a fresh sync passes the antigravity checks", func(t *testing.T) {
		f := setup(t, antigravityProfile)
		f.sync()

		checks := f.doctor()

		for _, id := range []string{"antigravity-hook", "antigravity-mcp", "antigravity-roles"} {
			expect(t, checks, id, OK)
		}
		if _, ok := checks["claude-md"]; ok {
			t.Error("claude-md for antigravity")
		}
		expect(t, checks, "agents-md-size", OK, "24000")
	})

	t.Run("a removed guard entry fails and an unknown role tool fails", func(t *testing.T) {
		f := setup(t, antigravityProfile)
		f.sync()
		f.write(".agents/hooks.json", "{}")
		f.write(".agents/agents/reviewer.md", strings.Replace(f.read(".agents/agents/reviewer.md"), "view_file", "view_files", 1))

		checks := f.doctor()

		expect(t, checks, "antigravity-hook", Fail, "guard hook missing")
		expect(t, checks, "antigravity-roles", Fail, "reviewer: view_files")
	})

	t.Run("AGENTS.md over 24000 bytes warns when antigravity is selected", func(t *testing.T) {
		f := setup(t, antigravityProfile)
		f.sync()
		f.write("AGENTS.md", strings.Replace(f.read("AGENTS.md"), sync.ProjectBegin, sync.ProjectBegin+"\n"+strings.Repeat("x", 9000), 1))

		expect(t, f.doctor(), "agents-md-size", Warn, "over 24000")
	})
}

func TestWithoutMachineChecksOnlyTheProjectChecksRun(t *testing.T) {
	f := setup(t, "toolkit: 1\nagents: [claude-code, codex, antigravity]\n")
	f.sync()

	checks := Doctor{Catalog: shippedCatalog, ProjectDir: f.project}.Run()

	var ids []string
	for _, check := range checks {
		ids = append(ids, check.ID)
	}
	if got := strings.Join(ids, " "); got != "profile project-packs lock version drift stale guard-policy agents-md-size gitignore" {
		t.Errorf("checks %s", got)
	}
}

func TestDefaultHomes(t *testing.T) {
	if got := DefaultCodexHome(map[string]string{"CODEX_HOME": "/c", "HOME": "/h"}); got != "/c" {
		t.Errorf("codex %s", got)
	}
	if got := DefaultCodexHome(map[string]string{"HOME": "/h"}); got != filepath.Join("/h", ".codex") {
		t.Errorf("codex %s", got)
	}
	if got := DefaultUserSkillsDir(map[string]string{"CLAUDE_CONFIG_DIR": "/cfg", "HOME": "/h"}); got != filepath.Join("/cfg", "skills") {
		t.Errorf("skills %s", got)
	}
	if got := DefaultUserSkillsDir(map[string]string{"USERPROFILE": "/u"}); got != filepath.Join("/u", ".claude", "skills") {
		t.Errorf("skills %s", got)
	}
	if DefaultCodexHome(map[string]string{}) != "" || DefaultUserSkillsDir(map[string]string{}) != "" {
		t.Error("a default without a home")
	}
}
