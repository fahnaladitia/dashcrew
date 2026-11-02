package profile

import (
	"reflect"
	"testing"
)

const otherPlatform = "platform: other\nplatform_config: { detect: [a], commands: { analyze: a, test: b, test_scoped: c, deps: d }"

func TestLinksAreReadAndKeptOutOfProject(t *testing.T) {
	p := parse(t, "toolkit: 1\nagents: [claude-code]\nproject:\n  links:\n"+
		"    - { package: shared_core, repo: 'git@github.com:acme/shared_core.git' }\n"+
		"    - { package: github.com/acme/api }\n")
	want := []Link{{Package: "shared_core", Repo: "git@github.com:acme/shared_core.git"}, {Package: "github.com/acme/api"}}
	if !reflect.DeepEqual(p.Links, want) {
		t.Errorf("links %#v", p.Links)
	}
	if p.Project.Has("links") {
		t.Error("project still holds links; templates must see only has_links")
	}
	if p.ToContext()["has_links"] != true {
		t.Error("has_links is not set")
	}
	if link, ok := p.LinkOf("github.com/acme/api"); !ok || link.Repo != "" {
		t.Errorf("LinkOf %#v %v", link, ok)
	}
	if _, ok := p.LinkOf("other"); ok {
		t.Error("LinkOf found an unlisted package")
	}
}

func TestNoLinksMeansNoFlag(t *testing.T) {
	p := parse(t, "toolkit: 1\nagents: [claude-code]\n")
	if p.Links != nil || p.ToContext()["has_links"] != false {
		t.Errorf("links %#v has_links %v", p.Links, p.ToContext()["has_links"])
	}
}

func TestLinkErrors(t *testing.T) {
	cases := []struct{ name, links, message string }{
		{"not a list", "links: { a: b }", "project.yaml project.links: expected a non-empty list of { package, repo }"},
		{"empty list", "links: []", "project.yaml project.links: expected a non-empty list of { package, repo }"},
		{"entry not a map", "links: [core]", "project.yaml project.links[0]: expected a map with package and an optional repo"},
		{"unknown key", "links: [{ package: core, ref: main }]", "project.yaml project.links[0].ref: unknown key"},
		{"no package", "links: [{ repo: 'https://x/y.git' }]", "project.yaml project.links[0].package: required, the dependency's name in pubspec.yaml or its Go module path"},
		{"package with a space", "links: [{ package: 'a b' }]", "project.yaml project.links[0].package: required, the dependency's name in pubspec.yaml or its Go module path"},
		{"twice", "links: [{ package: core }, { package: core }]", `project.yaml project.links[1].package: "core" is listed twice`},
		{"repo not a URL", "links: [{ package: core, repo: ../core }]", "project.yaml project.links[0].repo: expected a git URL, https://... or git@host:owner/repo.git"},
		{"repo with credentials", "links: [{ package: core, repo: 'https://user:token@github.com/a/b.git' }]", "project.yaml project.links[0].repo: expected a git URL, https://... or git@host:owner/repo.git"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseError(t, "toolkit: 1\nagents: [claude-code]\nproject: { "+c.links+" }\n").Error(); got != c.message {
				t.Errorf("got %q\nwant %q", got, c.message)
			}
		})
	}
}

func TestPlatformConfigLink(t *testing.T) {
	p := parse(t, "toolkit: 1\nagents: [claude-code]\n"+otherPlatform+", link: go.work }\n")
	if p.PlatformConfig.Link != "go.work" {
		t.Errorf("link %q", p.PlatformConfig.Link)
	}
	if p := parse(t, "toolkit: 1\nagents: [claude-code]\n"+otherPlatform+" }\n"); p.PlatformConfig.Link != "" {
		t.Errorf("link without the key %q", p.PlatformConfig.Link)
	}
	got := parseError(t, "toolkit: 1\nagents: [claude-code]\n"+otherPlatform+", link: Cargo.toml }\n").Error()
	if want := "project.yaml platform_config.link: expected one of pubspec_overrides.yaml, go.work"; got != want {
		t.Errorf("got %q", got)
	}
}
