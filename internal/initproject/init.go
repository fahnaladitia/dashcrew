package initproject

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// ProfilePath is the only file init writes.
const ProfilePath = ".ai/project.yaml"

var (
	packageName    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	whitespace     = regexp.MustCompile(`\s+`)
	promptTail     = regexp.MustCompile(`[:\s]+$`)
	plainScalar    = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_./-]*$`)
	reservedScalar = map[string]bool{"true": true, "false": true, "yes": true, "no": true, "on": true, "off": true,
		"null": true, "y": true, "n": true}
)

// Ask shows a prompt and returns the typed line, or nil at end of input (which keeps the proposal).
type Ask func(prompt string) *string

// Init writes `.ai/project.yaml` from what DetectPlatforms and Detect find, confirmed per question unless yes.
// A project that matches no platform is offered `platform: other`, never the default platform.
// It writes nothing else; `dashcrew sync` renders the toolkit afterwards.
type Init struct {
	Catalog    *catalog.Catalog
	ProjectDir string
	// Agents listed in the profile; empty means claude-code.
	Agents []string
}

type choice struct {
	option string
	name   string
	params *omap.Map[any]
}

func choiceOf(c *StackCandidate) *choice { return &choice{c.Option, c.Name, c.Params} }

func trimmed(answer *string) *string {
	if answer == nil {
		return nil
	}
	value := strings.TrimSpace(*answer)
	return &value
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func sorted(list []string) []string {
	out := append([]string{}, list...)
	sort.Strings(out)
	return out
}

// Run writes the profile; it refuses an existing profile unless force.
func (in Init) Run(yes, force bool, remoteBranches []string, ask Ask, out func(string)) error {
	file := filepath.Join(in.ProjectDir, filepath.FromSlash(ProfilePath))
	if _, err := os.Stat(file); err == nil && isFile(file) && !force {
		return &Error{ProfilePath + " already exists. Rerun with --force to replace it."}
	}

	c, err := in.Catalog.WithProjectPacks(in.ProjectDir)
	if err != nil {
		return &Error{"Project pack: " + err.Error()}
	}
	matches := DetectPlatforms(in.ProjectDir, c.Platforms)
	platformNames := make([]string, 0, len(c.Platforms))
	for name := range c.Platforms {
		platformNames = append(platformNames, name)
	}
	sort.Strings(platformNames)

	out("Detected in " + in.ProjectDir + ":")
	for _, name := range matches {
		files := []string{}
		for _, f := range c.Platforms[name].Detect {
			if isFile(filepath.Join(in.ProjectDir, f)) {
				files = append(files, f)
			}
		}
		out("  platform: " + name + " (" + strings.Join(files, ", ") + ")")
	}
	if len(matches) == 0 {
		expected := []string{}
		for _, name := range platformNames {
			expected = append(expected, name+" ("+strings.Join(c.Platforms[name].Detect, ", ")+")")
		}
		out("  platform: none of " + strings.Join(expected, ", "))
	}

	var platform string
	if yes {
		platform, err = platformWithoutAsking(matches)
	} else {
		platform, err = askPlatform(platformNames, matches, ask, out)
	}
	if err != nil {
		return err
	}
	var platformConfig *omap.Map[any]
	if platform == profile.OtherOption {
		if platformConfig, err = askPlatformConfig(ask, out); err != nil {
			return err
		}
	}

	known := c.KnownOptions()[platform]
	detection, err := Detect(in.ProjectDir, remoteBranches, platform)
	if err != nil {
		return err
	}
	candidates := map[string][]*StackCandidate{}
	for _, family := range detection.Stack.Keys() {
		if options, ok := known.Get(family); ok {
			for _, candidate := range detection.Stack.Value(family) {
				candidates[family] = append(candidates[family], fit(candidate, options))
			}
		}
	}

	families := make([]string, 0, len(candidates))
	for family := range candidates {
		families = append(families, family)
	}
	sort.Strings(families)
	for _, family := range families {
		parts := []string{}
		for _, c := range candidates[family] {
			parts = append(parts, c.Label()+" ("+strings.Join(c.Evidence, ", ")+")")
		}
		out("  " + family + ": " + strings.Join(parts, " | "))
	}
	if detection.Flavors.Len() > 0 {
		out("  flavors: " + strings.Join(detection.Flavors.Keys(), ", ") + " (lib/main_<flavor>.dart)")
	}
	if len(detection.LocalPackages) > 0 {
		out("  local packages: " + strings.Join(detection.LocalPackages, ", ") + " (path dependencies)")
	}
	if len(detection.BaseBranches) > 0 {
		out("  base branches: " + strings.Join(detection.BaseBranches, ", ") + " (remote branches)")
	}

	choices := map[string]*choice{}
	for _, family := range sorted(known.Keys()) {
		proposed := candidates[family]
		var picked *choice
		if yes {
			if len(proposed) > 1 {
				labels := []string{}
				for _, c := range proposed {
					labels = append(labels, c.Label())
				}
				out(family + ": skipped, " + strconv.Itoa(len(proposed)) + " candidates: " + strings.Join(labels, ", ") +
					"; set it in " + ProfilePath)
			}
			if len(proposed) == 1 {
				picked = choiceOf(proposed[0])
			}
		} else {
			picked = askFamily(family, known.Value(family), proposed, ask, out)
		}
		if picked != nil {
			choices[family] = picked
		}
	}

	agents := in.Agents
	if len(agents) == 0 {
		agents = []string{"claude-code"}
	}
	yaml := profileYaml(platform, platformConfig, agents, choices, detection)
	if _, err := profile.Parse(yaml, c.KnownOptions()); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(yaml), 0o644); err != nil {
		return err
	}
	out("wrote " + ProfilePath)
	out("next: dashcrew sync")
	return nil
}

func platformWithoutAsking(matches []string) (string, error) {
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", &Error{"No platform detected. Rerun without --yes to declare platform: " + profile.OtherOption +
			" and its commands."}
	}
	return "", &Error{strconv.Itoa(len(matches)) + " platforms detected: " + strings.Join(matches, ", ") +
		". Rerun without --yes and pick one."}
}

// askPlatform proposes the one detected platform, `other` when none matched, and nothing when several did.
func askPlatform(platformNames, matches []string, ask Ask, out func(string)) (string, error) {
	options := append(append([]string{}, platformNames...), profile.OtherOption)
	proposal := ""
	shown := profile.OtherOption
	switch len(matches) {
	case 0:
		proposal = profile.OtherOption
	case 1:
		proposal = matches[0]
		shown = matches[0]
	default:
		shown = strings.Join(matches, " | ")
	}
	for {
		answer := trimmed(ask("platform [" + shown + "]: "))
		if answer == nil && proposal == "" {
			return "", &Error{"Several platforms detected (" + strings.Join(matches, ", ") + "); pick one."}
		}
		if answer == nil || *answer == "" {
			if proposal != "" {
				return proposal, nil
			}
		} else if contains(options, *answer) {
			return *answer, nil
		}
		out("  expected one of " + strings.Join(options, ", "))
	}
}

// askPlatformConfig asks the `platform_config` of `other`: the detect files and each command, one by one.
// The result holds `detect` ([]string) and `commands` (an ordered map of string or nil).
func askPlatformConfig(ask Ask, out func(string)) (*omap.Map[any], error) {
	answer := func(prompt string, required bool) (*string, error) {
		for {
			value := trimmed(ask(prompt))
			if value == nil {
				if !required {
					return nil, nil
				}
				return nil, &Error{"platform: " + profile.OtherOption + " needs " + promptTail.ReplaceAllString(prompt, "") +
					"; rerun dashcrew init, or write platform_config in " + ProfilePath + " by hand."}
			}
			if *value != "" {
				return value, nil
			}
			if !required {
				return nil, nil
			}
			out("  required")
		}
	}

	detect, err := answer("detect (files that identify the platform, space separated): ", true)
	if err != nil {
		return nil, err
	}
	commands := omap.New[any]()
	for _, key := range profile.PlatformCommands {
		command, err := answer("commands."+key+": ", true)
		if err != nil {
			return nil, err
		}
		commands.Set(key, *command)
	}
	for _, key := range profile.OptionalPlatformCommands {
		command, err := answer("commands."+key+" (empty for none): ", false)
		if err != nil {
			return nil, err
		}
		if command == nil {
			commands.Set(key, nil)
		} else {
			commands.Set(key, *command)
		}
	}
	return omap.Of[any]("detect", whitespace.Split(*detect, -1), "commands", commands), nil
}

// fit turns a detected option the catalog has no pack for into `other` with the package name.
func fit(candidate *StackCandidate, options []string) *StackCandidate {
	if candidate.Option == profile.OtherOption || contains(options, candidate.Option) {
		return candidate
	}
	out := newCandidate(candidate.Family, profile.OtherOption, candidate.Packages[0])
	out.Packages = append(out.Packages, candidate.Packages...)
	for _, key := range candidate.Params.Keys() {
		out.Params.Set(key, candidate.Params.Value(key))
	}
	out.Evidence = append(out.Evidence, candidate.Evidence...)
	return out
}

func askFamily(family string, options []string, proposed []*StackCandidate, ask Ask, out func(string)) *choice {
	var fallback *choice
	if len(proposed) == 1 {
		fallback = choiceOf(proposed[0])
	}
	shown := "-"
	if len(proposed) > 0 {
		labels := []string{}
		for _, c := range proposed {
			labels = append(labels, c.Label())
		}
		shown = strings.Join(labels, " | ")
	}
	for {
		answer := trimmed(ask(family + " [" + shown + "]: "))
		if answer == nil || *answer == "" {
			return fallback
		}
		if *answer == "-" {
			return nil
		}
		if contains(options, *answer) {
			for _, c := range proposed {
				if c.Option == *answer && c.Name == "" {
					return choiceOf(c)
				}
			}
			return &choice{*answer, "", omap.New[any]()}
		}
		words := whitespace.Split(*answer, -1)
		if len(words) == 2 && words[0] == profile.OtherOption && packageName.MatchString(words[1]) {
			return &choice{profile.OtherOption, words[1], omap.New[any]()}
		}
		out("  expected one of " + strings.Join(append(sorted(options), profile.OtherOption+" <package>", "- (none)"), ", "))
	}
}

func profileYaml(platform string, platformConfig *omap.Map[any], agents []string, choices map[string]*choice,
	detection *Detection) string {
	var yaml strings.Builder
	yaml.WriteString("# Written by dashcrew init. Edit it, then run dashcrew sync.\n")
	yaml.WriteString("toolkit: 1\n")
	yaml.WriteString("platform: " + scalar(platform) + "\n")
	if platformConfig != nil {
		yaml.WriteString("platform_config:\n")
		yaml.WriteString("  detect: [" + scalars(platformConfig.Value("detect").([]string)) + "]\n")
		yaml.WriteString("  commands:\n")
		commands := platformConfig.Value("commands").(*omap.Map[any])
		for _, key := range commands.Keys() {
			if command, ok := commands.Value(key).(string); ok {
				yaml.WriteString("    " + key + ": " + scalar(command) + "\n")
			}
		}
	}
	yaml.WriteString("agents: [" + scalars(agents) + "]\n")
	if len(choices) > 0 {
		yaml.WriteString("stack:\n")
		families := make([]string, 0, len(choices))
		for family := range choices {
			families = append(families, family)
		}
		sort.Strings(families)
		for _, family := range families {
			c := choices[family]
			fields := []string{}
			if c.name != "" {
				fields = append(fields, "name: "+scalar(c.name))
			}
			for _, key := range c.params.Keys() {
				value := c.params.Value(key)
				if text, ok := value.(string); ok {
					fields = append(fields, key+": "+scalar(text))
				} else {
					fields = append(fields, key+": "+yamlutil.DartString(value))
				}
			}
			value := scalar(c.option)
			if len(fields) > 0 {
				value = "{ use: " + scalar(c.option) + ", " + strings.Join(fields, ", ") + " }"
			}
			yaml.WriteString("  " + family + ": " + value + "\n")
		}
	}
	var project strings.Builder
	if detection.Flavors.Len() > 0 {
		project.WriteString("  flavors:\n")
		for _, name := range detection.Flavors.Keys() {
			project.WriteString("    " + scalar(name) + ": " + scalar(detection.Flavors.Value(name)) + "\n")
		}
	}
	if len(detection.LocalPackages) > 0 {
		project.WriteString("  local_packages: [" + scalars(detection.LocalPackages) + "]\n")
	}
	if len(detection.BaseBranches) > 0 {
		project.WriteString("  base_branches: [" + scalars(detection.BaseBranches) + "]\n")
	}
	if project.Len() > 0 {
		yaml.WriteString("project:\n" + project.String())
	}
	return yaml.String()
}

func scalars(values []string) string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = scalar(value)
	}
	return strings.Join(out, ", ")
}

// scalar is plain YAML when safe, a JSON string (valid YAML) otherwise.
func scalar(value string) string {
	if plainScalar.MatchString(value) && !reservedScalar[strings.ToLower(value)] {
		return value
	}
	encoded, _ := dartjson.Encode(value)
	return encoded
}
