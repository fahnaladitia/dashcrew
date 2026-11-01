package sync

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	stdsync "sync"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/version"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

var (
	shippedOnce    stdsync.Once
	shippedCatalog *catalog.Catalog
)

const defaultProfile = `toolkit: 1
agents: [claude-code]
stack:
  state: bloc
  di: get_it
  analytics: posthog
project: { local_packages: [core] }
`

func newProject(t *testing.T, profile string) (Sync, string) {
	t.Helper()
	shippedOnce.Do(func() {
		var err error
		if shippedCatalog, err = catalog.LoadEmbedded(); err != nil {
			t.Fatal(err)
		}
	})
	project := t.TempDir()
	write(t, project, ".ai/project.yaml", profile)
	return Sync{Catalog: shippedCatalog, ProjectDir: project}, project
}

func write(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(root, relative string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
	return err == nil
}

func isLink(root, relative string) bool {
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(relative)))
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func run(t *testing.T, s Sync, force, adopt bool) *Result {
	t.Helper()
	result, err := s.Run(force, adopt)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// mustFail runs a sync that must stop with a sync Error whose message contains part.
func mustFail(t *testing.T, s Sync, part string) {
	t.Helper()
	_, err := s.Run(false, false)
	var syncErr *Error
	if !errors.As(err, &syncErr) || !strings.Contains(syncErr.Message, part) {
		t.Fatalf("want a sync error containing %q, got %v", part, err)
	}
}

func object(t *testing.T, text string) *omap.Map[any] {
	t.Helper()
	value, err := dartjson.Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := value.(*omap.Map[any])
	if !ok {
		t.Fatalf("not an object: %s", text)
	}
	return m
}

func encode(t *testing.T, value any) string {
	t.Helper()
	text, err := dartjson.Encode(value)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func lock(t *testing.T, project string) *omap.Map[any] {
	t.Helper()
	return object(t, read(t, project, LockPath))
}

func lockedPaths(t *testing.T, project string) []string {
	t.Helper()
	return lock(t, project).Value("files").(*omap.Map[any]).Keys()
}

func TestFirstSyncWritesTheClaudeCodeToolkit(t *testing.T) {
	s, project := newProject(t, defaultProfile)

	result := run(t, s, false, false)

	if len(result.Written) == 0 {
		t.Error("nothing written")
	}
	agents := read(t, project, "AGENTS.md")
	if !strings.HasPrefix(agents, GeneratedMarker+"\n\n") || !strings.Contains(agents, ProjectBegin) {
		t.Error("AGENTS.md frame")
	}
	if strings.TrimSpace(read(t, project, "CLAUDE.md")) != "@AGENTS.md" {
		t.Error("CLAUDE.md")
	}
	reviewer := read(t, project, ".claude/agents/reviewer.md")
	if !strings.HasPrefix(reviewer, "---\nname: reviewer\n") || !strings.Contains(reviewer, "tools: Read, Grep, Glob, Bash\nmodel: opus\neffort: high\n") {
		t.Errorf("reviewer %q", reviewer)
	}
	if !strings.Contains(read(t, project, ".claude/agents/explorer.md"), "tools: Read, Grep, Glob, Bash\nmodel: sonnet\neffort: medium\n") {
		t.Error("explorer")
	}
	if !strings.HasPrefix(read(t, project, ".claude/skills/plan/SKILL.md"), "---\nname: plan\ndescription: ") ||
		!strings.Contains(read(t, project, ".claude/skills/bloc-patterns/SKILL.md"), "name: bloc-patterns") {
		t.Error("skills")
	}
	if got := encode(t, object(t, read(t, project, GuardPolicyPath)).Value("forbidden_imports")); got != `["package:posthog_flutter"]` {
		t.Errorf("forbidden_imports %s", got)
	}
	preToolUse := object(t, read(t, project, ".claude/settings.json")).Value("hooks").(*omap.Map[any]).Value("PreToolUse").([]any)
	guards := 0
	for _, hook := range preToolUse {
		if strings.Contains(encode(t, hook), ".ai/bin/guard") {
			guards++
		}
	}
	if guards != 1 {
		t.Errorf("%d guard hooks", guards)
	}
	if !object(t, read(t, project, ".mcp.json")).Value("mcpServers").(*omap.Map[any]).Has("dart-mcp-server") {
		t.Error(".mcp.json")
	}
	if !exists(project, guard.ShimPath) || !exists(project, LockPath) {
		t.Error("shim or lock missing")
	}
}

func TestSecondSyncChangesNothing(t *testing.T) {
	s, _ := newProject(t, defaultProfile)
	run(t, s, false, false)

	result := run(t, s, false, false)

	if len(result.Written) != 0 || len(result.Removed) != 0 {
		t.Errorf("written %v, removed %v", result.Written, result.Removed)
	}
}

func TestForeignSettingsHooksAndMcpServersArePreserved(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	write(t, project, ".claude/settings.json", `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"/usr/local/bin/other-hook"}]}],"Stop":[{"hooks":[{"type":"command","command":"notify"}]}]},"permissions":{"allow":["Bash(ls)"]}}`)
	write(t, project, ".mcp.json", `{"mcpServers":{"graph":{"command":"graph","args":["serve"]}}}`)

	run(t, s, false, false)
	run(t, s, false, false)

	settings := read(t, project, ".claude/settings.json")
	for _, part := range []string{"/usr/local/bin/other-hook", "notify", "Bash(ls)"} {
		if !strings.Contains(settings, part) {
			t.Errorf("settings lost %s", part)
		}
	}
	if strings.Count(settings, ".ai/bin/guard") != 1 || !strings.Contains(read(t, project, ".mcp.json"), `"graph"`) {
		t.Errorf("settings %s", settings)
	}
}

func TestTheLockRecordsOwnedMcpServersPerAgent(t *testing.T) {
	s, project := newProject(t, defaultProfile)

	run(t, s, false, false)

	if got := encode(t, lock(t, project).Value("mcp_servers")); got != `{"claude-code":["dart-mcp-server"]}` {
		t.Errorf("mcp_servers %s", got)
	}
}

func TestAProfileThatStillSetsPlanGateSyncsWithAWarning(t *testing.T) {
	s, project := newProject(t, "toolkit: 1\nagents: [claude-code]\npolicy: { plan_gate: true }\n")

	result := run(t, s, false, false)

	if !slices.Contains(result.Warnings, "plan_gate was removed in P4; delete it from .ai/project.yaml") {
		t.Errorf("warnings %v", result.Warnings)
	}
	if strings.Contains(read(t, project, GuardPolicyPath), "plan_gate") {
		t.Error("plan_gate in the guard policy")
	}
}

func TestALockWithTheOldMcpListIsReadAsClaudeCodeOwnership(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	run(t, s, false, false)
	current := lock(t, project)
	current.Set("mcp_servers", []any{"dart-mcp-server"})
	write(t, project, LockPath, encode(t, current))
	write(t, project, ".mcp.json", `{"mcpServers":{"dart-mcp-server":{"command":"old"}}}`)

	result := run(t, s, false, false)

	if len(result.Warnings) != 0 || strings.Contains(read(t, project, ".mcp.json"), `"old"`) {
		t.Errorf("warnings %v", result.Warnings)
	}
	if got := encode(t, lock(t, project).Value("mcp_servers")); got != `{"claude-code":["dart-mcp-server"]}` {
		t.Errorf("mcp_servers %s", got)
	}
}

func TestAdapterForNamesTheSupportedAgentsAndRejectsTheOthers(t *testing.T) {
	s, project := newProject(t, "toolkit: 1\nagents: [claude-code]\n")
	for _, agent := range []string{"claude-code", "codex", "antigravity"} {
		adapter, err := adapters.AdapterFor(agent, s.Catalog, project, "flutter")
		if err != nil || adapter.Agent() != agent {
			t.Errorf("%s: %v", agent, err)
		}
	}
	if _, err := adapters.AdapterFor("cursor", s.Catalog, project, "flutter"); err == nil {
		t.Error("cursor accepted")
	}
}

func TestAHandEditedGeneratedFileBlocksSyncUntilForce(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	run(t, s, false, false)
	write(t, project, ".claude/skills/plan/SKILL.md", "edited by hand\n")

	mustFail(t, s, ".claude/skills/plan/SKILL.md")

	run(t, s, true, false)
	if read(t, project, ".claude/skills/plan/SKILL.md") == "edited by hand\n" {
		t.Error("not overwritten")
	}
}

func TestAnUnlockedFileThatDiffersIsAConflictButAMatchingOneIsNot(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	rendered, err := s.Render(false, nil)
	if err != nil {
		t.Fatal(err)
	}
	write(t, project, ".claude/agents/reviewer.md", rendered.Files.Value(".claude/agents/reviewer.md"))
	write(t, project, ".claude/agents/explorer.md", "someone else's explorer\n")

	_, err = s.Run(false, false)
	if err == nil || err.Error() != "Files changed outside dashcrew (rerun with --force to overwrite):\n  .claude/agents/explorer.md" {
		t.Errorf("got %v", err)
	}
}

func TestProjectBlockSurvivesSync(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	run(t, s, false, false)
	write(t, project, "AGENTS.md", strings.Replace(read(t, project, "AGENTS.md"), ProjectBegin, ProjectBegin+"\nTeam rule: deploy on Fridays never.", 1))

	run(t, s, false, false)

	if !strings.Contains(read(t, project, "AGENTS.md"), "Team rule: deploy on Fridays never.") {
		t.Error("project block lost")
	}
}

func TestExistingAgentsMdNotMadeByDashcrewNeedsAdopt(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	write(t, project, "AGENTS.md", "Our own rules.\n")

	mustFail(t, s, "--adopt")

	run(t, s, false, true)
	agents := read(t, project, "AGENTS.md")
	if !strings.Contains(agents, "dashcrew:generated") || !strings.Contains(agents, ProjectBegin+"\nOur own rules.\n"+ProjectEnd) {
		t.Errorf("AGENTS.md %q", agents[len(agents)-100:])
	}
}

func TestExistingClaudeMdIsKeptAndReportedWhenItDoesNotImportAgentsMd(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	write(t, project, "CLAUDE.md", "Personal notes.\n")

	result := run(t, s, false, false)

	if read(t, project, "CLAUDE.md") != "Personal notes.\n" || !strings.Contains(strings.Join(result.Warnings, "\n"), "CLAUDE.md") {
		t.Errorf("warnings %v", result.Warnings)
	}
}

func TestOnlyTheReviewerAndExplorerSubagentsAreGenerated(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	run(t, s, false, false)

	entries, err := os.ReadDir(filepath.Join(project, ".claude", "agents"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !reflect.DeepEqual(names, []string{"explorer.md", "reviewer.md"}) {
		t.Errorf("agents %v", names)
	}
}

func TestFilesNoLongerGeneratedAreRemoved(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	run(t, s, false, false)
	if !exists(project, ".claude/skills/bloc-patterns/SKILL.md") {
		t.Fatal("bloc-patterns not written")
	}
	write(t, project, ".ai/project.yaml", "toolkit: 1\nagents: [claude-code]\n")

	result := run(t, s, false, false)

	if exists(project, ".claude/skills/bloc-patterns/SKILL.md") || !slices.Contains(result.Removed, ".claude/skills/bloc-patterns/SKILL.md") {
		t.Errorf("removed %v", result.Removed)
	}
	if len(result.Notes) != 0 {
		t.Errorf("notes %v", result.Notes)
	}
}

func TestP4MigrationNote(t *testing.T) {
	lockRoleFile := func(t *testing.T, project, path string) {
		const content = "---\nname: tech-lead\n---\n"
		write(t, project, path, content)
		current := lock(t, project)
		current.Value("files").(*omap.Map[any]).Set(path, FileHash(path, content))
		write(t, project, LockPath, encode(t, current))
	}

	t.Run("a fresh sync prints no note", func(t *testing.T) {
		s, _ := newProject(t, defaultProfile)
		if notes := run(t, s, false, false).Notes; len(notes) != 0 {
			t.Errorf("notes %v", notes)
		}
	})

	t.Run("a locked role file from before P4 prints the note once and is removed", func(t *testing.T) {
		s, project := newProject(t, defaultProfile)
		run(t, s, false, false)
		lockRoleFile(t, project, ".codex/agents/tech-lead.toml")

		result := run(t, s, false, false)

		if !reflect.DeepEqual(result.Notes, []string{P4MigrationNote}) || !reflect.DeepEqual(result.Removed, []string{".codex/agents/tech-lead.toml"}) {
			t.Errorf("notes %v, removed %v", result.Notes, result.Removed)
		}
		if notes := run(t, s, false, false).Notes; len(notes) != 0 {
			t.Errorf("second notes %v", notes)
		}
	})

	t.Run("the note names what was removed and where the README explains it", func(t *testing.T) {
		for _, part := range []string{"roles", "plan gate", "orchestrate team", "Migrating to P4"} {
			if !strings.Contains(P4MigrationNote, part) {
				t.Errorf("note misses %q", part)
			}
		}
	})

	t.Run("only role files of a pre-P4 role count", func(t *testing.T) {
		cases := map[string]bool{
			".claude/agents/tech-lead.md":        true,
			".codex/agents/qa-engineer.toml":     true,
			".agents/agents/worktree-monitor.md": true,
			".claude/agents/reviewer.md":         false,
			".claude/skills/tech-lead/SKILL.md":  false,
			"x/.claude/agents/tech-lead.md":      false,
		}
		for path, want := range cases {
			if isPreP4Role(path) != want {
				t.Errorf("isPreP4Role(%s) != %v", path, want)
			}
		}
	})
}

func TestCodexAloneWritesItsToolkitAndNothingForClaudeCode(t *testing.T) {
	s, project := newProject(t, "toolkit: 1\nagents: [codex]\nstack: { analytics: posthog }\n")

	run(t, s, false, false)

	if !exists(project, "AGENTS.md") || !exists(project, ".agents/skills/plan/SKILL.md") || !exists(project, ".codex/agents/reviewer.toml") ||
		!strings.Contains(read(t, project, ".codex/hooks.json"), ".ai/bin/guard") ||
		!strings.Contains(read(t, project, ".codex/config.toml"), "[mcp_servers.dart-mcp-server]") {
		t.Error("codex files")
	}
	if exists(project, "CLAUDE.md") || exists(project, ".claude/settings.json") {
		t.Error("claude code files written")
	}
	if got := encode(t, lock(t, project).Value("mcp_servers")); got != `{"codex":["dart-mcp-server"]}` {
		t.Errorf("mcp_servers %s", got)
	}
	if written := run(t, s, false, false).Written; len(written) != 0 {
		t.Errorf("second sync wrote %v", written)
	}
}

func TestAntigravityAloneWritesItsToolkitAndNothingForTheOtherAgents(t *testing.T) {
	s, project := newProject(t, "toolkit: 1\nagents: [antigravity]\n")

	run(t, s, false, false)

	if !exists(project, ".agents/skills/plan/SKILL.md") || !exists(project, ".agents/agents/reviewer.md") ||
		!strings.Contains(read(t, project, ".agents/hooks.json"), "dashcrew-guard") ||
		!strings.Contains(read(t, project, ".agents/mcp_config.json"), "dart-mcp-server") {
		t.Error("antigravity files")
	}
	if exists(project, "CLAUDE.md") || exists(project, ".codex/hooks.json") {
		t.Error("other agents' files written")
	}
	if got := encode(t, lock(t, project).Value("mcp_servers")); got != `{"antigravity":["dart-mcp-server"]}` {
		t.Errorf("mcp_servers %s", got)
	}
	if written := run(t, s, false, false).Written; len(written) != 0 {
		t.Errorf("second sync wrote %v", written)
	}
}

func TestCodexAndAntigravityShareSkillsWrittenOnce(t *testing.T) {
	s, project := newProject(t, "toolkit: 1\nagents: [codex, antigravity]\n")

	result := run(t, s, false, false)

	count := 0
	for _, path := range result.Written {
		if path == ".agents/skills/plan/SKILL.md" {
			count++
		}
	}
	if count != 1 || !exists(project, ".codex/agents/explorer.toml") || !exists(project, ".agents/agents/explorer.md") {
		t.Errorf("plan skill written %d times", count)
	}
	if written := run(t, s, false, false).Written; len(written) != 0 {
		t.Errorf("second sync wrote %v", written)
	}
}

func TestBothAgentsShareAgentsMdAndTheGuardPolicy(t *testing.T) {
	s, project := newProject(t, "toolkit: 1\nagents: [claude-code, codex]\n")

	run(t, s, false, false)

	if !exists(project, ".claude/skills/plan/SKILL.md") || !exists(project, ".agents/skills/plan/SKILL.md") ||
		!strings.Contains(read(t, project, ".claude/settings.json"), ".ai/bin/guard") ||
		!strings.Contains(read(t, project, ".codex/hooks.json"), ".ai/bin/guard") {
		t.Error("shared files")
	}
	if written := run(t, s, false, false).Written; len(written) != 0 {
		t.Errorf("second sync wrote %v", written)
	}
}

func TestDashcrewVersionInTheLock(t *testing.T) {
	setLockVersion := func(t *testing.T, project string, value any) {
		current := lock(t, project)
		if value == nil {
			current.Delete("dashcrew_version")
		} else {
			current.Set("dashcrew_version", value)
		}
		write(t, project, LockPath, encode(t, current))
	}

	t.Run("sync records the version that wrote the lock", func(t *testing.T) {
		s, project := newProject(t, defaultProfile)
		run(t, s, false, false)
		if got := lock(t, project).Value("dashcrew_version"); got != version.Version {
			t.Errorf("version %v", got)
		}
	})

	t.Run("a lock from a newer dashcrew stops sync until --force", func(t *testing.T) {
		s, project := newProject(t, defaultProfile)
		run(t, s, false, false)
		setLockVersion(t, project, "999.0.0")

		mustFail(t, s, "999.0.0")

		run(t, s, true, false)
		if got := lock(t, project).Value("dashcrew_version"); got != version.Version {
			t.Errorf("version %v", got)
		}
	})

	t.Run("a lock from an older dashcrew, or with no version, is upgraded", func(t *testing.T) {
		s, project := newProject(t, defaultProfile)
		run(t, s, false, false)
		for _, value := range []any{"0.0.0", nil} {
			setLockVersion(t, project, value)
			run(t, s, false, false)
			if got := lock(t, project).Value("dashcrew_version"); got != version.Version {
				t.Errorf("from %v: version %v", value, got)
			}
		}
	})
}

func TestPullRequestTemplate(t *testing.T) {
	t.Run("sync writes the template with every required section", func(t *testing.T) {
		s, project := newProject(t, defaultProfile)
		run(t, s, false, false)

		template := read(t, project, PrTemplatePath)
		if !strings.HasPrefix(template, "<!-- dashcrew:generated") || !strings.Contains(template, "## Analytics") {
			t.Errorf("template %q", template)
		}
		if !slices.Contains(lockedPaths(t, project), PrTemplatePath) {
			t.Error("template not locked")
		}
	})

	t.Run("a template the team wrote is left alone with a warning", func(t *testing.T) {
		s, project := newProject(t, defaultProfile)
		write(t, project, PrTemplatePath, "## Our own\n")

		result := run(t, s, false, false)

		if read(t, project, PrTemplatePath) != "## Our own\n" || !strings.Contains(strings.Join(result.Warnings, "\n"), "pull_request_template.md") {
			t.Errorf("warnings %v", result.Warnings)
		}
		if slices.Contains(lockedPaths(t, project), PrTemplatePath) {
			t.Error("foreign template locked")
		}
	})
}

func TestLinksInTheProject(t *testing.T) {
	setup := func(t *testing.T) (Sync, string, string) {
		s, project := newProject(t, "toolkit: 1\nagents: [claude-code]\n")
		return s, project, t.TempDir()
	}
	link := func(t *testing.T, project, relative, target string) {
		path := filepath.Join(project, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("a generated file that is a link is reported, and --force never writes into its target", func(t *testing.T) {
		s, project, outside := setup(t)
		write(t, outside, "planner.md", "team planner\n")
		source := filepath.Join(outside, "planner.md")
		link(t, project, ".claude/agents/reviewer.md", source)

		mustFail(t, s, "link to "+source)
		if read(t, outside, "planner.md") != "team planner\n" {
			t.Error("target changed")
		}

		run(t, s, true, false)

		if read(t, outside, "planner.md") != "team planner\n" || isLink(project, ".claude/agents/reviewer.md") ||
			!strings.Contains(read(t, project, ".claude/agents/reviewer.md"), "name: reviewer") {
			t.Error("link not replaced by a real file")
		}
	})

	t.Run("a generated file inside a linked folder is reported, and --force leaves the folder target alone", func(t *testing.T) {
		s, project, outside := setup(t)
		write(t, outside, "skills/verify/SKILL.md", "team verify\n")
		link(t, project, ".claude/skills/verify", filepath.Join(outside, "skills", "verify"))

		mustFail(t, s, ".claude/skills/verify")

		run(t, s, true, false)

		if read(t, outside, "skills/verify/SKILL.md") != "team verify\n" || isLink(project, ".claude/skills/verify") ||
			!strings.Contains(read(t, project, ".claude/skills/verify/SKILL.md"), "name: verify") {
			t.Error("linked folder not replaced")
		}
	})

	t.Run("a merged file that is a link is merged into a real file, its target untouched", func(t *testing.T) {
		s, project, outside := setup(t)
		write(t, outside, "settings.json", `{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "notify"}]}]}}`)
		link(t, project, ".claude/settings.json", filepath.Join(outside, "settings.json"))

		run(t, s, false, false)

		source := read(t, outside, "settings.json")
		if !strings.Contains(source, `"notify"`) || strings.Contains(source, ".ai/bin/guard") || isLink(project, ".claude/settings.json") {
			t.Errorf("source %s", source)
		}
		settings := read(t, project, ".claude/settings.json")
		if !strings.Contains(settings, "notify") || !strings.Contains(settings, ".ai/bin/guard") {
			t.Errorf("settings %s", settings)
		}
	})
}

func TestGithubWorkflow(t *testing.T) {
	const ciProfile = "toolkit: 1\nagents: [claude-code]\nci: { github: true, dashcrew_ref: 0123abc }\n"

	t.Run("ci.github writes a workflow that runs dashcrew check at the pinned ref", func(t *testing.T) {
		s, project := newProject(t, ciProfile)
		run(t, s, false, false)

		text := read(t, project, GithubWorkflowPath)
		parsed, err := yamlutil.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		workflow := parsed.(*omap.Map[any])
		if keys := workflow.Value("on").(*omap.Map[any]).Keys(); !reflect.DeepEqual(keys, []string{"pull_request"}) {
			t.Errorf("on %v", keys)
		}
		steps := workflow.Value("jobs").(*omap.Map[any]).Value("check").(*omap.Map[any]).Value("steps").([]any)
		var runs []string
		for _, step := range steps {
			if command, ok := step.(*omap.Map[any]).Value("run").(string); ok {
				runs = append(runs, command)
			}
		}
		if !strings.Contains(strings.Join(runs, "\n"), "--git-ref 0123abc") || runs[len(runs)-1] != "dart pub global run dashcrew check" {
			t.Errorf("runs %v", runs)
		}
		if strings.Contains(text, "GITHUB_TOKEN") {
			t.Error("token in workflow")
		}
		if keys := workflow.Value("permissions").(*omap.Map[any]).Keys(); !reflect.DeepEqual(keys, []string{"contents", "pull-requests"}) {
			t.Errorf("permissions %v", keys)
		}
		if !slices.Contains(lockedPaths(t, project), GithubWorkflowPath) {
			t.Error("workflow not locked")
		}
	})

	t.Run("no workflow without ci.github, and turning it off removes it", func(t *testing.T) {
		s, project := newProject(t, "toolkit: 1\nagents: [claude-code]\n")
		run(t, s, false, false)
		if exists(project, GithubWorkflowPath) {
			t.Fatal("workflow without ci.github")
		}
		write(t, project, ".ai/project.yaml", ciProfile)
		run(t, s, false, false)
		write(t, project, ".ai/project.yaml", "toolkit: 1\nagents: [claude-code]\n")

		result := run(t, s, false, false)

		if exists(project, GithubWorkflowPath) || !slices.Contains(result.Removed, GithubWorkflowPath) {
			t.Errorf("removed %v", result.Removed)
		}
	})

	t.Run("a workflow the team wrote under that name is left alone with a warning", func(t *testing.T) {
		s, project := newProject(t, ciProfile)
		write(t, project, GithubWorkflowPath, "name: ours\n")

		result := run(t, s, false, false)

		if read(t, project, GithubWorkflowPath) != "name: ours\n" || !strings.Contains(strings.Join(result.Warnings, "\n"), GithubWorkflowPath) {
			t.Errorf("warnings %v", result.Warnings)
		}
	})
}

func TestGuardShim(t *testing.T) {
	t.Run("sync writes an executable shim for this dashcrew and locks it", func(t *testing.T) {
		s, project := newProject(t, defaultProfile)
		run(t, s, false, false)

		hash, err := guard.SourceHash()
		if err != nil {
			t.Fatal(err)
		}
		if read(t, project, guard.ShimPath) != guard.Shim(hash) {
			t.Error("shim content")
		}
		info, err := os.Stat(filepath.Join(project, ".ai", "bin", "guard"))
		if err != nil || info.Mode().Perm()&0o111 != 0o111 {
			t.Errorf("shim mode %v, %v", info, err)
		}
		if !slices.Contains(lockedPaths(t, project), guard.ShimPath) || lock(t, project).Value("guard_source") != hash {
			t.Error("shim not locked")
		}
		if exists(project, ".gitignore") {
			t.Error(".gitignore written")
		}
	})

	t.Run("the .ai/bin/ ignore line dashcrew added is removed, other lines stay", func(t *testing.T) {
		s, project := newProject(t, defaultProfile)
		write(t, project, ".gitignore", "build/\n# dashcrew: compiled guard, per machine\n.ai/bin/\n.env\n")

		result := run(t, s, false, false)

		if read(t, project, ".gitignore") != "build/\n.env\n" || !slices.Contains(result.Written, ".gitignore") {
			t.Errorf(".gitignore %q", read(t, project, ".gitignore"))
		}
	})

	t.Run("a compiled guard left by an older dashcrew is replaced without --force", func(t *testing.T) {
		s, project := newProject(t, defaultProfile)
		write(t, project, guard.ShimPath, string([]byte{0x7f, 0x45, 0x4c, 0x46, 0x00, 0xff, 0xfe}))

		run(t, s, false, false)

		if !strings.HasPrefix(read(t, project, guard.ShimPath), "#!/bin/sh") {
			t.Error("compiled guard kept")
		}
	})
}

func TestProjectPacks(t *testing.T) {
	writeArchitecturePack := func(t *testing.T, project string) {
		write(t, project, ".ai/packs/architecture/clean/pack.yaml", "description: clean\nparams: { domain_dir: lib/domain }\n")
		write(t, project, ".ai/packs/architecture/clean/rules.md", "---\ntarget: agents_md\norder: 380\n---\n## Clean layers\n\nUse cases live in `{{pack.architecture.domain_dir}}`.\n")
	}

	t.Run("project family renders and the lock records pack sources", func(t *testing.T) {
		s, project := newProject(t, "toolkit: 1\nagents: [claude-code]\nstack:\n  state: bloc\n  di: get_it\n  router: { use: other, name: go_router }\n  architecture: clean\n")
		writeArchitecturePack(t, project)
		write(t, project, ".ai/packs/state/bloc/pack.yaml", "description: team bloc\n")
		write(t, project, ".ai/packs/state/bloc/rules.md", "---\ntarget: agents_md\norder: 300\n---\n## State: team bloc\n")

		result := run(t, s, false, false)

		if !slices.Contains(result.Warnings, "skill bloc-patterns skipped: no selected pack defines skills/bloc-patterns.yaml") ||
			exists(project, ".claude/skills/bloc-patterns/SKILL.md") {
			t.Errorf("warnings %v", result.Warnings)
		}
		agents := read(t, project, "AGENTS.md")
		if !strings.Contains(agents, "Use cases live in `lib/domain`.") || !strings.Contains(agents, "## State: team bloc") ||
			strings.Contains(agents, "## State: flutter_bloc") {
			t.Error("AGENTS.md packs")
		}
		if got := encode(t, lock(t, project).Value("packs")); got != `{"architecture/clean":"project","di/get_it":"builtin","state/bloc":"project"}` {
			t.Errorf("packs %s", got)
		}
	})

	t.Run("project family without its pack is an unknown family", func(t *testing.T) {
		s, _ := newProject(t, "toolkit: 1\nagents: [claude-code]\nstack: { architecture: clean }\n")
		mustFail(t, s, "stack.architecture")
	})

	t.Run("broken project pack stops sync with its path", func(t *testing.T) {
		s, project := newProject(t, "toolkit: 1\nagents: [claude-code]\n")
		write(t, project, ".ai/packs/state/bloc/rules.md", "---\ntarget: agents_md\n---\nx\n")

		_, err := s.Run(false, false)
		if err == nil || err.Error() != "Project pack: .ai/packs/state/bloc/pack.yaml: missing pack.yaml" {
			t.Errorf("got %v", err)
		}
	})
}

func TestProfileErrorsAndAMissingProfile(t *testing.T) {
	s, project := newProject(t, "toolkit: 1\nagents: [claude-code]\nstak: {}\n")
	if _, err := s.Run(false, false); err == nil || err.Error() != "project.yaml stak: unknown key" {
		t.Errorf("got %v", err)
	}

	os.Remove(filepath.Join(project, ".ai", "project.yaml"))
	if _, err := s.Run(false, false); err == nil || !strings.HasPrefix(err.Error(), "No .ai/project.yaml in ") {
		t.Errorf("got %v", err)
	}
}

func TestRenderedFilesKeepTheDartOrder(t *testing.T) {
	s, _ := newProject(t, "toolkit: 1\nagents: [claude-code]\nci: { github: true, dashcrew_ref: main }\n")

	rendered, err := s.Render(false, nil)
	if err != nil {
		t.Fatal(err)
	}

	keys := rendered.Files.Keys()
	head := []string{"AGENTS.md", GuardPolicyPath, guard.ShimPath, PrTemplatePath, GithubWorkflowPath, ".claude/agents/explorer.md"}
	if !reflect.DeepEqual(keys[:len(head)], head) || keys[len(keys)-1] != "CLAUDE.md" {
		t.Errorf("order %v", keys)
	}
}

func TestFileHashIgnoresTheAgentsMdProjectBlockOnly(t *testing.T) {
	a := "x\n" + ProjectBegin + "\none\n" + ProjectEnd + "\n"
	b := "x\n" + ProjectBegin + "\ntwo\n" + ProjectEnd + "\n"
	if FileHash("AGENTS.md", a) != FileHash("AGENTS.md", b) {
		t.Error("AGENTS.md project block counted")
	}
	if FileHash("README.md", a) == FileHash("README.md", b) {
		t.Error("project block ignored outside AGENTS.md")
	}
	if FileHash("AGENTS.md", ProjectEnd+ProjectBegin) != FileHash("x", ProjectEnd+ProjectBegin) {
		t.Error("an end before the begin is not a block")
	}
}
