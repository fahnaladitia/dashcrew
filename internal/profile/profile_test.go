package profile

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

func known() KnownOptions {
	families := func(pairs ...any) *omap.Map[[]string] { return omap.Of[[]string](pairs...) }
	return KnownOptions{
		"flutter": families("state", []string{"bloc", "riverpod"}, "di", []string{"get_it", "none"}, "analytics", []string{"posthog", "none"}),
		"go":      families("tracker", []string{"none"}),
		"other":   families("tracker", []string{"none"}),
	}
}

func parse(t *testing.T, yaml string) *Profile {
	t.Helper()
	profile, err := Parse(yaml, known())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return profile
}

func parseError(t *testing.T, yaml string) *Error {
	t.Helper()
	_, err := Parse(yaml, known())
	var profileErr *Error
	if !errors.As(err, &profileErr) {
		t.Fatalf("expected a profile error, got %v", err)
	}
	return profileErr
}

func plain(value any) any {
	switch v := value.(type) {
	case *omap.Map[any]:
		out := map[string]any{}
		for _, key := range v.Keys() {
			out[key] = plain(v.Value(key))
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[key] = plain(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = plain(item)
		}
		return out
	case []map[string]any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = plain(item)
		}
		return out
	}
	return value
}

func TestMinimalProfileGetsDefaults(t *testing.T) {
	profile := parse(t, "toolkit: 1\nagents: [claude-code]\n")

	if !reflect.DeepEqual(profile.Agents, []string{"claude-code"}) {
		t.Errorf("agents %v", profile.Agents)
	}
	if profile.Language.Value("dartdoc") != "en" {
		t.Errorf("language.dartdoc %v", profile.Language.Value("dartdoc"))
	}
	if profile.Policy.Value("agent_commit") != false {
		t.Error("policy.agent_commit")
	}
	if profile.Policy.Value("dartdoc_max_lines") != 3 {
		t.Error("policy.dartdoc_max_lines")
	}
	if profile.Policy.Has("plan_gate") {
		t.Error("policy.plan_gate present")
	}
	if len(profile.Warnings) != 0 {
		t.Errorf("warnings %v", profile.Warnings)
	}
	if profile.Paths.Value("plans") != ".ai/plans" {
		t.Error("paths.plans")
	}
	if profile.Stack.Len() != 0 {
		t.Error("stack not empty")
	}
}

func TestPlanGateIsAcceptedIgnoredAndWarned(t *testing.T) {
	profile := parse(t, "toolkit: 1\nagents: [claude-code]\npolicy: { plan_gate: true }\n")

	if profile.Policy.Has("plan_gate") {
		t.Error("policy keeps plan_gate")
	}
	if profile.ToContext()["policy"].(*omap.Map[any]).Has("plan_gate") {
		t.Error("context keeps plan_gate")
	}
	if !reflect.DeepEqual(profile.Warnings, []string{"plan_gate was removed in P4; delete it from .ai/project.yaml"}) {
		t.Errorf("warnings %v", profile.Warnings)
	}
}

func TestStackAcceptsShorthandAndMapForm(t *testing.T) {
	profile := parse(t, `
toolkit: 1
agents: [claude-code]
stack:
  state: bloc
  analytics: { use: posthog, facade: Analytics.instance }
`)

	state := profile.Stack.Value("state")
	if state.Option != "bloc" || state.Params.Len() != 0 {
		t.Errorf("state %+v", state)
	}
	analytics := profile.Stack.Value("analytics")
	if analytics.Option != "posthog" || !reflect.DeepEqual(plain(analytics.Params), map[string]any{"facade": "Analytics.instance"}) {
		t.Errorf("analytics %+v", analytics)
	}
}

func TestOverridesReplaceDefaultsKeyByKey(t *testing.T) {
	profile := parse(t, `
toolkit: 1
agents: [claude-code]
language: { dartdoc: id }
paths: { plans: docs/plans }
`)

	if profile.Language.Value("dartdoc") != "id" || profile.Language.Value("pr") != "en" {
		t.Errorf("language %v", plain(profile.Language))
	}
	if profile.Paths.Value("plans") != "docs/plans" || profile.Paths.Value("audits") != ".ai/audits" {
		t.Errorf("paths %v", plain(profile.Paths))
	}
}

func TestErrorPaths(t *testing.T) {
	cases := []struct {
		name, yaml, path string
	}{
		{"unknown top-level key", "toolkit: 1\nagents: [claude-code]\nstak: {}\n", "stak"},
		{"unknown stack option", "toolkit: 1\nagents: [claude-code]\nstack: { state: mobx }\n", "stack.state"},
		{"other without a name (shorthand)", "toolkit: 1\nagents: [claude-code]\nstack: { state: other }\n", "stack.state.name"},
		{"other without a name (map)", "toolkit: 1\nagents: [claude-code]\nstack: { state: { use: other } }\n", "stack.state.name"},
		{"other with an empty name", "toolkit: 1\nagents: [claude-code]\nstack: { state: { use: other, name: '' } }\n", "stack.state.name"},
		{"other with a list name", "toolkit: 1\nagents: [claude-code]\nstack: { state: { use: other, name: [a] } }\n", "stack.state.name"},
		{"unknown stack family", "toolkit: 1\nagents: [claude-code]\nstack: { orm: drift }\n", "stack.orm"},
		{"missing toolkit version", "agents: [claude-code]\n", "toolkit"},
		{"unknown agent", "toolkit: 1\nagents: [cursor]\n", "agents"},
		{"flavors as a list", "toolkit: 1\nagents: [claude-code]\nproject: { flavors: [dev] }\n", "project.flavors"},
		{"flavor entrypoint as a list", "toolkit: 1\nagents: [claude-code]\nproject: { flavors: { dev: [a] } }\n", "project.flavors.dev"},
		{"empty flavor entrypoint", "toolkit: 1\nagents: [claude-code]\nproject: { flavors: { dev: '' } }\n", "project.flavors.dev"},
		{"github without a ref", "toolkit: 1\nagents: [claude-code]\nci: { github: true }\n", "ci.dashcrew_ref"},
		{"unknown ci key", "toolkit: 1\nagents: [claude-code]\nci: { gitlab: true }\n", "ci.gitlab"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseError(t, c.yaml).Path; got != c.path {
				t.Errorf("path %q, want %q", got, c.path)
			}
		})
	}
}

func TestErrorMessagesMatchTheDartEngine(t *testing.T) {
	cases := []struct {
		yaml, message string
	}{
		{"- a\n", "project.yaml : expected a YAML map"},
		{"toolkit: 1\nagents: [claude-code]\nstak: {}\n", "project.yaml stak: unknown key"},
		{"toolkit: 2\nagents: [claude-code]\n", "project.yaml toolkit: required, supported version is 1"},
		{"toolkit: 1\nagents: [claude-code]\nplatform: rust\n", `project.yaml platform: unknown platform "rust", expected one of flutter, go, other`},
		{"toolkit: 1\nagents: []\n", "project.yaml agents: required, a non-empty list"},
		{"toolkit: 1\nagents: [cursor]\n", `project.yaml agents: unsupported agent "cursor", expected one of claude-code, codex, antigravity`},
		{"toolkit: 1\nagents: [claude-code]\nstack: { state: mobx }\n", "project.yaml stack.state: expected one of bloc, riverpod, other"},
		{"toolkit: 1\nagents: [claude-code]\nstack: { orm: drift }\n", "project.yaml stack.orm: unknown stack family"},
		{"toolkit: 1\nagents: [claude-code]\nstack: [a]\n", "project.yaml stack: expected a map"},
		{"toolkit: 1\nagents: [claude-code]\nstack: { state: other }\n", "project.yaml stack.state.name: required with use: other, the library package name"},
		{"toolkit: 1\nagents: [claude-code]\nlanguage: { code: en }\n", "project.yaml language.code: unknown key"},
		{"toolkit: 1\nagents: [claude-code]\npolicy: [a]\n", "project.yaml policy: expected a map"},
		{"toolkit: 1\nagents: [claude-code]\nproject: { flavors: [dev] }\n", "project.yaml project.flavors: expected a map of flavor name to entrypoint"},
		{"toolkit: 1\nagents: [claude-code]\nproject: { flavors: { dev: '' } }\n", "project.yaml project.flavors.dev: expected the entrypoint path, e.g. lib/main_dev.dart"},
		{"toolkit: 1\nagents: [claude-code]\nci: { github: true }\n", "project.yaml ci.dashcrew_ref: required with github: true, the dashcrew commit or tag CI installs"},
		{"toolkit: 1\nagents: [claude-code]\nplatform: other\n", "project.yaml platform_config: required with platform: other, a map of detect and commands"},
		{"toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: [a]\n", "project.yaml platform_config: expected a map of detect and commands"},
		{"toolkit: 1\nagents: [claude-code]\nplatform_config: {}\n", "project.yaml platform_config: allowed only with platform: other"},
		{"toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { detect: [] }\n", "project.yaml platform_config.detect: required, a non-empty list of files that identify the platform"},
		{"toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { detect: [a] }\n", "project.yaml platform_config.commands: required, a map with analyze, test, test_scoped, deps"},
		{"toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { detect: [a], commands: { lint: x } }\n", "project.yaml platform_config.commands.lint: unknown command, expected one of analyze, test, test_scoped, deps, format"},
		{"toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { detect: [a], commands: { analyze: a } }\n", "project.yaml platform_config.commands.test: required, a shell command"},
		{"toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { detect: [a], commands: { analyze: a, test: b, test_scoped: c, deps: d, format: '' } }\n", "project.yaml platform_config.commands.format: optional, a shell command"},
	}
	for _, c := range cases {
		_, err := Parse(c.yaml, known())
		if err == nil || err.Error() != c.message {
			t.Errorf("%q:\n got %v\nwant %s", c.yaml, err, c.message)
		}
	}
}

func TestUnknownOptionErrorListsOther(t *testing.T) {
	if message := parseError(t, "toolkit: 1\nagents: [claude-code]\nstack: { state: mobx }\n").Message; !strings.Contains(message, "other") {
		t.Errorf("message %q", message)
	}
}

func TestOtherIsValidForEveryFamilyAndNeedsAName(t *testing.T) {
	profile := parse(t, `
toolkit: 1
agents: [claude-code]
stack:
  state: { use: other, name: mobx }
  di: { use: other, name: injectable }
`)

	state := profile.Stack.Value("state")
	if state.Option != "other" || !reflect.DeepEqual(plain(state.Params), map[string]any{"name": "mobx"}) {
		t.Errorf("state %+v", state)
	}
	context := profile.ToContext()
	want := []any{
		map[string]any{"family": "di", "name": "injectable"},
		map[string]any{"family": "state", "name": "mobx"},
	}
	if !reflect.DeepEqual(plain(context["stack_other"]), want) {
		t.Errorf("stack_other %v", context["stack_other"])
	}
	if context["has_stack_other"] != true {
		t.Error("has_stack_other")
	}
}

func TestNoOtherChoiceGivesAnEmptyStackOther(t *testing.T) {
	context := parse(t, "toolkit: 1\nagents: [claude-code]\nstack: { state: bloc }\n").ToContext()

	if len(context["stack_other"].([]any)) != 0 || context["has_stack_other"] != false {
		t.Errorf("stack_other %v, has_stack_other %v", context["stack_other"], context["has_stack_other"])
	}
}

func TestFlavorsBecomeABuildListSortedByName(t *testing.T) {
	context := parse(t, `
toolkit: 1
agents: [claude-code]
project: { flavors: { staging: lib/main_staging.dart, dev: lib/main_dev.dart } }
`).ToContext()

	build := context["build"].(map[string]any)
	want := []any{
		map[string]any{"name": "dev", "entrypoint": "lib/main_dev.dart"},
		map[string]any{"name": "staging", "entrypoint": "lib/main_staging.dart"},
	}
	if !reflect.DeepEqual(plain(build["flavors"]), want) || build["has_flavors"] != true {
		t.Errorf("build %v", build)
	}
}

func TestNoFlavorsGivesAnEmptyBuildList(t *testing.T) {
	build := parse(t, "toolkit: 1\nagents: [claude-code]\n").ToContext()["build"].(map[string]any)

	if len(build["flavors"].([]any)) != 0 || build["has_flavors"] != false {
		t.Errorf("build %v", build)
	}
}

func TestCIDefaultsToOffAndGithubNeedsARef(t *testing.T) {
	profile := parse(t, "toolkit: 1\nagents: [claude-code]\n")
	if !reflect.DeepEqual(plain(profile.CI), map[string]any{"github": false, "dashcrew_ref": nil}) {
		t.Errorf("ci %v", plain(profile.CI))
	}
	if ref := parse(t, "toolkit: 1\nagents: [claude-code]\nci: { github: true, dashcrew_ref: 0123abc }\n").CI.Value("dashcrew_ref"); ref != "0123abc" {
		t.Errorf("dashcrew_ref %#v", ref)
	}
}

func TestToContextExposesNestedMaps(t *testing.T) {
	context := parse(t, `
toolkit: 1
agents: [claude-code]
stack: { analytics: { use: posthog, facade: Analytics.instance } }
`).ToContext()

	if context["policy"].(*omap.Map[any]).Value("formatter") != false {
		t.Error("policy.formatter")
	}
	if context["stack"].(map[string]any)["analytics"].(map[string]any)["use"] != "posthog" {
		t.Error("stack.analytics.use")
	}
}

const platformCommands = "commands: { analyze: make lint, test: make test, test_scoped: make test-one, deps: make deps }"

func TestPlatformMissingReadsAsFlutter(t *testing.T) {
	profile := parse(t, "toolkit: 1\nagents: [claude-code]\nstack: { state: bloc }\n")

	if profile.Platform != "flutter" || profile.PlatformConfig != nil {
		t.Errorf("platform %q, config %+v", profile.Platform, profile.PlatformConfig)
	}
}

func TestProjectPlatformValidatesTheStackAgainstItsOwnFamilies(t *testing.T) {
	if profile := parse(t, "toolkit: 1\nagents: [claude-code]\nplatform: go\nstack: { tracker: none }\n"); profile.Platform != "go" {
		t.Errorf("platform %q", profile.Platform)
	}
	if path := parseError(t, "toolkit: 1\nagents: [claude-code]\nplatform: go\nstack: { state: bloc }\n").Path; path != "stack.state" {
		t.Errorf("path %q", path)
	}
}

func TestUnknownPlatformListsTheKnownOnes(t *testing.T) {
	err := parseError(t, "toolkit: 1\nagents: [claude-code]\nplatform: rust\n")

	if err.Path != "platform" {
		t.Errorf("path %q", err.Path)
	}
	for _, name := range []string{"flutter", "go", "other"} {
		if !strings.Contains(err.Message, name) {
			t.Errorf("message %q lacks %s", err.Message, name)
		}
	}
}

func TestOtherReadsDetectAndCommandsFromPlatformConfig(t *testing.T) {
	profile := parse(t, "toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config:\n  detect: [Makefile]\n  "+platformCommands+"\n")

	if profile.Platform != "other" {
		t.Errorf("platform %q", profile.Platform)
	}
	if !reflect.DeepEqual(profile.PlatformConfig.Detect, []string{"Makefile"}) {
		t.Errorf("detect %v", profile.PlatformConfig.Detect)
	}
	commands := profile.PlatformConfig.Commands
	if !reflect.DeepEqual(commands.Keys(), []string{"analyze", "test", "test_scoped", "deps", "format"}) {
		t.Errorf("commands keys %v", commands.Keys())
	}
	want := map[string]any{"analyze": "make lint", "test": "make test", "test_scoped": "make test-one", "deps": "make deps", "format": nil}
	if !reflect.DeepEqual(plain(commands), want) {
		t.Errorf("commands %v", plain(commands))
	}
}

func TestPlatformConfigErrors(t *testing.T) {
	if path := parseError(t, "toolkit: 1\nagents: [claude-code]\nplatform: other\n").Path; path != "platform_config" {
		t.Errorf("missing platform_config: %q", path)
	}
	err := parseError(t, "toolkit: 1\nagents: [claude-code]\nplatform_config: { detect: [Makefile], "+platformCommands+" }\n")
	if err.Path != "platform_config" || !strings.Contains(err.Message, "platform: other") {
		t.Errorf("platform_config without other: %+v", err)
	}

	broken := []struct{ config, path string }{
		{"detect: [], " + platformCommands, "platform_config.detect"},
		{"detect: Makefile, " + platformCommands, "platform_config.detect"},
		{"detect: [Makefile]", "platform_config.commands"},
		{"detect: [Makefile], commands: { analyze: make lint, test: make test, deps: make deps }", "platform_config.commands.test_scoped"},
		{"detect: [Makefile], commands: { analyze: '', test: a, test_scoped: b, deps: c }", "platform_config.commands.analyze"},
		{"detect: [Makefile], commands: { analyze: a, test: b, test_scoped: c, deps: d, lint: e }", "platform_config.commands.lint"},
		{"detect: [Makefile], " + platformCommands + ", link: x", "platform_config.link"},
	}
	for _, c := range broken {
		t.Run(c.path, func(t *testing.T) {
			if got := parseError(t, "toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { "+c.config+" }\n").Path; got != c.path {
				t.Errorf("path %q, want %q", got, c.path)
			}
		})
	}
}
