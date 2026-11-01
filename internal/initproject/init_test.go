package initproject

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

func shippedCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type initRun struct {
	yes, force     bool
	answers        []string
	remoteBranches []string
	catalog        *catalog.Catalog
}

type initResult struct {
	profile string
	output  []string
}

func newInitProject(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	write(t, project, "pubspec.yaml", `
name: app
dependencies:
  flutter_bloc: ^9.0.0
  get_it: ^8.0.0
  go_router: ^14.0.0
  dio: ^5.0.0
  http: ^1.0.0
  core: { path: packages/core }
dev_dependencies:
  mocktail: ^1.0.0
`)
	write(t, project, "lib/main_dev.dart", "")
	return project
}

// runInit answers each prompt from the list, then nil (end of input), as the Dart test does.
func runInit(t *testing.T, project string, r initRun) (initResult, error) {
	t.Helper()
	c := r.catalog
	if c == nil {
		c = shippedCatalog(t)
	}
	output := []string{}
	pending := append([]string{}, r.answers...)
	err := Init{Catalog: c, ProjectDir: project}.Run(r.yes, r.force, r.remoteBranches, func(prompt string) *string {
		output = append(output, prompt)
		if len(pending) == 0 {
			return nil
		}
		answer := pending[0]
		pending = pending[1:]
		return &answer
	}, func(line string) { output = append(output, line) })
	text, _ := os.ReadFile(filepath.Join(project, ".ai", "project.yaml"))
	return initResult{string(text), output}, err
}

func mustInit(t *testing.T, project string, r initRun) initResult {
	t.Helper()
	result, err := runInit(t, project, r)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func has(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func count(list []string, value string) int {
	n := 0
	for _, item := range list {
		if item == value {
			n++
		}
	}
	return n
}

func families(t *testing.T, c *catalog.Catalog, platform string) []string {
	t.Helper()
	keys := c.KnownOptions()[platform].Keys()
	sort.Strings(keys)
	return keys
}

func initErrorContaining(t *testing.T, err error, parts ...string) {
	t.Helper()
	var initErr *Error
	if !errors.As(err, &initErr) {
		t.Fatalf("expected an init error, got %v", err)
	}
	for _, part := range parts {
		if !strings.Contains(initErr.Message, part) {
			t.Errorf("message %q lacks %q", initErr.Message, part)
		}
	}
}

func TestYesWritesTheDetectionAndSkipsAFamilyWithTwoCandidates(t *testing.T) {
	project := newInitProject(t)
	result := mustInit(t, project, initRun{yes: true, remoteBranches: []string{"origin/main", "origin/develop"}})

	want := `# Written by dashcrew init. Edit it, then run dashcrew sync.
toolkit: 1
platform: flutter
agents: [claude-code]
stack:
  di: get_it
  router: go_router
  state: bloc
  test: mocktail
project:
  flavors:
    dev: lib/main_dev.dart
  local_packages: [packages/core]
  base_branches: [main, develop]
`
	if result.profile != want {
		t.Errorf("profile:\n%s", result.profile)
	}
	if !strings.Contains(strings.Join(result.output, "\n"), "network: skipped, 2 candidates: dio, http") {
		t.Errorf("output %v", result.output)
	}
	if last := result.output[len(result.output)-1]; last != "next: dashcrew sync" {
		t.Errorf("last line %q", last)
	}
	c := shippedCatalog(t)
	if _, err := profile.Parse(result.profile, c.KnownOptions()); err != nil {
		t.Error(err)
	}
}

func TestInteractiveAnswersKeepChangePickAndDropFamilies(t *testing.T) {
	project := newInitProject(t)
	answers := []string{""}
	for _, family := range families(t, shippedCatalog(t), "flutter") {
		answer := map[string]string{"network": "dio", "router": "getx", "test": "-", "tracker": "other linear_cli"}[family]
		answers = append(answers, answer)
	}

	result := mustInit(t, project, initRun{answers: answers})

	for _, part := range []string{"  network: dio\n", "  router: getx\n", "  tracker: { use: other, name: linear_cli }\n",
		"  state: bloc\n", "platform: flutter\n"} {
		if !strings.Contains(result.profile, part) {
			t.Errorf("profile lacks %q", part)
		}
	}
	if strings.Contains(result.profile, "  test:") {
		t.Error("test was dropped but is in the profile")
	}
	for _, line := range []string{"platform [flutter]: ", "state [bloc]: ", "network [dio | http]: "} {
		if !has(result.output, line) {
			t.Errorf("output lacks %q", line)
		}
	}
	if !strings.Contains(strings.Join(result.output, "\n"), "state: bloc (flutter_bloc in dependencies)") {
		t.Error("output lacks the state evidence")
	}
}

func TestAnInvalidAnswerAsksAgainWithTheValidOptions(t *testing.T) {
	project := newInitProject(t)
	answers := []string{""}
	for _, family := range families(t, shippedCatalog(t), "flutter") {
		if family == "state" {
			answers = append(answers, "mobx", "other")
		} else {
			answers = append(answers, "")
		}
	}

	result := mustInit(t, project, initRun{answers: answers})

	if n := count(result.output, "state [bloc]: "); n != 3 {
		t.Errorf("state asked %d times", n)
	}
	if !strings.Contains(strings.Join(result.output, "\n"), "expected one of") {
		t.Error("no expected options line")
	}
	if !strings.Contains(result.profile, "  state: bloc\n") {
		t.Errorf("profile:\n%s", result.profile)
	}
}

func TestDetectedParamsAreWrittenInTheMapForm(t *testing.T) {
	project := newInitProject(t)
	write(t, project, "pubspec.yaml", "name: app\ndependencies:\n  get_it: ^9.0.0\n  injectable: ^3.0.0\n")

	if result := mustInit(t, project, initRun{yes: true}); !strings.Contains(result.profile, "  di: { use: get_it, injectable: true }\n") {
		t.Errorf("profile:\n%s", result.profile)
	}
}

func TestRefusesAnExistingProfileWithoutForce(t *testing.T) {
	project := newInitProject(t)
	write(t, project, ".ai/project.yaml", "toolkit: 1\n")

	result, err := runInit(t, project, initRun{yes: true})
	initErrorContaining(t, err, "--force")
	if result.profile != "toolkit: 1\n" {
		t.Errorf("profile changed: %q", result.profile)
	}
	if forced := mustInit(t, project, initRun{yes: true, force: true}); !strings.HasPrefix(forced.profile, "# Written by dashcrew init.") {
		t.Errorf("forced profile %q", forced.profile)
	}
}

func TestAProjectPackFamilyIsOfferedAndAnOptionMissingFromTheCatalogBecomesOther(t *testing.T) {
	project := newInitProject(t)
	write(t, project, ".ai/packs/architecture/clean/pack.yaml", "description: clean\n")
	write(t, project, "pubspec.yaml", "name: app\ndependencies:\n  get: ^4.0.0\n")
	c := shippedCatalog(t)
	flutter := *c.Platforms["flutter"]
	packs := omap.New[*omap.Map[*catalog.Pack]]()
	for _, family := range flutter.Packs.Keys() {
		packs.Set(family, flutter.Packs.Value(family))
	}
	packs.Set("router", omap.New[*catalog.Pack]())
	flutter.Packs = packs
	withoutGetx := &catalog.Catalog{Core: c.Core, Packs: c.Packs, Roles: c.Roles, Skills: c.Skills, Workflows: c.Workflows,
		Platforms: map[string]*catalog.PlatformSpec{"flutter": &flutter}}

	result := mustInit(t, project, initRun{catalog: withoutGetx})

	if !has(result.output, "architecture [-]: ") || !has(result.output, "router [other get]: ") {
		t.Errorf("output %v", result.output)
	}
	if !strings.Contains(result.profile, "  router: { use: other, name: get }\n") {
		t.Errorf("profile:\n%s", result.profile)
	}
}

const goPlatform = "detect: [go.mod]\ncommands: { analyze: go vet ./..., test: go test ./..., test_scoped: go test, deps: go mod download }\n"

func TestARepoMatchingNoPlatformIsNeverGivenFlutterYesRefusesAndNamesOther(t *testing.T) {
	project := newInitProject(t)
	os.Remove(filepath.Join(project, "pubspec.yaml"))

	_, err := runInit(t, project, initRun{yes: true})
	initErrorContaining(t, err, "platform: other")
	if _, statErr := os.Stat(filepath.Join(project, ".ai", "project.yaml")); statErr == nil {
		t.Error("a profile was written")
	}
}

func TestARepoMatchingNoPlatformIsOfferedOtherAndTheCommandsAreAsked(t *testing.T) {
	project := newInitProject(t)
	os.Remove(filepath.Join(project, "pubspec.yaml"))
	write(t, project, "Makefile", "lint:\n")

	result := mustInit(t, project, initRun{answers: []string{"", "Makefile", "make lint", "make test", "", "make test-one", "make deps", ""}})

	if !has(result.output, "platform [other]: ") || !has(result.output, "  required") {
		t.Errorf("output %v", result.output)
	}
	want := `# Written by dashcrew init. Edit it, then run dashcrew sync.
toolkit: 1
platform: other
platform_config:
  detect: [Makefile]
  commands:
    analyze: "make lint"
    test: "make test"
    test_scoped: "make test-one"
    deps: "make deps"
agents: [claude-code]
`
	if result.profile != want {
		t.Errorf("profile:\n%s", result.profile)
	}
	parsed, err := profile.Parse(result.profile, shippedCatalog(t).KnownOptions())
	if err != nil {
		t.Fatal(err)
	}
	if format, ok := parsed.PlatformConfig.Commands.Get("format"); !ok || format != nil {
		t.Errorf("format %v, %v", format, ok)
	}
}

func TestOtherWithoutTheCommandsStopsInsteadOfWritingABrokenProfile(t *testing.T) {
	project := newInitProject(t)
	os.Remove(filepath.Join(project, "pubspec.yaml"))

	_, err := runInit(t, project, initRun{answers: []string{"", "Makefile"}})
	initErrorContaining(t, err, "commands.analyze")
}

func TestAProjectPlatformIsDetectedAndOffersItsOwnFamilies(t *testing.T) {
	project := newInitProject(t)
	os.Remove(filepath.Join(project, "pubspec.yaml"))
	write(t, project, "go.mod", "module example.com/app\n")
	write(t, project, ".ai/platforms/go/platform.yaml", goPlatform)
	write(t, project, ".ai/platforms/go/packs/router/chi/pack.yaml", "description: chi\n")
	withProject, err := shippedCatalog(t).WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}
	answers := []string{""}
	for _, family := range families(t, withProject, "go") {
		if family == "router" {
			answers = append(answers, "chi")
		} else {
			answers = append(answers, "")
		}
	}

	result := mustInit(t, project, initRun{answers: answers})

	if !strings.HasPrefix(result.output[0], "Detected in") {
		t.Errorf("first line %q", result.output[0])
	}
	for _, line := range []string{"  platform: go (go.mod)", "platform [go]: ", "router [-]: "} {
		if !has(result.output, line) {
			t.Errorf("output lacks %q", line)
		}
	}
	if has(result.output, "state [-]: ") {
		t.Error("state is a flutter family")
	}
	for _, part := range []string{"platform: go\n", "  router: chi\n"} {
		if !strings.Contains(result.profile, part) {
			t.Errorf("profile lacks %q", part)
		}
	}
	if strings.Contains(result.profile, "lib/main_dev.dart") {
		t.Error("flavors detected for go")
	}
}

func TestTwoMatchingPlatformsAreAskedAndYesRefuses(t *testing.T) {
	project := newInitProject(t)
	write(t, project, "go.mod", "module example.com/app\n")
	write(t, project, ".ai/platforms/go/platform.yaml", goPlatform)

	_, err := runInit(t, project, initRun{yes: true})
	initErrorContaining(t, err, "flutter", "go")
	answers := []string{"", "flutter"}
	for range families(t, shippedCatalog(t), "flutter") {
		answers = append(answers, "")
	}

	result := mustInit(t, project, initRun{answers: answers})

	if n := count(result.output, "platform [flutter | go]: "); n != 2 {
		t.Errorf("platform asked %d times", n)
	}
	if !strings.Contains(result.profile, "platform: flutter\n") {
		t.Errorf("profile:\n%s", result.profile)
	}
}
