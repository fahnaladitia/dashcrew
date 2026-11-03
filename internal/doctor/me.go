package doctor

import (
	"io/fs"
	"os/exec"
	"strings"

	embedded "github.com/fahnaladitia/dashcrew"
	"github.com/fahnaladitia/dashcrew/internal/me"
)

// me checks the personal profile: valid, its personal files in step, and none of this repo's tracked by git. It
// fails only for a tracked personal file, since that would reach the team.
func (d Doctor) me() Check {
	const id = "me"
	if out, err := exec.Command("git", append([]string{"-C", d.ProjectDir, "ls-files", "--"}, me.RepoFiles...)...).Output(); err == nil {
		if tracked := strings.Fields(string(out)); len(tracked) > 0 {
			return Check{id, Fail, "personal files tracked by git: " + strings.Join(tracked, ", ") + "; git rm --cached them, dashcrew me sync excludes them"}
		}
	}
	home, err := me.LoadOrNil(d.Me.Home)
	if err != nil {
		return Check{id, Warn, err.Error() + "; fix ~/.dashcrew/me.yaml"}
	}
	if home == nil {
		return Check{id, Skip, "no personal profile; dashcrew me init adds one"}
	}
	repo, err := me.LoadRepo(d.ProjectDir)
	if err != nil {
		return Check{id, Warn, err.Error()}
	}
	personal, err := fs.Sub(embedded.PersonalFS, "personal")
	if err != nil {
		return Check{id, Warn, err.Error()}
	}
	changes, err := me.Status(home, *d.Me, personal)
	if err != nil {
		return Check{id, Warn, err.Error()}
	}
	if len(changes) > 0 {
		var lines []string
		for _, change := range changes {
			lines = append(lines, change.String())
		}
		return Check{id, Warn, "personal files out of step: " + strings.Join(lines, ", ") + "; run dashcrew me sync"}
	}
	detail := me.EffectiveIn(home, repo).Describe() + ", stacks " + strings.Join(home.Stacks, ", ")
	if platform := me.RepoPlatform(d.ProjectDir); platform != "" && !contains(home.Stacks, platform) {
		detail += "; this project's platform " + platform + " is outside your stacks"
	}
	return Check{id, OK, detail}
}
