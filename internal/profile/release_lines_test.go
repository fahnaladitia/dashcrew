package profile

import (
	"testing"
)

func TestReleaseLinesAreNilWithoutTheKeyAndNeverReachTheContext(t *testing.T) {
	p := parse(t, "toolkit: 1\nagents: [claude-code]\n")
	if p.ReleaseLines != nil {
		t.Error("release_lines without the key")
	}
	with := parse(t, "toolkit: 1\nagents: [claude-code]\nproject:\n  release_lines:\n    main: develop\n"+
		"    release: \"release/(?P<version>[0-9]+\\\\.[0-9]+\\\\.[0-9]+)\"\n")
	if with.Project.Has("release_lines") {
		t.Error("release_lines reached the project context")
	}
}

func TestReleaseLinesParse(t *testing.T) {
	p := parse(t, "toolkit: 1\nagents: [claude-code]\nproject:\n  release_lines:\n    main: develop\n"+
		"    release: 'development/app-v(?P<version>[0-9.]+)'\n    hotfix: 'hotfix/(?P<version>[0-9.]+)'\n    check_ports: true\n")
	lines := p.ReleaseLines
	if lines == nil || lines.Main != "develop" || !lines.CheckPorts {
		t.Fatalf("parsed %+v", lines)
	}
	if lines.ReleasePattern != "development/app-v(?P<version>[0-9.]+)" || lines.HotfixPattern != "hotfix/(?P<version>[0-9.]+)" {
		t.Errorf("patterns %q %q", lines.ReleasePattern, lines.HotfixPattern)
	}
	for _, c := range []struct {
		branch, kind, version string
		ok                    bool
	}{
		{"development/app-v2.4.0", "release", "2.4.0", true},
		{"hotfix/2.4.5", "hotfix", "2.4.5", true},
		{"develop", "main", "", true},
		{"feat/development/app-v2.4.0", "", "", false},
		{"development/app-v2.4.0-x", "", "", false},
		{"feat/x", "", "", false},
	} {
		kind, version, ok := lines.Match(c.branch)
		if kind != c.kind || version != c.version || ok != c.ok {
			t.Errorf("Match(%q) = %q %q %v", c.branch, kind, version, ok)
		}
	}
}

func TestReleaseLinesCheckPortsDefaultsToOffAndHotfixIsOptional(t *testing.T) {
	p := parse(t, "toolkit: 1\nagents: [claude-code]\nproject: { release_lines: { main: main, release: 'r/(?P<version>.+)' } }\n")
	if p.ReleaseLines.CheckPorts || p.ReleaseLines.Hotfix != nil {
		t.Errorf("parsed %+v", p.ReleaseLines)
	}
}

func TestReleaseLinesErrors(t *testing.T) {
	const head = "toolkit: 1\nagents: [claude-code]\nproject: { release_lines: "
	for yaml, want := range map[string]string{
		"x }": "project.yaml project.release_lines: expected a map with main, release, hotfix and check_ports",
		"{ main: m, release: 'r/(?P<version>.+)', other: 1 } }": "project.yaml project.release_lines.other: unknown key",
		"{ release: 'r/(?P<version>.+)' } }":                    "project.yaml project.release_lines.main: required, the main line's branch name",
		"{ main: '', release: 'r/(?P<version>.+)' } }":          "project.yaml project.release_lines.main: required, the main line's branch name",
		"{ main: m } }":                                                 "project.yaml project.release_lines: needs release or hotfix, a regular expression with a named group (?P<version>...)",
		"{ main: m, release: 'r/(.+)' } }":                              "project.yaml project.release_lines.release: needs a named group version, e.g. release/(?P<version>[0-9]+\\.[0-9]+\\.[0-9]+)",
		"{ main: m, hotfix: 'h/(?P<v>.+)' } }":                          "project.yaml project.release_lines.hotfix: needs a named group version, e.g. hotfix/(?P<version>[0-9]+\\.[0-9]+\\.[0-9]+)",
		"{ main: m, release: 'r/(?P<version>[0-9' } }":                  "project.yaml project.release_lines.release: invalid regular expression: error parsing regexp: missing closing ]: `[0-9`",
		"{ main: m, release: 1 } }":                                     "project.yaml project.release_lines.release: expected a regular expression with a named group (?P<version>...)",
		"{ main: m, release: 'r/(?P<version>.+)', check_ports: yes } }": "project.yaml project.release_lines.check_ports: expected true or false",
	} {
		if got := parseError(t, head+yaml+"\n").Error(); got != want {
			t.Errorf("%s\n got %q\nwant %q", yaml, got, want)
		}
	}
}
