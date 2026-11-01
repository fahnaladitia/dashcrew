package render

import (
	"errors"
	"reflect"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

type fragmentOption func(*catalog.Fragment)

func target(name string) fragmentOption { return func(f *catalog.Fragment) { f.Target = name } }
func order(n int) fragmentOption        { return func(f *catalog.Fragment) { f.Order = n } }
func rules(ids ...string) fragmentOption {
	return func(f *catalog.Fragment) { f.Rules = ids }
}
func when(pairs ...any) fragmentOption {
	return func(f *catalog.Fragment) { f.When = omap.Of[any](pairs...) }
}

func fragment(source, body string, options ...fragmentOption) catalog.Fragment {
	f := catalog.Fragment{Source: source, Target: "agents_md", Order: 100, Rules: []string{}, When: omap.New[any](), Body: body}
	for _, option := range options {
		option(&f)
	}
	return f
}

func newCatalog(core []catalog.Fragment, packs catalog.Packs) *catalog.Catalog {
	if packs == nil {
		packs = omap.New[*omap.Map[*catalog.Pack]]()
	}
	return &catalog.Catalog{Core: core, Packs: packs, Roles: map[string]catalog.RoleMeta{}, Skills: map[string]catalog.DocMeta{},
		Workflows: map[string]catalog.DocMeta{}, Platforms: map[string]*catalog.PlatformSpec{}}
}

func pack(family, option string, params *omap.Map[any], fragments ...catalog.Fragment) *catalog.Pack {
	if params == nil {
		params = omap.New[any]()
	}
	return &catalog.Pack{Family: family, Option: option, Params: params, Fragments: fragments, Skills: map[string]catalog.DocMeta{}}
}

func packsOf(packs ...*catalog.Pack) catalog.Packs {
	out := omap.New[*omap.Map[*catalog.Pack]]()
	for _, p := range packs {
		options, ok := out.Get(p.Family)
		if !ok {
			options = omap.New[*catalog.Pack]()
			out.Set(p.Family, options)
		}
		options.Set(p.Option, p)
	}
	return out
}

func parseProfile(t *testing.T, yaml string, c *catalog.Catalog) *profile.Profile {
	t.Helper()
	p, err := profile.Parse(yaml, c.KnownOptions())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func renderAll(t *testing.T, c *catalog.Catalog, yaml string) []RenderedTarget {
	t.Helper()
	targets, err := New(c).Render(parseProfile(t, yaml, c))
	if err != nil {
		t.Fatal(err)
	}
	return targets
}

func strPtr(s string) *string { return &s }

const minimal = "toolkit: 1\nagents: [claude-code]\n"

func TestCoreFragmentsRenderAndJoinInOrder(t *testing.T) {
	c := newCatalog([]catalog.Fragment{fragment("core/b.md", "Second", order(20)), fragment("core/a.md", "First", order(10))}, nil)

	targets := renderAll(t, c, minimal)

	if len(targets) != 1 || targets[0].Target != "agents_md" || targets[0].Content != "First\n\nSecond\n" {
		t.Errorf("targets %+v", targets)
	}
}

func TestSameOrderFallsBackToSourcePath(t *testing.T) {
	c := newCatalog([]catalog.Fragment{fragment("core/z.md", "Z"), fragment("core/a.md", "A")}, nil)

	if content := renderAll(t, c, minimal)[0].Content; content != "A\n\nZ\n" {
		t.Errorf("content %q", content)
	}
}

func TestOnlySelectedPacksRenderWithParamsMergedOverDefaults(t *testing.T) {
	c := newCatalog(nil, packsOf(
		pack("state", "bloc", omap.Of[any]("reference_folder", "lib/default"), fragment("packs/state/bloc/r.md", "Bloc ref {{pack.state.reference_folder}}")),
		pack("state", "riverpod", nil, fragment("packs/state/riverpod/r.md", "Riverpod")),
	))

	targets := renderAll(t, c, "toolkit: 1\nagents: [claude-code]\nstack: { state: { use: bloc, reference_folder: lib/ui/orders/blocs } }\n")

	if targets[0].Content != "Bloc ref lib/ui/orders/blocs\n" {
		t.Errorf("content %q", targets[0].Content)
	}
}

func TestWhenFiltersFragments(t *testing.T) {
	c := newCatalog([]catalog.Fragment{
		fragment("core/commit.md", "Never commit.", when("policy.agent_commit", false)),
		fragment("core/commit_ok.md", "Commits allowed.", when("policy.agent_commit", true)),
		fragment("core/lang.md", "Indonesian or Malay docs.", when("language.dartdoc", []any{"id", "ms"})),
	}, nil)

	targets := renderAll(t, c, "toolkit: 1\nagents: [claude-code]\nlanguage: { dartdoc: id }\n")

	if targets[0].Content != "Never commit.\n\nIndonesian or Malay docs.\n" {
		t.Errorf("content %q", targets[0].Content)
	}
}

func TestValuesAreSubstitutedWithoutHtmlEscaping(t *testing.T) {
	c := newCatalog([]catalog.Fragment{fragment("core/doc.md", "Max {{policy.dartdoc_max_lines}} lines in {{paths.plans}} <b>&</b>")}, nil)

	if content := renderAll(t, c, minimal)[0].Content; content != "Max 3 lines in .ai/plans <b>&</b>\n" {
		t.Errorf("content %q", content)
	}
}

func TestMissingVariableFailsWithTheFragmentSource(t *testing.T) {
	c := newCatalog([]catalog.Fragment{fragment("core/typo.md", "{{policy.dartdoc_max_line}}")}, nil)

	_, err := New(c).Render(parseProfile(t, minimal, c))

	var renderErr *Error
	if !errors.As(err, &renderErr) || renderErr.Source != "core/typo.md" {
		t.Fatalf("got %v", err)
	}
	if err.Error() != "core/typo.md: Value was missing for variable tag: policy.dartdoc_max_line." {
		t.Errorf("message %q", err.Error())
	}
}

func TestTargetsAreSeparatedAndRulesCollectedPerTarget(t *testing.T) {
	c := newCatalog([]catalog.Fragment{
		fragment("core/git.md", "Git", rules("git.no-agent-commit")),
		fragment("core/plan.md", "Plan step", target("workflow/plan"), rules("plan.verdict")),
		fragment("core/plan2.md", "Plan gate", target("workflow/plan"), rules("plan.wait", "plan.verdict")),
	}, nil)

	targets := renderAll(t, c, minimal)

	if len(targets) != 2 || targets[0].Target != "agents_md" || targets[1].Target != "workflow/plan" {
		t.Fatalf("targets %+v", targets)
	}
	if !reflect.DeepEqual(targets[0].Rules, []string{"git.no-agent-commit"}) || !reflect.DeepEqual(targets[1].Rules, []string{"plan.verdict", "plan.wait"}) {
		t.Errorf("rules %v %v", targets[0].Rules, targets[1].Rules)
	}
}

const indexBody = "{{#skill_index}}- {{name}}{{#review}} (review){{/review}}: {{when_to_use}}\n{{/skill_index}}"

func agentsMdOf(t *testing.T, targets []RenderedTarget) string {
	t.Helper()
	for _, target := range targets {
		if target.Target == "agents_md" {
			return target.Content
		}
	}
	t.Fatal("no agents_md")
	return ""
}

func TestSkillIndexListsEveryRenderedSkillSortedFromCoreAndPacks(t *testing.T) {
	bloc := pack("state", "bloc", nil, fragment("packs/state/bloc/s.md", "B", target("skill/bloc-patterns")))
	bloc.Skills = map[string]catalog.DocMeta{"bloc-patterns": {Description: "Bloc patterns.", WhenToUse: strPtr("On a Bloc.")}}
	c := newCatalog([]catalog.Fragment{
		fragment("core/rules/index.md", indexBody),
		fragment("core/skills/zeta/a.md", "Z", target("skill/zeta")),
		fragment("core/skills/zeta/b.md", "Z2", target("skill/zeta")),
		fragment("core/skills/alpha/a.md", "A", target("skill/alpha")),
		fragment("core/workflows/plan/a.md", "P", target("workflow/plan")),
	}, packsOf(bloc))
	c.Skills = map[string]catalog.DocMeta{
		"zeta":  {Description: "Zeta.", WhenToUse: strPtr("Last."), Review: true},
		"alpha": {Description: "Alpha skill."},
	}
	c.Workflows = map[string]catalog.DocMeta{"plan": {Description: "Plan."}}

	content := agentsMdOf(t, renderAll(t, c, "toolkit: 1\nagents: [claude-code]\nstack: { state: bloc }\n"))

	if content != "- alpha: Alpha skill.\n- bloc-patterns: On a Bloc.\n- zeta (review): Last.\n" {
		t.Errorf("content %q", content)
	}
}

func TestSkillIndexSkipsFilteredOutSkillsAndSkillsWithoutMeta(t *testing.T) {
	c := newCatalog([]catalog.Fragment{
		fragment("core/rules/index.md", indexBody),
		fragment("core/skills/gated/a.md", "G", target("skill/gated"), when("policy.formatter", true)),
		fragment("core/skills/orphan/a.md", "O", target("skill/orphan")),
		fragment("core/skills/kept/a.md", "K", target("skill/kept")),
	}, nil)
	c.Skills = map[string]catalog.DocMeta{"gated": {Description: "Gated."}, "kept": {Description: "Kept.", WhenToUse: strPtr("Always.")}}

	if content := agentsMdOf(t, renderAll(t, c, minimal)); content != "- kept: Always.\n" {
		t.Errorf("content %q", content)
	}
}

func withPlatform() *catalog.Catalog {
	c := newCatalog([]catalog.Fragment{
		fragment("core/rules/testing.md", "Run `{{platform.commands.test_scoped}}`{{#platform.commands.format}}, then `{{.}}`{{/platform.commands.format}}.", order(10)),
		fragment("core/rules/code.md", "Generic code.", order(20)),
	}, packsOf(pack("tracker", "none", nil, fragment("packs/tracker/none/a.md", "No tracker.", order(90)))))
	c.Platforms["flutter"] = &catalog.PlatformSpec{
		Name:       "flutter",
		Root:       "platforms/flutter",
		Detect:     []string{"pubspec.yaml"},
		Commands:   omap.Of[any]("analyze", "lint", "test", "test all", "test_scoped", "test one", "deps", "fetch", "format", "fmt"),
		McpServers: omap.New[catalog.McpServer](),
		Fragments: []catalog.Fragment{
			fragment("platforms/flutter/rules/code.md", "Platform code.", order(20)),
			fragment("platforms/flutter/rules/lifecycle.md", "Dispose.", order(15)),
		},
		Packs: packsOf(pack("state", "bloc", nil, fragment("platforms/flutter/packs/state/bloc/a.md", "Bloc.", order(80)))),
	}
	return c
}

func TestSelectedPlatformAddsReplacesAndFillsPlatform(t *testing.T) {
	c := withPlatform()

	targets := renderAll(t, c, "toolkit: 1\nagents: [claude-code]\nstack: { state: bloc, tracker: none }\n")

	if targets[0].Content != "Run `test one`, then `fmt`.\n\nDispose.\n\nPlatform code.\n\nBloc.\n\nNo tracker.\n" {
		t.Errorf("content %q", targets[0].Content)
	}
}

func TestOtherRendersCoreOnlyWithPlatformConfigCommands(t *testing.T) {
	c := withPlatform()
	yaml := `
toolkit: 1
agents: [claude-code]
platform: other
platform_config: { detect: [Makefile], commands: { analyze: make lint, test: make test, test_scoped: make one, deps: make deps } }
stack: { tracker: none }
`

	targets := renderAll(t, c, yaml)

	if targets[0].Content != "Run `make one`.\n\nGeneric code.\n\nNo tracker.\n" {
		t.Errorf("content %q", targets[0].Content)
	}
	if name := New(c).ContextFor(parseProfile(t, yaml, c))["platform"].(map[string]any)["name"]; name != "other" {
		t.Errorf("platform.name %v", name)
	}
}
