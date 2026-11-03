package render

import (
	"maps"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
)

const (
	agentsMdLimit            = 32 * 1024
	antigravityAgentsMdLimit = 24000
	// agentsMdBudget is the heaviest AGENTS.md before release lines (P6): new catalog lines are paid with cuts.
	agentsMdBudget = 23788
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
	const knowledge = "knowledge: { repo: 'git@example.com:team/notes.git' }\n"
	const releaseLines = "project: { release_lines: { main: develop, release: 'release/(?P<version>[0-9.]+)' } }\n"
	withKnowledge := targetContent(t, renderShipped(t, profile+knowledge), "agents_md")
	withRelease := targetContent(t, renderShipped(t, profile+releaseLines), "agents_md")
	withBoth := targetContent(t, renderShipped(t, profile+knowledge+releaseLines), "agents_md")
	t.Logf("%s: AGENTS.md %d bytes, %d with knowledge, %d with release lines, %d with both", strings.Join(stack, ", "),
		len(content), len(withKnowledge), len(withRelease), len(withBoth))

	for name, size := range map[string]int{"plain": len(content), "knowledge": len(withKnowledge), "release lines": len(withRelease), "both": len(withBoth)} {
		if size > agentsMdBudget {
			t.Errorf("%s: AGENTS.md %d bytes, over the %d budget; pay for new lines with cuts in the same file", name, size, agentsMdBudget)
		}
	}
	if len(withKnowledge) > len(content) {
		t.Errorf("knowledge adds %d bytes to the heaviest AGENTS.md; pay for its line in the knowledge-gated text", len(withKnowledge)-len(content))
	}

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
	changes := map[string]bool{"workflow/learn": true, "skill/continuous-learning": true, "agents_md": true, "role/explorer": true, "workflow/plan": true,
		"role/reviewer": true, "skill/branching": true, "skill/prompt-polish": true}
	added := map[string]bool{"workflow/spec": true, "trd": true}
	for _, target := range on {
		if !targetNames(off)[target.Target] {
			if !added[target.Target] {
				t.Errorf("%s added with knowledge set", target.Target)
			}
			continue
		}
		if !changes[target.Target] && target.Content != targetContent(t, off, target.Target) {
			t.Errorf("%s changed with knowledge set", target.Target)
		}
	}
}

func TestKnowledgeAddsTheSpecWorkflowWithThePlatformTrdSections(t *testing.T) {
	const knowledge = "knowledge: { repo: git@example.com:team/notes-knowledge.git }\n"
	const other = "platform: other\nplatform_config: { detect: [go.mod], commands: { analyze: go vet, test: go test, test_scoped: go test, deps: go mod download } }\n"
	generic := []string{"- `Contracts`: `C-<n>` per endpoint", "- `Rollout order`:", "`Decisions` (`D-<n>`"}
	flutter := []string{"- `State`:", "- `Routing`:", "- `QA identifiers`:"}
	for _, platform := range []string{"", other} {
		off := renderShipped(t, "toolkit: 1\nagents: [claude-code]\n"+platform)
		on := renderShipped(t, "toolkit: 1\nagents: [claude-code]\n"+platform+knowledge)
		if targetNames(off)["workflow/spec"] || targetNames(off)["trd"] {
			t.Errorf("%q: spec without knowledge", platform)
		}
		if strings.Contains(targetContent(t, off, "agents_md"), "`spec`") || !strings.Contains(targetContent(t, on, "agents_md"), "`build-fix`, `spec`. Say") {
			t.Errorf("%q: the closed workflow list", platform)
		}
		spec := targetContent(t, on, "workflow/spec")
		if spec != strings.Replace(spec, "{{", "", 1) || !strings.Contains(spec, "in these sections:\n\n- `Impact`") {
			t.Errorf("%q: sections not inserted:\n%s", platform, spec)
		}
		for _, want := range append(generic, "dashcrew spec new <feature>", "dashcrew spec lint", "dashcrew spec revise", "`acceptance-criteria`",
			"`feature-architecture`", "~~AC-3~~ dropped: <why>", "owner: <role>", "git -C .ai/knowledge push -u origin spec-<feature>",
			"Approval is its merge, with `status: approved`") {
			if !strings.Contains(spec, want) {
				t.Errorf("%q: spec lacks %q", platform, want)
			}
		}
		for _, want := range flutter {
			if strings.Contains(spec, want) != (platform == "") {
				t.Errorf("%q: Flutter section %q present %v", platform, want, platform != "")
			}
		}
	}
}

func TestKnowledgeAddsALineToAgentsMdExplorerAndPlanOnlyWhenSet(t *testing.T) {
	for _, platform := range []string{"", "platform: other\nplatform_config: { detect: [go.mod], commands: { analyze: go vet, test: go test, test_scoped: go test, deps: go mod download } }\n"} {
		profile := "toolkit: 1\nagents: [claude-code]\n" + platform
		off := renderShipped(t, profile)
		on := renderShipped(t, profile+"knowledge: { repo: git@example.com:team/notes-knowledge.git }\n")
		wants := map[string][]string{
			"agents_md":     {"- Read `.ai/memory` (never rewrite) and `.ai/knowledge`; check pointers.\n"},
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

// TestKnowledgeTiesTasksToTheirSpec pins the P7 S3 text: the step 0 Spec line, its record in the branch config, the
// ACs as criteria, the plan's Spec section with declared departures, and the reviewer's checks; none without knowledge.
func TestKnowledgeTiesTasksToTheirSpec(t *testing.T) {
	const knowledge = "knowledge: { repo: git@example.com:team/notes-knowledge.git }\n"
	const releaseLines = "project: { release_lines: { main: develop, release: 'release/(?P<version>[0-9.]+)' } }\n"
	for _, platform := range []string{"", "platform: other\nplatform_config: { detect: [x], commands: { analyze: a, test: t, test_scoped: s, deps: d } }\n"} {
		profile := "toolkit: 1\nagents: [claude-code]\n" + platform
		off, on := renderShipped(t, profile), renderShipped(t, profile+knowledge)
		withLines := renderShipped(t, profile+knowledge+releaseLines)
		checks := []struct{ target, text string }{
			{"skill/prompt-polish", "(<evidence>)\nSpec       specs/<release>/<feature>@rev<n> | none\n"},
			{"skill/prompt-polish", "- `Spec`: the spec the task builds, `specs/<release>/<feature>@rev<n>`"},
			{"skill/prompt-polish", "the success criteria are the ACs this task covers, by ID"},
			{"skill/branching", "\n7. With a spec, record it: `git config branch.<name>.dashcrew-spec specs/<release>/<feature>@rev<n>`."},
			{"workflow/plan", "### Spec\n"},
			{"workflow/plan", "`departs from D-n: <what and why>`"},
			{"workflow/plan", "makes the verdict NO-GO"},
			{"workflow/plan", "needs a `spec revise` pull request on the knowledge repo, opened by a member; the summary names it as follow-up."},
			{"role/reviewer", "the step 0 success criteria and `Spec`, the plan path"},
			{"role/reviewer", "- Spec: with a step 0 `Spec`, the plan names the spec and revision"},
			{"role/reviewer", "that the plan does not declare as a departure is HIGH."},
			{"workflow/learn", "propose its `features/<slug>.md` page the same way, through the inbox"},
			{"workflow/learn", "marked `superseded` only when a newer spec replaces it"},
		}
		for _, c := range checks {
			if strings.Contains(targetContent(t, off, c.target), c.text) {
				t.Errorf("%q %s without knowledge holds %q", platform, c.target, c.text)
			}
			if !strings.Contains(targetContent(t, on, c.target), c.text) {
				t.Errorf("%q %s with knowledge lacks %q", platform, c.target, c.text)
			}
		}
		for _, target := range []string{"agents_md", "role/reviewer", "workflow/plan", "skill/branching", "skill/prompt-polish"} {
			if strings.Contains(targetContent(t, off, target), "dashcrew-spec") || strings.Contains(targetContent(t, off, target), "Spec:") {
				t.Errorf("%q %s mentions a spec without knowledge", platform, target)
			}
		}
		if branching := targetContent(t, withLines, "skill/branching"); !strings.Contains(branching, "\n8. With a spec, record it:") {
			t.Errorf("%q: with release lines the spec step is 8:\n%s", platform, branching)
		}
		if agentsMd := targetContent(t, on, "agents_md"); strings.Contains(agentsMd, "`spec-driven-development`") {
			t.Errorf("%q: spec-driven-development is listed next to the spec workflow", platform)
		}
		if !strings.Contains(targetContent(t, off, "agents_md"), "`spec-driven-development`") {
			t.Errorf("%q: spec-driven-development is gone without knowledge", platform)
		}
	}
}

func TestReleaseTextRendersOnlyWithReleaseLines(t *testing.T) {
	const releaseLines = "project: { release_lines: { main: develop, release: 'release/(?P<version>[0-9.]+)' } }\n"
	for _, platform := range []string{"", "platform: other\nplatform_config: { detect: [x], commands: { analyze: a, test: t, test_scoped: s, deps: d } }\n"} {
		plain := renderShipped(t, "toolkit: 1\nagents: [claude-code]\n"+platform+"knowledge: { repo: x }\n")
		lines := renderShipped(t, "toolkit: 1\nagents: [claude-code]\n"+platform+"knowledge: { repo: x }\n"+releaseLines)
		checks := []struct{ target, text string }{
			{"skill/prompt-polish", "Release    <version> | none\nSpec       "},
			{"agents_md", "`build-fix`, `port`, `spec`. Say"},
			{"agents_md", "`audit` (read-only), `port` and `worktree` sit outside the chain."},
			{"skill/branching", "7. With a release, record it: `git config branch.<name>.dashcrew-release <version>`."},
			{"skill/prompt-polish", "- `Release`: the version the task ships in"},
			{"skill/pr-description", "keeps its plan in `.ai/plans/<release>/<task-slug>/`"},
			{"workflow/plan", "uses `.ai/plans/<release>/<task-slug>/plan.md`"},
			{"workflow/learn", "Pages describe the main line."},
		}
		for _, c := range checks {
			if strings.Contains(targetContent(t, plain, c.target), c.text) {
				t.Errorf("%s%s without release_lines holds %q", platform, c.target, c.text)
			}
			if !strings.Contains(targetContent(t, lines, c.target), c.text) {
				t.Errorf("%s%s with release_lines lacks %q", platform, c.target, c.text)
			}
		}
		if targetNames(plain)["workflow/port"] || !targetNames(lines)["workflow/port"] {
			t.Errorf("%sworkflow/port: plain %v, release lines %v", platform, targetNames(plain)["workflow/port"], targetNames(lines)["workflow/port"])
		}
		if port := targetContent(t, lines, "workflow/port"); !strings.Contains(port, "git merge --no-ff --no-commit -X find-renames") ||
			!strings.Contains(port, "The user commits, pushes and opens the PR.") {
			t.Errorf("port workflow:\n%s", port)
		}
	}
}

// TestLinksAddTextOnlyOutsideAgentsMd pins P9 M4: project.links adds the cross-repo text to the branching,
// pr-description and verify skills and the Dependencies heading to the PR template, and not one byte to AGENTS.md.
func TestLinksAddTextOnlyOutsideAgentsMd(t *testing.T) {
	const links = "project: { links: [{ package: shop_core }] }\n"
	for _, platform := range []string{"", "platform: other\nplatform_config: { detect: [go.mod], commands: { analyze: go vet, test: go test, test_scoped: go test, deps: go mod download }, link: go.work }\n"} {
		profile := "toolkit: 1\nagents: [claude-code]\n" + platform
		off, on := renderShipped(t, profile), renderShipped(t, profile+links)
		if targetContent(t, on, "agents_md") != targetContent(t, off, "agents_md") {
			t.Errorf("%q: project.links changes AGENTS.md", platform)
		}
		for _, c := range []struct{ target, text string }{
			{"skill/branching", "## Across repos\n"},
			{"skill/branching", "`dashcrew link add <package> <path>`"},
			{"skill/pr-description", "- `## Dependencies` - each dependency in `project.links`"},
			{"pr_template", "## Dependencies\n"},
			{"workflow/verify", "run `dashcrew check --links` before the commit command"},
		} {
			if !strings.Contains(targetContent(t, on, c.target), c.text) {
				t.Errorf("%q %s lacks %q", platform, c.target, c.text)
			}
			if strings.Contains(targetContent(t, off, c.target), c.text) {
				t.Errorf("%q %s has %q without project.links", platform, c.target, c.text)
			}
		}
	}
}

// TestTrackerStepsLiveInSkillsOnly pins P11 T0: with a tracker pack the branching and pr-create skills carry the
// tracker step, naming the pack's skill; without one they do not, and AGENTS.md never carries the step.
func TestTrackerStepsLiveInSkillsOnly(t *testing.T) {
	const base = "toolkit: 1\nagents: [claude-code]\n"
	off := renderShipped(t, base+"stack: { tracker: none }\n")
	on := renderShipped(t, base+"stack: { tracker: clickup }\n")
	for _, c := range []struct{ target, text string }{
		{"skill/branching", "set the task to its start status as the `tracker-clickup` skill says"},
		{"skill/pr-create", "set its review status and post the PR comment, both, as the `tracker-clickup` skill says"},
	} {
		if !strings.Contains(targetContent(t, on, c.target), c.text) {
			t.Errorf("%s lacks %q", c.target, c.text)
		}
		if strings.Contains(targetContent(t, off, c.target), "## Tracker task") {
			t.Errorf("%s has the tracker step without a tracker", c.target)
		}
	}
	if strings.Contains(targetContent(t, on, "agents_md"), "## Tracker task") {
		t.Error("AGENTS.md carries the tracker step")
	}
}

// TestGithubTrackerLabelsMode pins the labels mode of the GitHub tracker pack, with its params overridden.
func TestGithubTrackerLabelsMode(t *testing.T) {
	rendered := renderShipped(t, "toolkit: 1\nagents: [claude-code]\nstack:\n  tracker: { use: github, mode: labels, label_start: wip, pr_link: Closes }\n")
	agents, skill := targetContent(t, rendered, "agents_md"), targetContent(t, rendered, "skill/tracker-github")
	for _, want := range []string{"Commands (mode `labels`): skill `tracker-github`.", "body has `Closes #<n>`"} {
		if !strings.Contains(agents, want) {
			t.Errorf("AGENTS.md lacks %q", want)
		}
	}
	for _, want := range []string{"keeps issue status in mode `labels`", "`gh issue edit <n> --add-label \"wip\"`", "--remove-label \"wip\" --add-label \"status: in review\""} {
		if !strings.Contains(skill, want) {
			t.Errorf("tracker-github lacks %q", want)
		}
	}
}

// TestQuestionsAreAskedWhenTheyComeUp pins the question triggers added after the P7 live run: a spec asks its open
// questions before hand-over, a plan carries a Questions line that blocks GO, implementation stops on a
// contradiction, and the reviewer's open questions reach the user.
func TestQuestionsAreAskedWhenTheyComeUp(t *testing.T) {
	const knowledge = "knowledge: { repo: git@example.com:team/notes-knowledge.git }\n"
	for _, platform := range []string{"", "platform: other\nplatform_config: { detect: [x], commands: { analyze: a, test: t, test_scoped: s, deps: d } }\n"} {
		rendered := renderShipped(t, "toolkit: 1\nagents: [claude-code]\n"+platform+knowledge)
		for _, c := range []struct{ target, text string }{
			{"workflow/spec", "## 4. Questions"},
			{"workflow/spec", "Never hand over a spec with a question the user could have answered."},
			{"workflow/plan", "Questions: <each question only the user can settle, found while planning, or none>"},
			{"workflow/plan", "The `Questions:` line is not `none`"},
			{"workflow/tdd", "### When the code disagrees"},
			{"workflow/code-review", "and the open questions the reviewer raised, go to the user once, as one list"},
			{"agents_md", "Ask every question at step 0, together; one found later, when found."},
		} {
			if !strings.Contains(targetContent(t, rendered, c.target), c.text) {
				t.Errorf("%q %s lacks %q", platform, c.target, c.text)
			}
		}
	}
}

// TestChatBlocksShareOneLayout pins the chat block layout: AGENTS.md states it once with the comparison table rule
// and the SUMMARY fields, and the step 0, review and verify templates follow it outside AGENTS.md.
func TestChatBlocksShareOneLayout(t *testing.T) {
	const rule = "\n========================================================================\n"
	for _, platform := range []string{"", "platform: other\nplatform_config: { detect: [x], commands: { analyze: a, test: t, test_scoped: s, deps: d } }\n"} {
		rendered := renderShipped(t, "toolkit: 1\nagents: [claude-code]\n"+platform)
		for _, c := range []struct{ target, text string }{
			{"agents_md", "72-column `=` rule"},
			{"agents_md", "A comparison (options, before and after, trade-offs) is a table in that layout"},
			{"agents_md", "Above trivial, the summary is the `SUMMARY` block in `pr-description`:"},
			{"agents_md", "polish it with `prompt-polish` and print its block."},
			{"agents_md", "The main session always asks open questions, with the agent's question tool when it has one, and waits."},
			{"skill/prompt-polish", "With the agent's question tool, ask through it: each `Q` by its id, the branch and base"},
			{"skill/prompt-polish", "STEP 0  <feat | fix | hotfix>: <polished request, one sentence>" + rule},
			{"skill/prompt-polish", "-- Questions -----------------------------------------------------------\nQ1    <question>\n"},
			{"skill/prompt-polish", "Reply \"ok\", or answer by id (Q1 ...), pick a branch and a base, edit any line."},
			{"workflow/code-review", "REVIEW  <BLOCK | PASS WITH NOTES | PASS>  (<plan | n files>)" + rule},
			{"role/reviewer", "Return one `REVIEW` block, in the format of the `code-review` workflow's Verdict section"},
			{"workflow/verify", "VERIFY  <PASS | FAIL>  PR ready: <yes | no>" + rule},
			{"workflow/learn", "LEARN" + rule},
			{"workflow/build-fix", "BUILD FIX" + rule},
		} {
			if !strings.Contains(targetContent(t, rendered, c.target), c.text) {
				t.Errorf("%q %s lacks %q", platform, c.target, c.text)
			}
		}
		ui := "`unit`, `ui`, `integration` or `manual`"
		if platform == "" {
			ui = "`unit`, `widget`, `integration` or `manual`"
		}
		if !strings.Contains(targetContent(t, rendered, "skill/prompt-polish"), ui) {
			t.Errorf("%q: prompt-polish lacks the platform test types %q", platform, ui)
		}
		summary := targetContent(t, rendered, "skill/pr-description")
		if !strings.Contains(summary, "SUMMARY"+rule) || strings.Contains(summary, "- `Tracker ") {
			t.Errorf("%q: pr-description lacks the SUMMARY block or holds a tracker row without a tracker", platform)
		}
		tracked := targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\n"+platform+"stack: { tracker: clickup }\n"), "skill/pr-description")
		if !strings.Contains(tracked, "- `Tracker    <task>  <old status> -> <new status>, comment <posted | none>`") {
			t.Errorf("%q: pr-description lacks the tracker row with tracker: clickup", platform)
		}
		const extras = "knowledge: { repo: x }\nproject: { release_lines: { main: develop, release: 'release/(?P<version>[0-9.]+)' } }\n"
		agentsMd := targetContent(t, renderShipped(t, "toolkit: 1\nagents: [claude-code]\n"+platform+extras), "agents_md")
		for _, gone := range []string{"TASK:", "Success criteria: <id>", "Release:          ", "Spec:             "} {
			if strings.Contains(agentsMd, gone) {
				t.Errorf("%q agents_md still holds the step 0 template line %q", platform, gone)
			}
		}
	}
}

// TestEachChatBlockHasOneSource pins the single source of every chat block: a platform overrides the prose around
// a block, never the block, so a layout change is made in one file.
func TestEachChatBlockHasOneSource(t *testing.T) {
	c := shipped(t)
	fragments := append([]catalog.Fragment{}, c.Core...)
	for _, name := range slices.Sorted(maps.Keys(c.Platforms)) {
		fragments = append(fragments, c.Platforms[name].Fragments...)
	}
	for _, title := range []string{"STEP 0  <", "REVIEW  <", "VERIFY  <", "BUILD FIX\n=", "LEARN\n=", "SUMMARY\n="} {
		var sources []string
		for _, fragment := range fragments {
			if strings.Contains(fragment.Body, title) {
				sources = append(sources, fragment.Source)
			}
		}
		if len(sources) != 1 {
			t.Errorf("%q is written in %d sources, want 1: %v", title, len(sources), sources)
		}
	}
}
