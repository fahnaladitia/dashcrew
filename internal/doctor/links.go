package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/link"
)

// links reports the override file `dashcrew link` manages: one not written by dashcrew, one git would commit, links
// whose checkout is gone, and (as info) linked checkouts on another branch. No check without `project.links` and
// without an override file.
func (d Doctor) links() (Check, bool) {
	const id = "links"
	p, err := d.parseProfile()
	if err != nil {
		return Check{}, false
	}
	l := &link.Links{Dir: d.ProjectDir, Profile: p, Platform: d.Catalog.PlatformFor(p)}
	name, err := l.File()
	if err != nil {
		if len(p.Links) == 0 {
			return Check{}, false
		}
		return Check{id, Warn, err.Error()}, true
	}
	path := filepath.Join(d.ProjectDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if len(p.Links) == 0 {
			return Check{}, false
		}
		return Check{id, OK, "no links; dashcrew link add <package> <path> points a dependency at a local checkout"}, true
	}
	if exec.Command("git", "-C", d.ProjectDir, "ls-files", "--error-unmatch", "--", name).Run() == nil {
		return Check{id, Fail, name + " is committed; remove it from git (git rm --cached " + name + ")"}, true
	}
	if exec.Command("git", "-C", d.ProjectDir, "check-ignore", "-q", "--", name).Run() != nil {
		return Check{id, Warn, name + " is not ignored by git and would be committed; dashcrew link add excludes it"}, true
	}
	if !link.Managed(strings.ReplaceAll(string(data), "\r\n", "\n")) {
		return Check{id, Warn, name + " was not written by dashcrew link; dashcrew check refuses it if it is ever committed"}, true
	}
	entries, err := l.Read()
	if err != nil {
		return Check{id, Warn, err.Error()}, true
	}
	branch := link.Branch(d.ProjectDir)
	var missing, other []string
	for _, entry := range entries {
		target := filepath.Join(d.ProjectDir, filepath.FromSlash(entry.Path))
		if !isDir(target) {
			missing = append(missing, entry.Package+" ("+entry.Path+")")
		} else if targetBranch := link.Branch(target); targetBranch != branch {
			other = append(other, entry.Package+" on "+orNone(targetBranch))
		}
	}
	switch {
	case len(missing) > 0:
		return Check{id, Warn, "linked checkout missing: " + strings.Join(missing, ", ") + "; dashcrew link remove <package>"}, true
	case len(other) > 0:
		return Check{id, Info, "this checkout is on " + orNone(branch) + "; " + strings.Join(other, ", ")}, true
	}
	return Check{id, OK, strconv.Itoa(len(entries)) + " linked in " + name}, true
}

func orNone(branch string) string {
	if branch == "" {
		return "no branch"
	}
	return branch
}
