package adapters

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/render"
)

const fullStack = `
toolkit: 1
agents: [codex]
stack:
  state: bloc
  di: get_it
  router: getx
  network: dio
  storage: hive
  test: manual-fakes
  analytics: posthog
  tracker: clickup
  qa: semantics-identifier
  design: figma
`

func codexFiles(t *testing.T, yaml string) map[string]string {
	t.Helper()
	c := shipped(t)
	return files(t, c, NewCodex(c, t.TempDir(), "flutter"), yaml)
}

func TestCodexSkillsAndWorkflowsHaveNameAndDescriptionOnly(t *testing.T) {
	out := codexFiles(t, fullStack)

	plan := out[".agents/skills/plan/SKILL.md"]
	frontMatter := plan[4 : 4+strings.Index(plan[4:], "\n---\n")]
	var keys []string
	for _, line := range strings.Split(frontMatter, "\n") {
		keys = append(keys, strings.SplitN(line, ":", 2)[0])
	}
	if !reflect.DeepEqual(keys, []string{"name", "description"}) {
		t.Errorf("front matter keys %v", keys)
	}
	if !strings.HasPrefix(plan, "---\nname: plan\ndescription: \"Plan a change") {
		t.Errorf("plan %q", plan[:80])
	}
	if !strings.Contains(plan, "\n---\n\nArguments: `<task description or confirmed step 0 prompt>`\n\n# plan") {
		t.Error("arguments line")
	}
	if _, ok := out[".agents/skills/bloc-patterns/SKILL.md"]; !ok {
		t.Error("bloc-patterns")
	}
	for path := range out {
		if !strings.HasPrefix(path, ".agents/skills/") && !strings.HasPrefix(path, ".codex/agents/") {
			t.Errorf("unexpected file %s", path)
		}
	}
}

func TestCodexFoldsWhenToUseIntoTheDescription(t *testing.T) {
	skill := codexFiles(t, fullStack)[".agents/skills/data-layer-tdd/SKILL.md"]

	if !regexp.MustCompile(`(?m)^description: ".+ Use when: .+"$`).MatchString(skill) {
		t.Errorf("skill %q", skill[:200])
	}
}

func TestCodexBodiesMatchTheClaudeCodeSkills(t *testing.T) {
	c := shipped(t)
	claude := files(t, c, NewClaudeCode(c, t.TempDir(), "flutter"), strings.Replace(fullStack, "[codex]", "[claude-code]", 1))
	body := func(skill string) string { return skill[strings.Index(skill[4:], "\n---\n")+4+5:] }

	for path, content := range codexFiles(t, fullStack) {
		if !strings.HasPrefix(path, ".agents/skills/") {
			continue
		}
		name := strings.Split(path, "/")[2]
		if !strings.HasSuffix(body(content), body(claude[".claude/skills/"+name+"/SKILL.md"])) {
			t.Errorf("%s body differs", name)
		}
	}
}

func TestCodexSkillListFitsTheBudget(t *testing.T) {
	var skills []string
	for path, content := range codexFiles(t, fullStack) {
		if strings.HasPrefix(path, ".agents/skills/") {
			skills = append(skills, content)
		}
	}

	if size := SkillListChars(skills); size >= SkillListBudget {
		t.Errorf("skill list %d chars", size)
	}
}

func TestSkillListCountsNameAndDescription(t *testing.T) {
	if got := SkillListChars([]string{"---\nname: ab\ndescription: \"cde\"\n---\n\nbody\n"}); got != 5 {
		t.Errorf("got %d", got)
	}
}

func codexMerge(t *testing.T, project string, warnings *[]string) MergeResult {
	t.Helper()
	result, err := NewCodex(shipped(t), project, "flutter").Merge(map[string]bool{}, warnings)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCodexGuardHookIsAddedNextToUserHooksAndWrittenOnce(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, ".codex/hooks.json", `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"other-hook"}]}],"PostToolUse":[{"hooks":[{"type":"command","command":"notify"}]}]}}`)

	if written := codexMerge(t, project, nil).Written; !contains(written, ".codex/hooks.json") {
		t.Errorf("first merge %v", written)
	}
	if written := codexMerge(t, project, nil).Written; contains(written, ".codex/hooks.json") {
		t.Errorf("second merge %v", written)
	}

	hooks := decodeMap(t, readFile(t, project, ".codex/hooks.json"))
	preToolUse := hooks.Value("hooks").(*omap.Map[any]).Value("PreToolUse").([]any)
	if len(preToolUse) != 2 {
		t.Fatalf("PreToolUse %d", len(preToolUse))
	}
	last := preToolUse[1].(*omap.Map[any])
	if !strings.Contains(compact(t, last), ".ai/bin/guard") || last.Value("matcher") != CodexGuardMatcher {
		t.Errorf("last %s", compact(t, last))
	}
	if !strings.Contains(compact(t, hooks), "notify") {
		t.Error("notify lost")
	}
}

func TestCodexEditedGuardEntryIsReplacedNotDuplicated(t *testing.T) {
	project := t.TempDir()
	codexMerge(t, project, nil)
	writeFile(t, project, ".codex/hooks.json", strings.Replace(readFile(t, project, ".codex/hooks.json"), `"timeout": 30`, `"timeout": 5`, 1))

	codexMerge(t, project, nil)

	hooks := readFile(t, project, ".codex/hooks.json")
	if strings.Count(hooks, ".ai/bin/guard") != 1 || !strings.Contains(hooks, `"timeout": 30`) {
		t.Errorf("hooks %s", hooks)
	}
}

const codexBlock = `# dashcrew:begin - written by dashcrew sync; edit outside this block
[mcp_servers.dart-mcp-server]
command = "dart"
args = ["mcp-server"]
# dashcrew:end
`

func TestCodexNewConfigHoldsTheBlockOnly(t *testing.T) {
	project := t.TempDir()

	result := codexMerge(t, project, nil)

	if readFile(t, project, ".codex/config.toml") != codexBlock || !contains(result.Written, ".codex/config.toml") {
		t.Errorf("config %q written %v", readFile(t, project, ".codex/config.toml"), result.Written)
	}
	if !reflect.DeepEqual(result.OwnedMcp, map[string]bool{"dart-mcp-server": true}) {
		t.Errorf("owned %v", result.OwnedMcp)
	}
}

func TestCodexBlockGoesAfterTheUserSettings(t *testing.T) {
	project := t.TempDir()
	const user = "model = \"some-model\"\n\n[mcp_servers.graph]\ncommand = \"graph\"\n"
	writeFile(t, project, ".codex/config.toml", user)

	codexMerge(t, project, nil)

	if got := readFile(t, project, ".codex/config.toml"); got != user+"\n"+codexBlock {
		t.Errorf("config %q", got)
	}
	if contains(codexMerge(t, project, nil).Written, ".codex/config.toml") {
		t.Error("second merge rewrote config.toml")
	}
}

func TestCodexStaleBlockIsReplacedInPlace(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, ".codex/config.toml", "model = \"m\"\n\n# dashcrew:begin\n[mcp_servers.old]\ncommand = \"x\"\n# dashcrew:end\n\n[profiles.fast]\nmodel = \"f\"\n")

	codexMerge(t, project, nil)

	if got := readFile(t, project, ".codex/config.toml"); got != "model = \"m\"\n\n"+codexBlock+"\n[profiles.fast]\nmodel = \"f\"\n" {
		t.Errorf("config %q", got)
	}
}

func TestCodexServerDefinedOutsideTheBlockIsLeftAlone(t *testing.T) {
	project := t.TempDir()
	const user = "[mcp_servers.\"dart-mcp-server\"]\ncommand = \"fvm\"\n"
	writeFile(t, project, ".codex/config.toml", user)
	var warnings []string

	result := codexMerge(t, project, &warnings)

	if readFile(t, project, ".codex/config.toml") != user || len(result.OwnedMcp) != 0 {
		t.Errorf("config changed or owned %v", result.OwnedMcp)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "dart-mcp-server") {
		t.Errorf("warnings %v", warnings)
	}
}

func TestDefinesMcpServer(t *testing.T) {
	cases := []struct {
		toml string
		want bool
	}{
		{"[mcp_servers.x]\n", true},
		{"[ mcp_servers . \"x\" ]\n", true},
		{"mcp_servers.x.command = \"a\"\n", true},
		{"mcp_servers.x = { command = \"a\" }\n", true},
		{"[mcp_servers]\nx = { command = \"a\" }\n", true},
		{"[mcp_servers.xy]\n", false},
		{"# [mcp_servers.x]\n", false},
	}
	for _, c := range cases {
		if got := DefinesMcpServer(c.toml, "x"); got != c.want {
			t.Errorf("%q: %v", c.toml, got)
		}
	}
}

func TestCodexRoleFiles(t *testing.T) {
	out := codexFiles(t, fullStack)

	reviewer := out[".codex/agents/reviewer.toml"]
	if !strings.HasPrefix(reviewer, "name = \"reviewer\"\ndescription = \"") || !strings.Contains(reviewer, "\nmodel_reasoning_effort = \"high\"\n") ||
		!strings.Contains(reviewer, "\ndeveloper_instructions = \"\"\"\n") || !strings.HasSuffix(reviewer, "\"\"\"\n") {
		t.Errorf("reviewer %q", reviewer[:200])
	}
	if !strings.Contains(out[".codex/agents/explorer.toml"], "\nmodel_reasoning_effort = \"medium\"\n") {
		t.Error("explorer effort")
	}
}

func TestRoleGatedByAnUnselectedPackIsNotGenerated(t *testing.T) {
	fragment := func(source, target, body string) catalog.Fragment {
		return catalog.Fragment{Source: source, Target: target, Order: 100, Rules: []string{}, When: omap.New[any](), Body: body}
	}
	gated := &catalog.Catalog{
		Core:  []catalog.Fragment{fragment("core/roles/gated/role.md", "role/gated", "Gated."), fragment("core/roles/open/role.md", "role/open", "Open.")},
		Packs: omap.New[*omap.Map[*catalog.Pack]](),
		Roles: map[string]catalog.RoleMeta{
			"gated": {Name: "gated", Description: "Gated.", Capability: "read", When: omap.Of[any]("stack.design.use", []any{"figma"})},
			"open":  {Name: "open", Description: "Open.", Capability: "read", When: omap.New[any]()},
		},
		Skills: map[string]catalog.DocMeta{}, Workflows: map[string]catalog.DocMeta{}, Platforms: map[string]*catalog.PlatformSpec{},
	}
	p, err := profile.Parse("toolkit: 1\nagents: [codex]\n", gated.KnownOptions())
	if err != nil {
		t.Fatal(err)
	}
	r := render.New(gated)
	targets, err := r.Render(p)
	if err != nil {
		t.Fatal(err)
	}

	out, err := NewCodex(gated, t.TempDir(), "flutter").Files(p, targets, r.ContextFor(p), map[string]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if got := keysWithPrefix(plain(out), ".codex/agents/"); !reflect.DeepEqual(got, []string{".codex/agents/open.toml"}) {
		t.Errorf("roles %v", got)
	}
}

func TestTomlMultilineEscapesBackslashesAndTripleQuotes(t *testing.T) {
	if got := TomlMultiline(`a \d """ b`); got != "\"\"\"\n"+`a \\d ""\" b`+"\"\"\"" {
		t.Errorf("got %q", got)
	}
}
