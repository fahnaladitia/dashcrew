package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

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

func load(t *testing.T, root string) *Catalog {
	t.Helper()
	catalog, err := Load(os.DirFS(root))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return catalog
}

func catalogError(t *testing.T, err error) *Error {
	t.Helper()
	var catalogErr *Error
	if !errors.As(err, &catalogErr) {
		t.Fatalf("expected a catalog error, got %v", err)
	}
	return catalogErr
}

// knownSets flattens KnownOptions for comparison: platform -> family -> sorted options.
func knownSets(known profile.KnownOptions) map[string]map[string][]string {
	out := map[string]map[string][]string{}
	for platform, families := range known {
		out[platform] = map[string][]string{}
		for _, family := range families.Keys() {
			options := append([]string{}, families.Value(family)...)
			sort.Strings(options)
			out[platform][family] = options
		}
	}
	return out
}

func sources(fragments []Fragment) []string {
	out := []string{}
	for _, fragment := range fragments {
		out = append(out, fragment.Source)
	}
	return out
}

func TestLoadsCoreFragmentsWithFrontMatter(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core/git/commits.md", "---\ntarget: agents_md\norder: 20\nrules: [git.no-agent-commit]\nwhen: { policy.agent_commit: false }\n---\nNever commit.\n")

	catalog := load(t, root)

	if len(catalog.Core) != 1 {
		t.Fatalf("core %d", len(catalog.Core))
	}
	fragment := catalog.Core[0]
	if fragment.Target != "agents_md" || fragment.Order != 20 || !reflect.DeepEqual(fragment.Rules, []string{"git.no-agent-commit"}) {
		t.Errorf("fragment %+v", fragment)
	}
	if fragment.When.Len() != 1 || fragment.When.Value("policy.agent_commit") != false {
		t.Errorf("when %v", fragment.When)
	}
	if strings.TrimSpace(fragment.Body) != "Never commit." || fragment.Source != "core/git/commits.md" {
		t.Errorf("body %q source %q", fragment.Body, fragment.Source)
	}
}

func TestOrderDefaultsTo100AndRulesToEmpty(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core/a.md", "---\ntarget: agents_md\n---\nA\n")

	fragment := load(t, root).Core[0]

	if fragment.Order != 100 || len(fragment.Rules) != 0 || fragment.When.Len() != 0 {
		t.Errorf("fragment %+v", fragment)
	}
}

func TestFragmentErrors(t *testing.T) {
	cases := []struct{ content, message string }{
		{"---\norder: 1\n---\nx\n", "core/broken.md: front matter needs a target"},
		{"no front matter\n", "core/broken.md: missing front matter"},
		{"---\ntarget: a\n", "core/broken.md: unterminated front matter"},
		{"---\n- a\n---\nx\n", "core/broken.md: front matter must be a map"},
		{"---\ntarget: a\norder: 1.5\n---\nx\n", "core/broken.md: order must be an integer"},
		{"---\ntarget: a\nrules: x\n---\nx\n", "core/broken.md: rules must be a list"},
		{"---\ntarget: a\nwhen: [x]\n---\nx\n", "core/broken.md: when must be a map"},
	}
	for _, c := range cases {
		root := t.TempDir()
		write(t, root, "core/broken.md", c.content)
		_, err := Load(os.DirFS(root))
		catalogErr := catalogError(t, err)
		if catalogErr.Source != "core/broken.md" || catalogErr.Error() != c.message {
			t.Errorf("%q: got %v, want %s", c.content, err, c.message)
		}
	}
}

func TestCRLFFrontMatterIsNormalized(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core/a.md", "---\r\ntarget: agents_md\r\n---\r\nA\r\n")

	if body := load(t, root).Core[0].Body; body != "A\n" {
		t.Errorf("body %q", body)
	}
}

func TestLoadsPacksWithParamsAndFragments(t *testing.T) {
	root := t.TempDir()
	write(t, root, "packs/state/bloc/pack.yaml", "description: flutter_bloc state management\nparams:\n  reference_folder: lib/ui/example/blocs\n")
	write(t, root, "packs/state/bloc/rules.md", "---\ntarget: agents_md\n---\nUse Bloc.\n")
	write(t, root, "packs/state/riverpod/pack.yaml", "description: riverpod\n")

	catalog := load(t, root)

	bloc := catalog.Packs.Value("state").Value("bloc")
	if bloc.Family != "state" || bloc.Option != "bloc" || bloc.Params.Value("reference_folder") != "lib/ui/example/blocs" {
		t.Errorf("bloc %+v", bloc)
	}
	if len(bloc.Fragments) != 1 || strings.TrimSpace(bloc.Fragments[0].Body) != "Use Bloc." {
		t.Errorf("bloc fragments %+v", bloc.Fragments)
	}
	if len(catalog.Packs.Value("state").Value("riverpod").Fragments) != 0 {
		t.Error("riverpod fragments")
	}
	if got := knownSets(catalog.KnownOptions())["flutter"]; !reflect.DeepEqual(got, map[string][]string{"state": {"bloc", "riverpod"}}) {
		t.Errorf("known %v", got)
	}
}

func TestSkillMetaReadsReviewFalseWhenAbsent(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core/skills/security/meta.yaml", "description: Security checklist.\nwhen_to_use: On auth changes.\nreview: true\n")
	write(t, root, "core/skills/branching/meta.yaml", "description: Branch setup.\n")

	catalog := load(t, root)

	if !catalog.Skills["security"].Review || catalog.Skills["branching"].Review {
		t.Errorf("skills %+v", catalog.Skills)
	}
	if when := catalog.Skills["security"].WhenToUse; when == nil || *when != "On auth changes." {
		t.Errorf("when_to_use %v", when)
	}
}

func TestSkillMetaWithNonBooleanReviewFails(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core/skills/security/meta.yaml", "description: Security checklist.\nreview: yes please\n")

	_, err := Load(os.DirFS(root))

	if err == nil || err.Error() != "core/skills/security/meta.yaml: review: expected true or false" {
		t.Errorf("got %v", err)
	}
}

func TestPackFolderWithoutPackYamlFails(t *testing.T) {
	root := t.TempDir()
	write(t, root, "packs/state/bloc/rules.md", "---\ntarget: agents_md\n---\nx\n")

	_, err := Load(os.DirFS(root))

	if err == nil || err.Error() != "packs/state/bloc/pack.yaml: missing pack.yaml" {
		t.Errorf("got %v", err)
	}
}

func projectPacksRoot(t *testing.T) (string, string) {
	root, project := t.TempDir(), t.TempDir()
	write(t, root, "packs/state/bloc/pack.yaml", "description: bloc\n")
	write(t, root, "packs/state/bloc/rules.md", "---\ntarget: agents_md\n---\nUse Bloc.\n")
	write(t, root, "packs/state/riverpod/pack.yaml", "description: riverpod\n")
	return root, project
}

func TestProjectPackReplacesABuiltInOptionAndAddsAFamily(t *testing.T) {
	root, project := projectPacksRoot(t)
	write(t, project, ".ai/packs/state/bloc/pack.yaml", "description: team bloc\nparams: { dir: lib/blocs }\n")
	write(t, project, ".ai/packs/state/bloc/rules.md", "---\ntarget: agents_md\n---\nTeam Bloc.\n")
	write(t, project, ".ai/packs/architecture/clean/pack.yaml", "description: clean\n")
	write(t, project, ".ai/packs/architecture/clean/rules.md", "---\ntarget: agents_md\n---\nClean layers.\n")
	builtin := load(t, root)

	catalog, err := builtin.WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}

	bloc := catalog.Packs.Value("state").Value("bloc")
	if !bloc.FromProject || bloc.Params.Value("dir") != "lib/blocs" {
		t.Errorf("bloc %+v", bloc)
	}
	if strings.TrimSpace(bloc.Fragments[0].Body) != "Team Bloc." || bloc.Fragments[0].Source != ".ai/packs/state/bloc/rules.md" {
		t.Errorf("bloc fragment %+v", bloc.Fragments[0])
	}
	if catalog.Packs.Value("state").Value("riverpod").FromProject {
		t.Error("riverpod from project")
	}
	want := map[string][]string{"state": {"bloc", "riverpod"}, "architecture": {"clean"}}
	if got := knownSets(catalog.KnownOptions())["flutter"]; !reflect.DeepEqual(got, want) {
		t.Errorf("known %v", got)
	}
	if builtin.Packs.Value("state").Value("bloc").FromProject || !reflect.DeepEqual(builtin.Packs.Keys(), []string{"state"}) {
		t.Error("built-in catalog changed")
	}
}

func TestNoProjectPacksKeepsTheBuiltInPacks(t *testing.T) {
	root, project := projectPacksRoot(t)

	catalog, err := load(t, root).WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}

	if got := knownSets(catalog.KnownOptions())["flutter"]; !reflect.DeepEqual(got, map[string][]string{"state": {"bloc", "riverpod"}}) {
		t.Errorf("known %v", got)
	}
}

func TestBrokenProjectPackFailsWithItsProjectPath(t *testing.T) {
	root, project := projectPacksRoot(t)
	write(t, project, ".ai/packs/state/bloc/rules.md", "---\ntarget: agents_md\n---\nx\n")

	_, err := load(t, root).WithProjectPacks(project)

	if source := catalogError(t, err).Source; source != ".ai/packs/state/bloc/pack.yaml" {
		t.Errorf("source %q", source)
	}
}

func TestOptionNamedOtherIsReserved(t *testing.T) {
	root, project := projectPacksRoot(t)
	write(t, project, ".ai/packs/state/other/pack.yaml", "description: x\n")

	_, err := load(t, root).WithProjectPacks(project)

	if err == nil || err.Error() != `.ai/packs/state/other/pack.yaml: option name "other" is reserved` {
		t.Errorf("got %v", err)
	}
}

const flutterYaml = `description: Flutter apps
detect: [pubspec.yaml]
commands: { analyze: flutter analyze, test: flutter test, test_scoped: flutter test --reporter expanded, deps: flutter pub get, format: dart format }
link: pubspec_overrides.yaml
doc_comment: Dartdoc
mcp_servers:
  lang-server: { command: lang, args: [mcp] }
`

func platformsRoot(t *testing.T) (string, string) {
	root, project := t.TempDir(), t.TempDir()
	write(t, root, "core/rules/code.md", "---\ntarget: agents_md\norder: 50\n---\nGeneric code.\n")
	write(t, root, "core/rules/testing.md", "---\ntarget: agents_md\norder: 80\n---\nGeneric testing.\n")
	write(t, root, "core/skills/docs/meta.yaml", "description: generic docs\n")
	write(t, root, "packs/tracker/none/pack.yaml", "description: no tracker\n")
	write(t, root, "platforms/flutter/platform.yaml", flutterYaml)
	write(t, root, "platforms/flutter/rules/lifecycle.md", "---\ntarget: agents_md\norder: 71\n---\nDispose controllers.\n")
	write(t, root, "platforms/flutter/rules/testing.md", "---\ntarget: agents_md\norder: 80\n---\nRun flutter test.\n")
	write(t, root, "platforms/flutter/skills/docs/meta.yaml", "description: Dartdoc details\n")
	write(t, root, "platforms/flutter/skills/perf/meta.yaml", "description: widget performance\n")
	write(t, root, "platforms/flutter/skills/perf/perf.md", "---\ntarget: skill/perf\n---\nUse const.\n")
	write(t, root, "platforms/flutter/packs/state/bloc/pack.yaml", "description: bloc\n")
	return root, project
}

func TestLoadsABuiltInPlatform(t *testing.T) {
	root, _ := platformsRoot(t)

	platform := load(t, root).Platforms["flutter"]

	if platform.Name != "flutter" || *platform.Description != "Flutter apps" || !reflect.DeepEqual(platform.Detect, []string{"pubspec.yaml"}) {
		t.Errorf("platform %+v", platform)
	}
	wantCommands := omap.Of[any]("analyze", "flutter analyze", "test", "flutter test", "test_scoped", "flutter test --reporter expanded",
		"deps", "flutter pub get", "format", "dart format")
	if !reflect.DeepEqual(platform.Commands, wantCommands) {
		t.Errorf("commands %v", platform.Commands)
	}
	if *platform.Link != "pubspec_overrides.yaml" || *platform.DocComment != "Dartdoc" {
		t.Errorf("link %v doc_comment %v", platform.Link, platform.DocComment)
	}
	if !reflect.DeepEqual(platform.McpServers.Keys(), []string{"lang-server"}) {
		t.Errorf("mcp %v", platform.McpServers.Keys())
	}
	if server := platform.McpServers.Value("lang-server"); server.Command != "lang" || !reflect.DeepEqual(server.Args, []string{"mcp"}) {
		t.Errorf("server %+v", server)
	}
	if platform.FromProject {
		t.Error("from project")
	}
	wantSources := []string{"platforms/flutter/rules/lifecycle.md", "platforms/flutter/rules/testing.md", "platforms/flutter/skills/perf/perf.md"}
	if got := sources(platform.Fragments); !reflect.DeepEqual(got, wantSources) {
		t.Errorf("fragments %v", got)
	}
	if len(platform.Skills) != 2 || platform.Skills["docs"].Description != "Dartdoc details" {
		t.Errorf("skills %v", platform.Skills)
	}
	if platform.Packs.Value("state").Value("bloc").Family != "state" {
		t.Error("platform pack")
	}
}

func TestPlatformPacksResolveBeforeSharedPacks(t *testing.T) {
	root, _ := platformsRoot(t)
	catalog := load(t, root)

	if !reflect.DeepEqual(catalog.Packs.Keys(), []string{"tracker"}) {
		t.Errorf("packs %v", catalog.Packs.Keys())
	}
	want := map[string]map[string][]string{
		"flutter": {"state": {"bloc"}, "tracker": {"none"}},
		"other":   {"tracker": {"none"}},
	}
	if got := knownSets(catalog.KnownOptions()); !reflect.DeepEqual(got, want) {
		t.Errorf("known %v", got)
	}
	if catalog.PacksFor("flutter").Value("state").Value("bloc").FromProject {
		t.Error("bloc from project")
	}
	if got := catalog.PacksFor("other").Keys(); !reflect.DeepEqual(got, []string{"tracker"}) {
		t.Errorf("other packs %v", got)
	}
}

func TestProjectPackReplacesThePlatformPack(t *testing.T) {
	root, project := platformsRoot(t)
	write(t, project, ".ai/packs/state/bloc/pack.yaml", "description: team bloc\n")

	catalog, err := load(t, root).WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}

	if !catalog.PacksFor("flutter").Value("state").Value("bloc").FromProject {
		t.Error("bloc not from project")
	}
}

func TestPlatformFragmentReplacesCoreFragmentAndMetasOverrideCore(t *testing.T) {
	root, _ := platformsRoot(t)
	catalog := load(t, root)

	want := []string{"core/rules/code.md", "platforms/flutter/rules/lifecycle.md", "platforms/flutter/rules/testing.md", "platforms/flutter/skills/perf/perf.md"}
	if got := sources(catalog.FragmentsFor("flutter")); !reflect.DeepEqual(got, want) {
		t.Errorf("flutter %v", got)
	}
	if got := sources(catalog.FragmentsFor("other")); !reflect.DeepEqual(got, []string{"core/rules/code.md", "core/rules/testing.md"}) {
		t.Errorf("other %v", got)
	}
	if catalog.SkillsFor("flutter")["docs"].Description != "Dartdoc details" || catalog.SkillsFor("other")["docs"].Description != "generic docs" {
		t.Error("skill descriptions")
	}
	if _, ok := catalog.SkillsFor("other")["perf"]; ok {
		t.Error("other has perf")
	}
}

func TestProjectPlatformLoadsFromDotAi(t *testing.T) {
	root, project := platformsRoot(t)
	write(t, project, ".ai/platforms/go/platform.yaml", "detect: [go.mod]\ncommands: { analyze: go vet ./..., test: go test ./..., test_scoped: go test, deps: go mod download }\n")
	write(t, project, ".ai/platforms/go/rules/errors.md", "---\ntarget: agents_md\norder: 71\n---\nWrap errors.\n")
	write(t, project, ".ai/platforms/go/packs/router/chi/pack.yaml", "description: chi\n")

	catalog, err := load(t, root).WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}
	gp := catalog.Platforms["go"]

	if !gp.FromProject || gp.Commands.Value("format") != nil || gp.DocComment != nil || gp.McpServers.Len() != 0 {
		t.Errorf("go %+v", gp)
	}
	if got := sources(gp.Fragments); !reflect.DeepEqual(got, []string{".ai/platforms/go/rules/errors.md"}) {
		t.Errorf("fragments %v", got)
	}
	if got := knownSets(catalog.KnownOptions())["go"]; !reflect.DeepEqual(got, map[string][]string{"router": {"chi"}, "tracker": {"none"}}) {
		t.Errorf("known %v", got)
	}
	var bodies []string
	for _, fragment := range catalog.FragmentsFor("go") {
		bodies = append(bodies, strings.TrimSpace(fragment.Body))
	}
	if !reflect.DeepEqual(bodies, []string{"Generic code.", "Generic testing.", "Wrap errors."}) {
		t.Errorf("bodies %v", bodies)
	}
}

func TestProjectPlatformWithABuiltInNameReplacesItWhole(t *testing.T) {
	root, project := platformsRoot(t)
	write(t, project, ".ai/platforms/flutter/platform.yaml", strings.Replace(flutterYaml, "Flutter apps", "Team Flutter", 1))

	catalog, err := load(t, root).WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}

	if *catalog.Platforms["flutter"].Description != "Team Flutter" || len(catalog.Platforms["flutter"].Fragments) != 0 {
		t.Errorf("flutter %+v", catalog.Platforms["flutter"])
	}
	if got := knownSets(catalog.KnownOptions())["flutter"]; !reflect.DeepEqual(got, map[string][]string{"tracker": {"none"}}) {
		t.Errorf("known %v", got)
	}
}

func TestInvalidPlatformYamlFailsWithItsPath(t *testing.T) {
	cases := []struct{ yaml, message string }{
		{"detect: [go.mod]\n", "commands: required, a map with analyze, test, test_scoped, deps"},
		{"commands: { analyze: a, test: b, test_scoped: c, deps: d }\n", "detect: required, a non-empty list of files that identify the platform"},
		{"detect: [go.mod]\ncommands: { analyze: a, test: b, deps: d }\n", "commands.test_scoped: required, a shell command"},
		{"detect: [go.mod]\ncommands: { analyze: a, test: b, test_scoped: c, deps: d }\nlinks: x\n", "unknown key links"},
		{"detect: [go.mod]\ncommands: { analyze: a, test: b, test_scoped: c, deps: d }\nmcp_servers: { x: { args: [a] } }\n", "mcp_servers.x: expected { command: <string>, args: [<string>, ...] }"},
		{"detect: [go.mod]\ncommands: { analyze: a, test: b, test_scoped: c, deps: d }\nlink: [x]\n", "link: expected a string"},
		{"detect: [go.mod]\ncommands: { analyze: a, test: b, test_scoped: c, deps: d }\nmcp_servers: [x]\n", "mcp_servers: expected a map of name to command and args"},
		{"- a\n", "expected a YAML map"},
	}
	for _, c := range cases {
		t.Run(c.message, func(t *testing.T) {
			root, project := platformsRoot(t)
			write(t, project, ".ai/platforms/go/platform.yaml", c.yaml)

			_, err := load(t, root).WithProjectPacks(project)

			catalogErr := catalogError(t, err)
			if catalogErr.Source != ".ai/platforms/go/platform.yaml" || catalogErr.Message != c.message {
				t.Errorf("got %v", err)
			}
		})
	}
}

func TestPlatformFolderWithoutPlatformYamlFails(t *testing.T) {
	root, _ := platformsRoot(t)
	write(t, root, "platforms/go/rules/a.md", "---\ntarget: agents_md\n---\nx\n")

	_, err := Load(os.DirFS(root))

	if err == nil || err.Error() != "platforms/go/platform.yaml: missing platform.yaml" {
		t.Errorf("got %v", err)
	}
}

func TestPlatformNamedOtherIsReserved(t *testing.T) {
	root, project := platformsRoot(t)
	write(t, project, ".ai/platforms/other/platform.yaml", "detect: [x]\ncommands: { analyze: a, test: b, test_scoped: c, deps: d }\n")

	_, err := load(t, root).WithProjectPacks(project)

	if err == nil || err.Error() != `.ai/platforms/other/platform.yaml: platform name "other" is reserved` {
		t.Errorf("got %v", err)
	}
}

func TestPlatformForBuildsOtherFromPlatformConfig(t *testing.T) {
	root, _ := platformsRoot(t)
	catalog := load(t, root)
	p, err := profile.Parse(`
toolkit: 1
agents: [claude-code]
platform: other
platform_config: { detect: [Makefile], commands: { analyze: make lint, test: make test, test_scoped: make test-one, deps: make deps } }
`, catalog.KnownOptions())
	if err != nil {
		t.Fatal(err)
	}

	platform := catalog.PlatformFor(p)

	if platform.Name != "other" || !reflect.DeepEqual(platform.Detect, []string{"Makefile"}) || len(platform.Fragments) != 0 {
		t.Errorf("platform %+v", platform)
	}
	context := platform.ToContext()
	wantCommands := omap.Of[any]("analyze", "make lint", "test", "make test", "test_scoped", "make test-one", "deps", "make deps", "format", nil)
	want := map[string]any{
		"name": "other", "description": nil, "detect": []any{"Makefile"}, "commands": wantCommands, "link": nil, "doc_comment": nil,
	}
	if !reflect.DeepEqual(context, want) {
		t.Errorf("context %#v", context)
	}
}

func TestRoleYaml(t *testing.T) {
	root := t.TempDir()
	write(t, root, "core/roles/reviewer/role.yaml", "description: Reviews.\ncapability: read\nmodel: high\neffort: high\nwhen: { stack.design.use: [figma] }\n")
	write(t, root, "core/roles/broken/role.yaml", "description: x\ncapability: admin\n")

	_, err := Load(os.DirFS(root))
	if err == nil || err.Error() != "core/roles/broken/role.yaml: capability must be one of read, write-plans, write-code" {
		t.Errorf("got %v", err)
	}

	os.RemoveAll(filepath.Join(root, "core", "roles", "broken"))
	role := load(t, root).Roles["reviewer"]
	if role.Name != "reviewer" || role.Description != "Reviews." || role.Capability != "read" || *role.Model != "high" || *role.Effort != "high" {
		t.Errorf("role %+v", role)
	}
	if role.When.Len() != 1 {
		t.Errorf("when %v", role.When)
	}
}

func TestShippedCatalogLoads(t *testing.T) {
	catalog, err := LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Core) == 0 || catalog.Platforms["flutter"] == nil || len(catalog.Roles) != 2 {
		t.Errorf("core %d, platforms %v, roles %d", len(catalog.Core), catalog.Platforms, len(catalog.Roles))
	}
}
