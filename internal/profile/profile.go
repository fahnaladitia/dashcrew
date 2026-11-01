// Package profile parses `.ai/project.yaml` with the same strict keys, defaults and error messages as the Dart engine.
package profile

import (
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Error is a problem in `.ai/project.yaml`, with the YAML path that caused it.
type Error struct {
	Path    string
	Message string
}

func (e *Error) Error() string { return "project.yaml " + e.Path + ": " + e.Message }

func fail(path, message string) *Error { return &Error{Path: path, Message: message} }

// StackChoice is one `stack.<family>` choice: the pack option and its params.
type StackChoice struct {
	Option string
	Params *omap.Map[any]
}

// PlatformConfig is the `platform_config` of a `platform: other` profile.
type PlatformConfig struct {
	Detect []string
	// Commands holds PlatformCommands then OptionalPlatformCommands, in that order; a missing format is nil.
	Commands *omap.Map[any]
}

// KnownOptions maps every valid platform name, `other` included, to its stack families and their options in order.
type KnownOptions map[string]*omap.Map[[]string]

const (
	// PlanGateRemoved warns about a profile that still sets `policy.plan_gate`, which is read and ignored for one release.
	PlanGateRemoved = "plan_gate was removed in P4; delete it from .ai/project.yaml"
	// OtherOption is valid for every family (a library with no pack) and names the inline platform.
	OtherOption = "other"
	// DefaultPlatform is the platform of a profile without a `platform` key.
	DefaultPlatform = "flutter"
)

var (
	// SupportedAgents in the order error messages list them.
	SupportedAgents = []string{"claude-code", "codex", "antigravity"}
	// PlatformCommands a platform must declare.
	PlatformCommands = []string{"analyze", "test", "test_scoped", "deps"}
	// OptionalPlatformCommands a platform may leave out.
	OptionalPlatformCommands = []string{"format"}

	topLevelKeys = []string{"toolkit", "platform", "platform_config", "agents", "language", "stack", "project", "policy", "paths", "ci"}
)

func ciDefaults() *omap.Map[any] { return omap.Of[any]("github", false, "dashcrew_ref", nil) }

func languageDefaults() *omap.Map[any] {
	return omap.Of[any]("dartdoc", "en", "pr", "en", "chat", "en")
}

func policyDefaults() *omap.Map[any] {
	return omap.Of[any]("agent_commit", false, "formatter", false, "dartdoc_max_lines", 3, "tier_gates", true, "terse_prompts", false)
}

// PathDefaults are the `paths.*` of a profile that leaves them out.
func PathDefaults() *omap.Map[any] {
	return omap.Of[any]("plans", ".ai/plans", "audits", ".ai/audits", "pr", ".ai/pr", "memory", ".ai/memory")
}

func projectDefaults() *omap.Map[any] {
	return omap.Of[any](
		"flavors", omap.New[any](),
		"local_packages", []any{},
		"base_branches", []any{"main"},
		"branch_prefix", omap.Of[any]("feat", "feat/", "fix", "fix/", "hotfix", "hotfix/"),
		"build_wrapper", nil,
	)
}

// Profile is a parsed `.ai/project.yaml` with defaults applied.
type Profile struct {
	// Platform is a catalog platform name or OtherOption.
	Platform string
	// PlatformConfig is set only for OtherOption.
	PlatformConfig *PlatformConfig
	Agents         []string
	Language       *omap.Map[any]
	Stack          *omap.Map[StackChoice]
	Project        *omap.Map[any]
	Policy         *omap.Map[any]
	Paths          *omap.Map[any]
	// CI with `github: true` makes sync write a workflow running `dashcrew check` at the pinned `dashcrew_ref`.
	CI *omap.Map[any]
	// Warnings are keys the profile still accepts but ignores.
	Warnings []string
}

// PlatformCommandsFrom validates a platform `commands` map; every required command is a non-empty string and
// a missing `format` is nil. fail receives the offending key (`commands` or `commands.<name>`) and the message.
func PlatformCommandsFrom(value any, fail func(key, message string) error) (*omap.Map[any], error) {
	all := append(append([]string{}, PlatformCommands...), OptionalPlatformCommands...)
	commands, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fail("commands", "required, a map with "+strings.Join(PlatformCommands, ", "))
	}
	for _, key := range commands.Keys() {
		if !contains(all, key) {
			return nil, fail("commands."+key, "unknown command, expected one of "+strings.Join(all, ", "))
		}
	}
	out := omap.New[any]()
	for _, key := range all {
		command := commands.Value(key)
		optional := contains(OptionalPlatformCommands, key)
		text, isString := command.(string)
		if (command == nil && !optional) || (command != nil && (!isString || text == "")) {
			kind := "required"
			if optional {
				kind = "optional"
			}
			return nil, fail("commands."+key, kind+", a shell command")
		}
		out.Set(key, command)
	}
	return out, nil
}

// Parse reads a profile; known lists every valid platform with its families and options.
func Parse(text string, known KnownOptions) (*Profile, error) {
	parsed, err := yamlutil.Parse(text)
	if err != nil {
		return nil, err
	}
	root, ok := parsed.(*omap.Map[any])
	if !ok {
		return nil, fail("", "expected a YAML map")
	}
	for _, key := range root.Keys() {
		if !contains(topLevelKeys, key) {
			return nil, fail(key, "unknown key")
		}
	}
	if !yamlutil.Equal(root.Value("toolkit"), 1) {
		return nil, fail("toolkit", "required, supported version is 1")
	}

	platformValue := root.Value("platform")
	if platformValue == nil {
		platformValue = DefaultPlatform
	}
	platforms := make([]string, 0, len(known))
	for name := range known {
		platforms = append(platforms, name)
	}
	sort.Strings(platforms)
	platform, isString := platformValue.(string)
	if _, isKnown := known[platform]; !isString || !isKnown {
		return nil, fail("platform", `unknown platform "`+yamlutil.DartString(platformValue)+`", expected one of `+strings.Join(platforms, ", "))
	}

	var warnings []string
	if policy, ok := root.Value("policy").(*omap.Map[any]); ok && policy.Has("plan_gate") {
		warnings = append(warnings, PlanGateRemoved)
		trimmed := policy.Copy()
		trimmed.Delete("plan_gate")
		root.Set("policy", trimmed)
	}

	profile := &Profile{Platform: platform, Warnings: warnings}
	if profile.PlatformConfig, err = parsePlatformConfig(platform, root.Value("platform_config")); err != nil {
		return nil, err
	}
	if profile.Agents, err = parseAgents(root.Value("agents")); err != nil {
		return nil, err
	}
	if profile.Language, err = merge("language", languageDefaults(), root.Value("language"), true); err != nil {
		return nil, err
	}
	if profile.Stack, err = parseStack(root.Value("stack"), known[platform]); err != nil {
		return nil, err
	}
	if profile.Project, err = parseProject(root.Value("project")); err != nil {
		return nil, err
	}
	if profile.Policy, err = merge("policy", policyDefaults(), root.Value("policy"), true); err != nil {
		return nil, err
	}
	if profile.Paths, err = merge("paths", PathDefaults(), root.Value("paths"), false); err != nil {
		return nil, err
	}
	if profile.CI, err = parseCI(root.Value("ci")); err != nil {
		return nil, err
	}
	return profile, nil
}

func parsePlatformConfig(platform string, value any) (*PlatformConfig, error) {
	if platform != OtherOption {
		if value != nil {
			return nil, fail("platform_config", "allowed only with platform: "+OtherOption)
		}
		return nil, nil
	}
	if value == nil {
		return nil, fail("platform_config", "required with platform: "+OtherOption+", a map of detect and commands")
	}
	config, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fail("platform_config", "expected a map of detect and commands")
	}
	for _, key := range config.Keys() {
		if key != "detect" && key != "commands" {
			return nil, fail("platform_config."+key, "unknown key")
		}
	}
	detect, ok := StringList(config.Value("detect"))
	if !ok || len(detect) == 0 {
		return nil, fail("platform_config.detect", "required, a non-empty list of files that identify the platform")
	}
	commands, err := PlatformCommandsFrom(config.Value("commands"), func(key, message string) error {
		return fail("platform_config."+key, message)
	})
	if err != nil {
		return nil, err
	}
	return &PlatformConfig{Detect: detect, Commands: commands}, nil
}

// StringList returns value as a list of non-empty strings; false when it is not a list or holds anything else.
func StringList(value any) ([]string, bool) {
	list, ok := value.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok || text == "" {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
}

func parseAgents(value any) ([]string, error) {
	list, ok := value.([]any)
	if !ok || len(list) == 0 {
		return nil, fail("agents", "required, a non-empty list")
	}
	agents := make([]string, 0, len(list))
	for _, agent := range list {
		name, isString := agent.(string)
		if !isString || !contains(SupportedAgents, name) {
			return nil, fail("agents", `unsupported agent "`+yamlutil.DartString(agent)+`", expected one of `+strings.Join(SupportedAgents, ", "))
		}
		agents = append(agents, name)
	}
	return agents, nil
}

func merge(path string, defaults *omap.Map[any], value any, strict bool) (*omap.Map[any], error) {
	if value == nil {
		return defaults, nil
	}
	overrides, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fail(path, "expected a map")
	}
	if strict {
		for _, key := range overrides.Keys() {
			if !defaults.Has(key) {
				return nil, fail(path+"."+key, "unknown key")
			}
		}
	}
	for _, key := range overrides.Keys() {
		defaults.Set(key, overrides.Value(key))
	}
	return defaults, nil
}

func parseProject(value any) (*omap.Map[any], error) {
	project, err := merge("project", projectDefaults(), value, false)
	if err != nil {
		return nil, err
	}
	flavors, ok := project.Value("flavors").(*omap.Map[any])
	if !ok {
		return nil, fail("project.flavors", "expected a map of flavor name to entrypoint")
	}
	for _, name := range flavors.Keys() {
		if entrypoint, ok := flavors.Value(name).(string); !ok || entrypoint == "" {
			return nil, fail("project.flavors."+name, "expected the entrypoint path, e.g. lib/main_"+name+".dart")
		}
	}
	return project, nil
}

func parseCI(value any) (*omap.Map[any], error) {
	ci, err := merge("ci", ciDefaults(), value, true)
	if err != nil {
		return nil, err
	}
	ref, isString := ci.Value("dashcrew_ref").(string)
	if ci.Value("github") == true && (!isString || ref == "") {
		return nil, fail("ci.dashcrew_ref", "required with github: true, the dashcrew commit or tag CI installs")
	}
	return ci, nil
}

func parseStack(value any, known *omap.Map[[]string]) (*omap.Map[StackChoice], error) {
	choices := omap.New[StackChoice]()
	if value == nil {
		return choices, nil
	}
	stack, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fail("stack", "expected a map")
	}
	for _, family := range stack.Keys() {
		options, ok := known.Get(family)
		if !ok {
			return nil, fail("stack."+family, "unknown stack family")
		}
		var option string
		hasOption := false
		params := omap.New[any]()
		switch raw := stack.Value(family).(type) {
		case string:
			option, hasOption = raw, true
		case *omap.Map[any]:
			option, hasOption = raw.Value("use").(string)
			for _, key := range raw.Keys() {
				if key != "use" {
					params.Set(key, raw.Value(key))
				}
			}
		}
		if !hasOption || !(contains(options, option) || option == OtherOption) {
			allowed := append([]string{}, options...)
			if !contains(allowed, OtherOption) {
				allowed = append(allowed, OtherOption)
			}
			return nil, fail("stack."+family, "expected one of "+strings.Join(allowed, ", "))
		}
		if name, ok := params.Value("name").(string); option == OtherOption && (!ok || name == "") {
			return nil, fail("stack."+family+".name", "required with use: "+OtherOption+", the library package name")
		}
		choices.Set(family, StackChoice{Option: option, Params: params})
	}
	return choices, nil
}

// ToContext is the template context: every section as maps; `stack.<family>` is `{use: option, ...params}`.
// `stack_other` lists the `other` choices as `{family, name}` sorted by family; `build.flavors` lists
// `project.flavors` as `{name, entrypoint}` sorted by name.
func (p *Profile) ToContext() map[string]any {
	flavors := p.Project.Value("flavors").(*omap.Map[any])
	names := flavors.Keys()
	sort.Strings(names)
	buildFlavors := []any{}
	for _, name := range names {
		buildFlavors = append(buildFlavors, map[string]any{"name": name, "entrypoint": flavors.Value(name)})
	}

	families := p.Stack.Keys()
	sort.Strings(families)
	others := []any{}
	for _, family := range families {
		if choice := p.Stack.Value(family); choice.Option == OtherOption {
			others = append(others, map[string]any{"family": family, "name": choice.Params.Value("name")})
		}
	}

	stack := map[string]any{}
	for _, family := range p.Stack.Keys() {
		choice := p.Stack.Value(family)
		entry := map[string]any{"use": choice.Option}
		for _, key := range choice.Params.Keys() {
			entry[key] = choice.Params.Value(key)
		}
		stack[family] = entry
	}

	agents := make([]any, len(p.Agents))
	for i, agent := range p.Agents {
		agents[i] = agent
	}
	return map[string]any{
		"agents":          agents,
		"language":        p.Language,
		"stack":           stack,
		"stack_other":     others,
		"has_stack_other": len(others) > 0,
		"project":         p.Project,
		"policy":          p.Policy,
		"paths":           p.Paths,
		"ci":              p.CI,
		"build":           map[string]any{"flavors": buildFlavors, "has_flavors": len(buildFlavors) > 0},
	}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
