package commit

import (
	"reflect"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

func linesConfig(t *testing.T) *profile.Commit {
	t.Helper()
	p, err := profile.Parse("toolkit: 1\nagents: [claude-code]\nproject:\n  release_lines: { main: develop, release: 'release/(?P<version>[0-9.]+)' }\n",
		profile.KnownOptions{"flutter": omap.New[[]string]()})
	if err != nil {
		t.Fatal(err)
	}
	return p.Commit
}

func TestPortTrailersAreAllowedOnlyWithReleaseLinesAndKeepTheirShape(t *testing.T) {
	cfg := linesConfig(t)
	if !cfg.PortTrailers {
		t.Fatal("release_lines does not allow the port trailers")
	}
	cases := map[string][]string{
		"fix: totals\n\nPorted-from: abc1234":                    nil,
		"fix: totals\n\nPorted-from: " + strings.Repeat("a", 40): nil,
		"fix: totals\n\nPort: yes":                               nil,
		"fix: totals\n\nPorted-from: abc1234\nPort: yes":         nil,
		"fix: totals\n\nPorted-from: abc12":                      {`trailer Ported-from "abc12" is not a commit hash (7 to 40 hex characters)`},
		"fix: totals\n\nPorted-from: release/2.4.0":              {`trailer Ported-from "release/2.4.0" is not a commit hash (7 to 40 hex characters)`},
		"fix: totals\n\nPort: true":                              {`trailer Port "true" must be yes`},
	}
	for message, want := range cases {
		if got := Lint(cfg, message); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %q, want %q", message, got, want)
		}
	}
	plain := config(t, "").Commit
	if plain.PortTrailers {
		t.Error("PortTrailers without release_lines")
	}
	if got := Lint(plain, "fix: totals\n\nPorted-from: abc1234"); len(got) != 1 || !strings.Contains(got[0], `trailer "Ported-from" is not allowed`) {
		t.Errorf("%q", got)
	}
}

func TestParsedReadsTypeAndTrailers(t *testing.T) {
	cfg := linesConfig(t)
	got := Parse(cfg, "fix(cart): keep the total\n\nWhy.\n\nPorted-from: abc1234\nPort: yes")
	if got.Type != "fix" || got.Subject != "fix(cart): keep the total" {
		t.Errorf("%+v", got)
	}
	if !reflect.DeepEqual(got.Trailers, [][2]string{{"Ported-from", "abc1234"}, {"Port", "yes"}}) {
		t.Errorf("%q", got.Trailers)
	}
	if got := Parse(cfg, "Fix things\n\nnot: a trailer block\nbecause this line is prose"); got.Type != "" || len(got.Trailers) != 0 {
		t.Errorf("%+v", got)
	}
}

func TestMessageAddsPortedFromTrailersAfterTheTicket(t *testing.T) {
	cfg := linesConfig(t)
	got := Message(cfg, Parts{Type: "fix", Subject: "totals", PortedFrom: []string{"abc1234", "def5678"}})
	if want := "fix: totals\n\nPorted-from: abc1234\nPorted-from: def5678"; got != want {
		t.Errorf("%q", got)
	}
}
