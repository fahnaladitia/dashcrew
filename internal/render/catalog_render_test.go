package render

import (
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
)

const (
	agentsMdLimit            = 32 * 1024
	antigravityAgentsMdLimit = 24000
)

var (
	shippedOnce    sync.Once
	shippedCatalog *catalog.Catalog
)

func shipped(t *testing.T) *catalog.Catalog {
	t.Helper()
	shippedOnce.Do(func() {
		var err error
		if shippedCatalog, err = catalog.LoadEmbedded(); err != nil {
			t.Fatal(err)
		}
	})
	return shippedCatalog
}

func renderShipped(t *testing.T, yaml string) []RenderedTarget {
	t.Helper()
	return renderAll(t, shipped(t), yaml)
}

func targetContent(t *testing.T, targets []RenderedTarget, name string) string {
	t.Helper()
	for _, target := range targets {
		if target.Target == name {
			return target.Content
		}
	}
	t.Fatalf("no target %s", name)
	return ""
}

func targetNames(targets []RenderedTarget) map[string]bool {
	out := map[string]bool{}
	for _, target := range targets {
		out[target.Target] = true
	}
	return out
}

func flutterFamilies(t *testing.T) ([]string, map[string][]string) {
	known := shipped(t).KnownOptions()["flutter"]
	options := map[string][]string{}
	for _, family := range known.Keys() {
		options[family] = known.Value(family)
	}
	return known.Keys(), options
}

func TestShippedCatalogRendersWithNoStack(t *testing.T) {
	targets := renderShipped(t, minimal)

	if size := len(targetContent(t, targets, "agents_md")); size >= agentsMdLimit {
		t.Errorf("AGENTS.md %d bytes", size)
	}
}

func TestShippedCatalogRendersEveryNonNoneOption(t *testing.T) {
	families, options := flutterFamilies(t)
	for _, family := range families {
		for _, option := range options[family] {
			if option == "none" {
				continue
			}
			targets := renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack: { "+family+": "+option+" }\n")
			if size := len(targetContent(t, targets, "agents_md")); size >= agentsMdLimit {
				t.Errorf("%s/%s: AGENTS.md %d bytes", family, option, size)
			}
		}
	}
}

func TestEveryFamilySetToOtherRendersOneLinePerFamily(t *testing.T) {
	families, _ := flutterFamilies(t)
	sorted := append([]string{}, families...)
	sort.Strings(sorted)
	var stack []string
	for _, family := range sorted {
		stack = append(stack, "  "+family+": { use: other, name: lib_"+family+" }")
	}
	targets := renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack:\n"+strings.Join(stack, "\n")+"\n")
	var agentsMd RenderedTarget
	for _, target := range targets {
		if target.Target == "agents_md" {
			agentsMd = target
		}
	}

	if !strings.Contains(agentsMd.Content, "## Libraries without a dashcrew pack") {
		t.Error("no libraries section")
	}
	for _, family := range sorted {
		if !strings.Contains(agentsMd.Content, "- "+family+": `lib_"+family+"`") {
			t.Errorf("no line for %s", family)
		}
	}
	found := false
	for _, rule := range agentsMd.Rules {
		found = found || rule == "stack.other-follow-official-docs"
	}
	if !found {
		t.Error("rule stack.other-follow-official-docs missing")
	}
	if len(agentsMd.Content) >= agentsMdLimit {
		t.Errorf("AGENTS.md %d bytes", len(agentsMd.Content))
	}
}

func TestNoOtherChoiceRendersNoLibrariesSection(t *testing.T) {
	content := targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack: { state: bloc }\n"), "agents_md")

	if strings.Contains(content, "without a dashcrew pack") {
		t.Error("libraries section rendered")
	}
}

func TestFlavorsRenderOneRunLineEachWithTheBuildWrapper(t *testing.T) {
	content := targetContent(t, renderShipped(t, `
toolkit: 1
agents: [claude-code]
project:
  flavors: { staging: lib/main_staging.dart, dev: lib/main_dev.dart }
  build_wrapper: tool/run.sh
`), "agents_md")

	want := "## Build and run\n\n" +
		"- `dev`: `flutter run --flavor dev -t lib/main_dev.dart`\n" +
		"- `staging`: `flutter run --flavor staging -t lib/main_staging.dart`\n" +
		"- Real builds go through `tool/run.sh`"
	if !strings.Contains(content, want) {
		t.Error("build section differs")
	}
}

func TestNoFlavorsRendersNoBuildSection(t *testing.T) {
	content := targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\nproject: { build_wrapper: tool/run.sh }\n"), "agents_md")

	if strings.Contains(content, "## Build and run") {
		t.Error("build section rendered")
	}
}

func TestStatePacks(t *testing.T) {
	agentsMd := func(stack string) string {
		return targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack: { "+stack+" }\n"), "agents_md")
	}
	targets := func(stack string) map[string]bool {
		return targetNames(renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack: { "+stack+" }\n"))
	}

	riverpod := agentsMd("state: riverpod")
	if !strings.Contains(riverpod, "## State: Riverpod") || !strings.Contains(riverpod, "`ref.mounted`") ||
		!strings.Contains(riverpod, "declared by hand") || strings.Contains(riverpod, "@riverpod") {
		t.Error("riverpod rules")
	}
	if names := targets("state: riverpod"); !names["skill/riverpod-patterns"] || !names["workflow/tdd"] {
		t.Error("riverpod targets")
	}

	codegen := agentsMd("state: { use: riverpod, codegen: true }")
	if !strings.Contains(codegen, "`@riverpod`") || strings.Contains(codegen, "declared by hand") {
		t.Error("riverpod codegen rules")
	}

	if !strings.Contains(agentsMd("state: provider"), "## State: provider") || !targets("state: provider")["skill/provider-patterns"] {
		t.Error("provider")
	}

	if !strings.Contains(agentsMd("state: none"), "## State: Flutter only") {
		t.Error("none rules")
	}
	for name := range targets("state: none") {
		if strings.HasSuffix(name, "-patterns") {
			t.Errorf("none renders %s", name)
		}
	}
}

func TestPackHeadings(t *testing.T) {
	headings := map[string]string{
		"di: riverpod":                "## Dependency injection: Riverpod",
		"di: provider":                "## Dependency injection: provider",
		"di: none":                    "## Dependency injection: constructor parameters",
		"router: go_router":           "## Navigation: go_router",
		"router: auto_route":          "## Navigation: auto_route",
		"router: none":                "## Navigation: Navigator",
		"network: http":               "## Network: http",
		"network: retrofit":           "## Network: Retrofit",
		"network: none":               "## Network: no HTTP package",
		"storage: shared_preferences": "## Local storage: shared_preferences",
		"storage: sqflite":            "## Local storage: sqflite",
		"test: mocktail":              "## Tests: mocktail",
		"test: mockito":               "## Tests: mockito",
		"test: none":                  "## Tests: no mocking convention",
		"analytics: firebase":         "## Analytics: Firebase",
		"analytics: mixpanel":         "## Analytics: Mixpanel",
	}
	for stack, heading := range headings {
		if content := targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack: { "+stack+" }\n"), "agents_md"); !strings.Contains(content, heading) {
			t.Errorf("%s does not render %q", stack, heading)
		}
	}
}

func TestGetItWithInjectableSwitchesToAnnotations(t *testing.T) {
	rules := func(di string) string {
		return targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack: { di: "+di+" }\n"), "agents_md")
	}

	injectable := rules("{ use: get_it, injectable: true }")
	if !strings.Contains(injectable, "`@lazySingleton`") || strings.Contains(injectable, "Register every new dependency") {
		t.Error("injectable")
	}
	if !strings.Contains(rules("get_it"), "Register every new dependency") {
		t.Error("get_it")
	}
}

func TestDiRiverpodWithAnotherStateLibrary(t *testing.T) {
	rules := func(stack string) string {
		return targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack: { "+stack+" }\n"), "agents_md")
	}

	if !strings.Contains(rules("di: riverpod, state: bloc"), "`ConsumerWidget`") {
		t.Error("bloc")
	}
	if strings.Contains(rules("di: riverpod, state: riverpod"), "State is not Riverpod") {
		t.Error("riverpod")
	}
}

func TestMocktailWithBlocAddsBlocTest(t *testing.T) {
	tdd := func(stack string) string {
		return targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack: { "+stack+" }\n"), "workflow/tdd")
	}

	if !strings.Contains(tdd("state: bloc, test: mocktail"), "blocTest") || strings.Contains(tdd("state: bloc, test: manual-fakes"), "blocTest") {
		t.Error("blocTest")
	}
}

func TestLargestOptionOfEveryFamilyStaysUnderTheLimit(t *testing.T) {
	families, options := flutterFamilies(t)
	var stack []string
	for _, family := range families {
		largest, bytes := "", -1
		for _, option := range options[family] {
			size := len(targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack: { "+family+": "+option+" }\n"), "agents_md"))
			if size > bytes {
				largest, bytes = option, size
			}
		}
		stack = append(stack, family+": "+largest)
	}

	profile := "toolkit: 1\nagents: [claude-code]\nstack: { " + strings.Join(stack, ", ") + " }\n"
	content := targetContent(t, renderShipped(t, profile), "agents_md")
	withKnowledge := targetContent(t, renderShipped(t, profile+"knowledge: { repo: 'git@example.com:team/notes.git' }\n"), "agents_md")
	t.Logf("%s: AGENTS.md %d bytes, %d with knowledge", strings.Join(stack, ", "), len(content), len(withKnowledge))

	if len(content) >= antigravityAgentsMdLimit {
		t.Errorf("%s: AGENTS.md %d bytes", strings.Join(stack, ", "), len(content))
	}
	if len(withKnowledge) > len(content) {
		t.Errorf("knowledge adds %d bytes to the heaviest AGENTS.md; pay for its line in the knowledge-gated text", len(withKnowledge)-len(content))
	}
}

func prHeadings(t *testing.T, stack string) []string {
	content := targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack: { "+stack+" }\n"), "pr_template")
	var out []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			out = append(out, line)
		}
	}
	return out
}

func TestPrTemplate(t *testing.T) {
	core := []string{"## Summary", "## Changes", "## Affected files", "## Testing", "## Notes / not done"}
	if got := prHeadings(t, ""); !reflect.DeepEqual(got, core) {
		t.Errorf("core %v", got)
	}
	for _, analytics := range []string{"posthog", "firebase", "mixpanel"} {
		want := []string{"## Summary", "## Changes", "## Affected files", "## Testing", "## QA Identifiers", "## Analytics", "## Notes / not done"}
		if got := prHeadings(t, "qa: semantics-identifier, analytics: "+analytics); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v", analytics, got)
		}
	}
}

func TestPlanGateNoLongerChangesRenderedText(t *testing.T) {
	off := renderShipped(t, minimal)
	on := renderShipped(t, "toolkit: 1\nagents: [claude-code]\npolicy: { plan_gate: true }\n")
	for _, name := range []string{"agents_md", "workflow/plan", "pr_template", "skill/pr-description"} {
		content := targetContent(t, on, name)
		if content != targetContent(t, off, name) {
			t.Errorf("%s differs", name)
		}
		for _, banned := range []string{"dashcrew gate", "Plan: #", "/approve"} {
			if strings.Contains(content, banned) {
				t.Errorf("%s contains %q", name, banned)
			}
		}
	}
}

func TestSampleFullStackStaysUnderTheLimitWithEveryPolicyFlipped(t *testing.T) {
	targets := renderShipped(t, `
toolkit: 1
agents: [claude-code, codex]
language: { dartdoc: id, pr: id, chat: id }
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
project:
  local_packages: [core, shared]
  base_branches: [develop, main]
  build_wrapper: tool/run.sh
policy:
  agent_commit: true
  formatter: true
  tier_gates: false
  terse_prompts: true
`)

	if size := len(targetContent(t, targets, "agents_md")); size >= agentsMdLimit {
		t.Errorf("AGENTS.md %d bytes", size)
	}
	names := targetNames(targets)
	for _, name := range []string{"skill/bloc-patterns", "role/reviewer", "role/explorer", "workflow/plan"} {
		if !names[name] {
			t.Errorf("missing %s", name)
		}
	}
}

func TestKnowledgeAddsTheProposalBranchToLearnOnly(t *testing.T) {
	off := renderShipped(t, minimal)
	on := renderShipped(t, "toolkit: 1\nagents: [claude-code]\nknowledge: { repo: git@example.com:team/notes-knowledge.git }\n")
	for _, name := range []string{"workflow/learn", "skill/continuous-learning"} {
		withKnowledge, without := targetContent(t, on, name), targetContent(t, off, name)
		if !strings.HasPrefix(withKnowledge, without) || len(withKnowledge) == len(without) {
			t.Errorf("%s: the knowledge branch must be appended to the unchanged text", name)
		}
		if strings.Contains(without, ".ai/knowledge") {
			t.Errorf("%s mentions knowledge without the profile key", name)
		}
	}
	learn := targetContent(t, on, "workflow/learn")
	for _, want := range []string{".ai/knowledge/inbox/<yyyy-mm-dd>-<slug>.md", ".ai/packs/"} {
		if !strings.Contains(learn, want) {
			t.Errorf("learn lacks %q", want)
		}
	}
	skill := targetContent(t, on, "skill/continuous-learning")
	for _, want := range []string{"kind: feature | repo", "target:", "repo:", "branch:", "commit:", "`<repo>:<path>:<line>`"} {
		if !strings.Contains(skill, want) {
			t.Errorf("continuous-learning lacks %q", want)
		}
	}
	changes := map[string]bool{"workflow/learn": true, "skill/continuous-learning": true, "agents_md": true, "role/explorer": true, "workflow/plan": true}
	for _, target := range on {
		if !changes[target.Target] && target.Content != targetContent(t, off, target.Target) {
			t.Errorf("%s changed with knowledge set", target.Target)
		}
	}
}

func TestKnowledgeAddsALineToAgentsMdExplorerAndPlanOnlyWhenSet(t *testing.T) {
	for _, platform := range []string{"", "platform: other\nplatform_config: { detect: [go.mod], commands: { analyze: go vet, test: go test, test_scoped: go test, deps: go mod download } }\n"} {
		profile := "toolkit: 1\nagents: [claude-code]\n" + platform
		off := renderShipped(t, profile)
		on := renderShipped(t, profile+"knowledge: { repo: git@example.com:team/notes-knowledge.git }\n")
		wants := map[string][]string{
			"agents_md":     {"- Read `.ai/memory` (never rewrite it) and `.ai/knowledge` before planning; verify its code pointers before use.\n"},
			"role/explorer": {"Before sweeping code, search `.ai/knowledge`", "name the pages you used", "Knowledge: <pages used, or none>"},
			"workflow/plan": {"Search `.ai/knowledge` for the feature before the code.", "names every knowledge page the plan relied on"},
		}
		for name, lines := range wants {
			withKnowledge, without := targetContent(t, on, name), targetContent(t, off, name)
			for _, line := range lines {
				if !strings.Contains(withKnowledge, line) {
					t.Errorf("%q %s lacks %q", platform, name, line)
				}
			}
			if strings.Contains(without, ".ai/knowledge") || strings.Contains(without, "Knowledge:") {
				t.Errorf("%q %s mentions knowledge without the profile key", platform, name)
			}
		}
		if agentsMd := targetContent(t, on, "agents_md"); strings.Contains(agentsMd, "team feedback") || len(agentsMd) > len(targetContent(t, off, "agents_md")) {
			t.Errorf("%q: the knowledge line is not paid for by the memory line it replaces", platform)
		}
	}
}
