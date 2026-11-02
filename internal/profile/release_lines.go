package profile

import (
	"regexp"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// ReleaseLines is `project.release_lines`: the main line and the patterns that name release and hotfix lines.
type ReleaseLines struct {
	// Main is the main development line's branch name.
	Main string
	// ReleasePattern and HotfixPattern are the profile's regular expressions; empty when not set.
	ReleasePattern, HotfixPattern string
	// Release and Hotfix match a whole branch name and capture `version`; nil when not set.
	Release, Hotfix *regexp.Regexp
	// CheckPorts makes CI run `dashcrew check --ports`.
	CheckPorts bool
}

// Line kinds Match returns.
const (
	MainLine    = "main"
	ReleaseLine = "release"
	HotfixLine  = "hotfix"
)

var releaseLinesKeys = []string{"main", "release", "hotfix", "check_ports"}

// Match names the line branch is: MainLine, or ReleaseLine or HotfixLine with the captured version. A pattern must
// match the whole name; release wins over hotfix when both match.
func (r *ReleaseLines) Match(branch string) (kind, version string, ok bool) {
	if branch == r.Main {
		return MainLine, "", true
	}
	for _, line := range []struct {
		kind    string
		pattern *regexp.Regexp
	}{{ReleaseLine, r.Release}, {HotfixLine, r.Hotfix}} {
		if line.pattern == nil {
			continue
		}
		if match := line.pattern.FindStringSubmatch(branch); match != nil {
			return line.kind, match[line.pattern.SubexpIndex("version")], true
		}
	}
	return "", "", false
}

// parseReleaseLines reads `project.release_lines` with strict keys and removes it from project, so the template
// context of a profile stays as it was.
func parseReleaseLines(project *omap.Map[any]) (*ReleaseLines, error) {
	value := project.Value("release_lines")
	project.Delete("release_lines")
	if value == nil {
		return nil, nil
	}
	config, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fail("project.release_lines", "expected a map with main, release, hotfix and check_ports")
	}
	for _, key := range config.Keys() {
		if !contains(releaseLinesKeys, key) {
			return nil, fail("project.release_lines."+key, "unknown key")
		}
	}
	main, ok := config.Value("main").(string)
	if !ok || main == "" {
		return nil, fail("project.release_lines.main", "required, the main line's branch name")
	}
	lines := &ReleaseLines{Main: main}
	var err error
	if lines.ReleasePattern, lines.Release, err = linePattern(config, "release"); err != nil {
		return nil, err
	}
	if lines.HotfixPattern, lines.Hotfix, err = linePattern(config, "hotfix"); err != nil {
		return nil, err
	}
	if lines.Release == nil && lines.Hotfix == nil {
		return nil, fail("project.release_lines", "needs release or hotfix, a regular expression with a named group (?P<version>...)")
	}
	if config.Has("check_ports") {
		if lines.CheckPorts, ok = config.Value("check_ports").(bool); !ok {
			return nil, fail("project.release_lines.check_ports", "expected true or false")
		}
	}
	return lines, nil
}

// linePattern compiles `release_lines.<key>` anchored to the whole branch name; nil when the key is absent.
func linePattern(config *omap.Map[any], key string) (string, *regexp.Regexp, error) {
	if !config.Has(key) {
		return "", nil, nil
	}
	path := "project.release_lines." + key
	text, ok := config.Value(key).(string)
	if !ok || text == "" {
		return "", nil, fail(path, "expected a regular expression with a named group (?P<version>...)")
	}
	if _, err := regexp.Compile(text); err != nil {
		return "", nil, fail(path, "invalid regular expression: "+err.Error())
	}
	pattern := regexp.MustCompile(`^(?:` + text + `)$`)
	if pattern.SubexpIndex("version") < 0 {
		return "", nil, fail(path, `needs a named group version, e.g. `+key+`/(?P<version>[0-9]+\.[0-9]+\.[0-9]+)`)
	}
	return text, pattern, nil
}
