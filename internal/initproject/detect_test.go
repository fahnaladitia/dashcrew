package initproject

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

func write(t *testing.T, root, relative, content string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func detectWith(t *testing.T, dependencies, devDependencies map[string]string) *Detection {
	t.Helper()
	project := t.TempDir()
	lines := func(m map[string]string) string {
		keys := make([]string, 0, len(m))
		for key := range m {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := []string{}
		for _, key := range keys {
			out = append(out, "  "+key+": "+m[key])
		}
		return strings.Join(out, "\n")
	}
	write(t, project, "pubspec.yaml", "name: app\ndependencies:\n  flutter: { sdk: flutter }\n"+lines(dependencies)+
		"\ndev_dependencies:\n"+lines(devDependencies)+"\n")
	detection, err := Detect(project, nil, "flutter")
	if err != nil {
		t.Fatal(err)
	}
	return detection
}

func labels(candidates []*StackCandidate) []string {
	out := []string{}
	for _, c := range candidates {
		out = append(out, c.Label())
	}
	return out
}

func TestEachPackageDetectsItsFamily(t *testing.T) {
	rows := []struct{ pkg, expected string }{
		{"flutter_bloc", "state: bloc"},
		{"bloc", "state: bloc"},
		{"get_it", "di: get_it"},
		{"get", "router: getx"},
		{"go_router", "router: go_router"},
		{"auto_route", "router: auto_route"},
		{"dio", "network: dio"},
		{"hive", "storage: hive"},
		{"hive_ce", "storage: hive"},
		{"hive_flutter", "storage: hive"},
		{"posthog_flutter", "analytics: posthog"},
		{"mobx", "state: other mobx"},
		{"http", "network: http"},
		{"retrofit", "network: retrofit"},
		{"chopper", "network: other chopper"},
		{"isar", "storage: other isar"},
		{"drift", "storage: other drift"},
		{"sqflite", "storage: sqflite"},
		{"shared_preferences", "storage: shared_preferences"},
		{"firebase_analytics", "analytics: firebase"},
		{"mixpanel_flutter", "analytics: mixpanel"},
	}
	if len(rows) != 21 {
		t.Fatalf("%d rows, the Dart test has 21", len(rows))
	}
	for _, row := range rows {
		t.Run(row.pkg+" detects "+row.expected, func(t *testing.T) {
			detection := detectWith(t, map[string]string{row.pkg: "^1.0.0"}, nil)
			family, _, _ := strings.Cut(row.expected, ":")

			if keys := detection.Stack.Keys(); !reflect.DeepEqual(keys, []string{family}) {
				t.Fatalf("families %v", keys)
			}
			candidates := detection.Stack.Value(family)
			if len(candidates) != 1 || family+": "+candidates[0].Label() != row.expected {
				t.Fatalf("candidates %v", labels(candidates))
			}
			if !reflect.DeepEqual(candidates[0].Evidence, []string{row.pkg + " in dependencies"}) {
				t.Errorf("evidence %v", candidates[0].Evidence)
			}
		})
	}
}

func TestMockLibrariesInDevDependenciesDetectTest(t *testing.T) {
	for _, pkg := range []string{"mocktail", "mockito"} {
		t.Run(pkg, func(t *testing.T) {
			detection := detectWith(t, nil, map[string]string{pkg: "^1.0.0"})

			candidates := detection.Stack.Value("test")
			if len(candidates) != 1 || candidates[0].Label() != pkg {
				t.Fatalf("test candidates %v", labels(candidates))
			}
			if !reflect.DeepEqual(candidates[0].Evidence, []string{pkg + " in dev_dependencies"}) {
				t.Errorf("evidence %v", candidates[0].Evidence)
			}
		})
	}
}

func TestRiverpodAndProviderFillStateAndDi(t *testing.T) {
	for _, row := range [][2]string{{"flutter_riverpod", "riverpod"}, {"riverpod", "riverpod"}, {"hooks_riverpod", "riverpod"},
		{"provider", "provider"}} {
		t.Run(row[0], func(t *testing.T) {
			detection := detectWith(t, map[string]string{row[0]: "^1.0.0"}, nil)

			for _, family := range []string{"state", "di"} {
				if got := labels(detection.Stack.Value(family)); !reflect.DeepEqual(got, []string{row[1]}) {
					t.Errorf("%s: %v", family, got)
				}
			}
		})
	}
}

func TestInjectableSelectsGetItWithTheInjectableParam(t *testing.T) {
	detection := detectWith(t, map[string]string{"get_it": "^9.0.0", "injectable": "^3.0.0"}, nil)

	candidates := detection.Stack.Value("di")
	if len(candidates) != 1 {
		t.Fatalf("candidates %v", labels(candidates))
	}
	c := candidates[0]
	if c.Option != "get_it" {
		t.Errorf("option %s", c.Option)
	}
	if !reflect.DeepEqual(c.Params.Keys(), []string{"injectable"}) || c.Params.Value("injectable") != true {
		t.Errorf("params %v", c.Params.Keys())
	}
	if !reflect.DeepEqual(c.Evidence, []string{"get_it in dependencies", "injectable in dependencies"}) {
		t.Errorf("evidence %v", c.Evidence)
	}
}

func TestTwoPackagesOfOneLibraryGiveOneCandidateWithBothAsEvidence(t *testing.T) {
	detection := detectWith(t, map[string]string{"flutter_bloc": "^9.0.0", "bloc": "^9.0.0"}, nil)

	candidates := detection.Stack.Value("state")
	if len(candidates) != 1 || candidates[0].Option != "bloc" {
		t.Fatalf("candidates %v", labels(candidates))
	}
	if !reflect.DeepEqual(candidates[0].Evidence, []string{"bloc in dependencies", "flutter_bloc in dependencies"}) {
		t.Errorf("evidence %v", candidates[0].Evidence)
	}
}

func TestTwoLibrariesForOneFamilyGiveTwoCandidates(t *testing.T) {
	detection := detectWith(t, map[string]string{"dio": "^5.0.0", "http": "^1.0.0"}, nil)

	if got := labels(detection.Stack.Value("network")); !reflect.DeepEqual(got, []string{"dio", "http"}) {
		t.Errorf("network %v", got)
	}
}

func TestUnknownPackagesAreIgnored(t *testing.T) {
	if detection := detectWith(t, map[string]string{"intl": "any", "equatable": "any"}, nil); detection.Stack.Len() != 0 {
		t.Errorf("stack %v", detection.Stack.Keys())
	}
}

func TestPathDependenciesInsideTheProjectBecomeLocalPackages(t *testing.T) {
	project := t.TempDir()
	write(t, project, "pubspec.yaml", `
name: app
dependencies:
  core: { path: packages/core/ }
  shared:
    path: ./shared
  outside: { path: ../outside }
`)

	detection, err := Detect(project, nil, "flutter")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(detection.LocalPackages, []string{"packages/core", "shared"}) {
		t.Errorf("local packages %v", detection.LocalPackages)
	}
}

func TestMainFlavorFilesBecomeFlavors(t *testing.T) {
	project := t.TempDir()
	write(t, project, "pubspec.yaml", "name: app\n")
	write(t, project, "lib/main.dart", "")
	write(t, project, "lib/main_dev.dart", "")
	write(t, project, "lib/main_prod.dart", "")

	detection, err := Detect(project, nil, "flutter")
	if err != nil {
		t.Fatal(err)
	}
	want := omap.Of[string]("dev", "lib/main_dev.dart", "prod", "lib/main_prod.dart")
	if !reflect.DeepEqual(detection.Flavors.Keys(), want.Keys()) || detection.Flavors.Value("dev") != "lib/main_dev.dart" ||
		detection.Flavors.Value("prod") != "lib/main_prod.dart" {
		t.Errorf("flavors %v", detection.Flavors.Keys())
	}
}

func TestKnownRemoteBranchesBecomeBaseBranchesInAFixedOrder(t *testing.T) {
	project := t.TempDir()
	write(t, project, "pubspec.yaml", "name: app\n")

	detection, err := Detect(project, []string{"origin/feature/x", "origin/develop", "origin/main", "origin/HEAD"}, "flutter")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(detection.BaseBranches, []string{"main", "develop"}) {
		t.Errorf("base branches %v", detection.BaseBranches)
	}
}

func TestMissingPubspecFails(t *testing.T) {
	var initErr *Error
	if _, err := Detect(t.TempDir(), nil, "flutter"); !errors.As(err, &initErr) {
		t.Errorf("error %v", err)
	}
}

func TestPlatformsMatchOnAnyOfTheirDetectFiles(t *testing.T) {
	project := t.TempDir()
	commands := omap.Of[any]("analyze", "a", "test", "b", "test_scoped", "c", "deps", "d", "format", nil)
	platforms := map[string]*catalog.PlatformSpec{
		"flutter": {Name: "flutter", Detect: []string{"pubspec.yaml"}, Commands: commands},
		"go":      {Name: "go", Detect: []string{"go.mod", "go.work"}, Commands: commands},
	}

	if got := DetectPlatforms(project, platforms); len(got) != 0 {
		t.Errorf("empty project: %v", got)
	}
	write(t, project, "go.work", "")
	if got := DetectPlatforms(project, platforms); !reflect.DeepEqual(got, []string{"go"}) {
		t.Errorf("go.work: %v", got)
	}
	write(t, project, "pubspec.yaml", "name: app\n")
	if got := DetectPlatforms(project, platforms); !reflect.DeepEqual(got, []string{"flutter", "go"}) {
		t.Errorf("both: %v", got)
	}
}

func TestAPlatformOtherThanTheDefaultReadsNoPubspec(t *testing.T) {
	project := t.TempDir()
	write(t, project, "pubspec.yaml", "name: app\ndependencies:\n  dio: ^5.0.0\n")
	write(t, project, "lib/main_dev.dart", "")

	detection, err := Detect(project, []string{"origin/main"}, "go")
	if err != nil {
		t.Fatal(err)
	}
	if detection.Stack.Len() != 0 || detection.Flavors.Len() != 0 {
		t.Errorf("stack %v, flavors %v", detection.Stack.Keys(), detection.Flavors.Keys())
	}
	if !reflect.DeepEqual(detection.BaseBranches, []string{"main"}) {
		t.Errorf("base branches %v", detection.BaseBranches)
	}
}
