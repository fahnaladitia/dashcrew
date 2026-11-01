// Package initproject is `dashcrew init`: it detects the platform and stack of a project and writes
// `.ai/project.yaml`, with the same questions, output lines and profile text as the Dart engine.
package initproject

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Error is a refused init; Message says why and how to proceed.
type Error struct {
	Message string
}

func (e *Error) Error() string { return e.Message }

// DetectionRule is the stack choice one dependency implies; Name is set for `other`, Params for a pack setting.
type DetectionRule struct {
	Family string
	Option string
	Name   string
	Params *omap.Map[any]
}

func rule(family, option string) DetectionRule { return DetectionRule{Family: family, Option: option} }

func otherRule(family, name string) DetectionRule {
	return DetectionRule{Family: family, Option: profile.OtherOption, Name: name}
}

// DetectionRules maps a dependency name to the stack choices it implies; one library may fill several families.
// Adding a library is one entry here plus a test row.
var DetectionRules = map[string][]DetectionRule{
	"flutter_bloc":       {rule("state", "bloc")},
	"bloc":               {rule("state", "bloc")},
	"flutter_riverpod":   {rule("state", "riverpod"), rule("di", "riverpod")},
	"hooks_riverpod":     {rule("state", "riverpod"), rule("di", "riverpod")},
	"riverpod":           {rule("state", "riverpod"), rule("di", "riverpod")},
	"provider":           {rule("state", "provider"), rule("di", "provider")},
	"mobx":               {otherRule("state", "mobx")},
	"flutter_mobx":       {otherRule("state", "mobx")},
	"get_it":             {rule("di", "get_it")},
	"injectable":         {{Family: "di", Option: "get_it", Params: omap.Of[any]("injectable", true)}},
	"get":                {rule("router", "getx")},
	"go_router":          {rule("router", "go_router")},
	"auto_route":         {rule("router", "auto_route")},
	"dio":                {rule("network", "dio")},
	"http":               {rule("network", "http")},
	"retrofit":           {rule("network", "retrofit")},
	"chopper":            {otherRule("network", "chopper")},
	"hive":               {rule("storage", "hive")},
	"hive_ce":            {rule("storage", "hive")},
	"hive_flutter":       {rule("storage", "hive")},
	"hive_ce_flutter":    {rule("storage", "hive")},
	"shared_preferences": {rule("storage", "shared_preferences")},
	"sqflite":            {rule("storage", "sqflite")},
	"isar":               {otherRule("storage", "isar")},
	"drift":              {otherRule("storage", "drift")},
	"posthog_flutter":    {rule("analytics", "posthog")},
	"firebase_analytics": {rule("analytics", "firebase")},
	"mixpanel_flutter":   {rule("analytics", "mixpanel")},
	"mocktail":           {rule("test", "mocktail")},
	"mockito":            {rule("test", "mockito")},
}

// BaseBranchCandidates are the remote branches init proposes as base branches, in this order.
var BaseBranchCandidates = []string{"main", "master", "develop", "dev"}

// StackCandidate is one proposed value for a stack family, with the dependencies that led to it.
type StackCandidate struct {
	Family string
	Option string
	// Name is the library of an `other` candidate; empty otherwise.
	Name string
	// Params are pack params the dependencies imply, such as `injectable: true` for get_it.
	Params *omap.Map[any]
	// Packages are the dependency names behind the candidate; Evidence is the same with their pubspec section.
	Packages []string
	Evidence []string
}

func newCandidate(family, option, name string) *StackCandidate {
	return &StackCandidate{Family: family, Option: option, Name: name, Params: omap.New[any]()}
}

// Label is the option, or `other <name>` for a library without a pack.
func (c *StackCandidate) Label() string {
	if c.Name == "" {
		return c.Option
	}
	return c.Option + " " + c.Name
}

// Detection is what init read from the project; it guesses nothing beyond these sources.
type Detection struct {
	// Stack maps a family to its distinct candidates in detection order; more than one means the user chooses.
	Stack         *omap.Map[[]*StackCandidate]
	Flavors       *omap.Map[string]
	LocalPackages []string
	BaseBranches  []string
}

// isFile is a file or a link to one, as Dart's `File.existsSync`.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// DetectPlatforms names the platforms with at least one detect file in projectDir, sorted.
func DetectPlatforms(projectDir string, platforms map[string]*catalog.PlatformSpec) []string {
	names := make([]string, 0, len(platforms))
	for name := range platforms {
		names = append(names, name)
	}
	sort.Strings(names)
	out := []string{}
	for _, name := range names {
		for _, file := range platforms[name].Detect {
			if isFile(filepath.Join(projectDir, file)) {
				out = append(out, name)
				break
			}
		}
	}
	return out
}

var entrypoint = regexp.MustCompile(`^main_([a-z0-9_]+)\.dart$`)

// Detect reads remoteBranches (`origin/<name>`) and, for the default platform only, `pubspec.yaml` and
// `lib/main_<flavor>.dart`; any other platform gets no stack, flavor or local package proposal.
func Detect(projectDir string, remoteBranches []string, platform string) (*Detection, error) {
	detection := &Detection{Stack: omap.New[[]*StackCandidate](), Flavors: omap.New[string](), LocalPackages: []string{},
		BaseBranches: []string{}}
	for _, name := range BaseBranchCandidates {
		for _, remote := range remoteBranches {
			if remote == "origin/"+name {
				detection.BaseBranches = append(detection.BaseBranches, name)
				break
			}
		}
	}
	if platform != profile.DefaultPlatform {
		return detection, nil
	}
	pubspecPath := filepath.Join(projectDir, "pubspec.yaml")
	if !isFile(pubspecPath) {
		return nil, &Error{"No pubspec.yaml in " + projectDir + ". Run dashcrew init in the Flutter project root."}
	}
	text, err := os.ReadFile(pubspecPath)
	if err != nil {
		return nil, err
	}
	parsed, err := yamlutil.Parse(string(text))
	if err != nil {
		return nil, err
	}
	pubspec, ok := parsed.(*omap.Map[any])
	if !ok {
		return nil, &Error{"pubspec.yaml is not a YAML map."}
	}

	for _, section := range []string{"dependencies", "dev_dependencies"} {
		dependencies, ok := pubspec.Value(section).(*omap.Map[any])
		if !ok {
			continue
		}
		names := dependencies.Keys()
		sort.Strings(names)
		for _, name := range names {
			if source, ok := dependencies.Value(name).(*omap.Map[any]); ok {
				if raw, ok := source.Value("path").(string); ok {
					local := path.Clean(strings.ReplaceAll(raw, `\`, "/"))
					if !strings.HasPrefix(local, "..") && !path.IsAbs(local) {
						detection.LocalPackages = append(detection.LocalPackages, local)
					}
				}
			}
			for _, r := range DetectionRules[name] {
				candidates := detection.Stack.Value(r.Family)
				var candidate *StackCandidate
				for _, c := range candidates {
					if c.Option == r.Option && c.Name == r.Name {
						candidate = c
						break
					}
				}
				if candidate == nil {
					candidate = newCandidate(r.Family, r.Option, r.Name)
					candidates = append(candidates, candidate)
				}
				detection.Stack.Set(r.Family, candidates)
				if r.Params != nil {
					for _, key := range r.Params.Keys() {
						candidate.Params.Set(key, r.Params.Value(key))
					}
				}
				candidate.Packages = append(candidate.Packages, name)
				candidate.Evidence = append(candidate.Evidence, name+" in "+section)
			}
		}
	}

	entries, err := os.ReadDir(filepath.Join(projectDir, "lib"))
	if err == nil {
		for _, entry := range entries {
			match := entrypoint.FindStringSubmatch(entry.Name())
			if match != nil && isFile(filepath.Join(projectDir, "lib", entry.Name())) {
				detection.Flavors.Set(match[1], "lib/"+entry.Name())
			}
		}
	}
	return detection, nil
}

// RemoteBranches lists `origin/<name>` for every remote-tracking branch; empty outside a git repository.
func RemoteBranches(projectDir string) []string {
	cmd := exec.Command("git", "branch", "-r", "--format=%(refname:short)")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err != nil {
		return []string{}
	}
	out := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}
