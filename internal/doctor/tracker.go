package doctor

import (
	"os"
	"os/exec"
	"strings"
)

// githubTracker checks that the `gh` on PATH has `gh project` when the GitHub tracker keeps status in a Project
// (P11): gh before 2.31 has no such command, and the status flow would stop on its first step. No check for other
// trackers or for mode labels.
func (d Doctor) githubTracker() (Check, bool) {
	const id = "tracker"
	p, err := d.parseProfile()
	if err != nil {
		return Check{}, false
	}
	choice, ok := p.Stack.Get("tracker")
	if !ok || choice.Option != "github" {
		return Check{}, false
	}
	mode, _ := choice.Params.Value("mode").(string)
	if mode == "" {
		if options, ok := d.Catalog.Packs.Get("tracker"); ok {
			if pack, ok := options.Get("github"); ok && pack.Params != nil {
				mode, _ = pack.Params.Value("mode").(string)
			}
		}
	}
	if mode != "project" {
		return Check{}, false
	}
	pathEnv := d.searchPath()
	gh, found := lookPath("gh", pathEnv)
	if !found {
		return Check{id, Warn, "gh is not on PATH; the GitHub tracker sets status with gh project (gh 2.31 or newer)"}, true
	}
	cmd := exec.Command(gh, "project", "--help")
	cmd.Env = append(os.Environ(), "PATH="+pathEnv)
	if err := cmd.Run(); err != nil {
		version, _ := exec.Command(gh, "--version").Output()
		first, _, _ := strings.Cut(strings.TrimSpace(string(version)), "\n")
		return Check{id, Warn, first + " has no gh project command; upgrade gh to 2.31 or newer (brew upgrade gh)"}, true
	}
	return Check{id, OK, "gh has the project command; it needs the project scope (gh auth refresh -s project)"}, true
}
