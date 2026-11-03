package doctor

import (
	"io/fs"
	"strings"

	embedded "github.com/fahnaladitia/dashcrew"
	"github.com/fahnaladitia/dashcrew/internal/me"
)

// me checks the personal profile: valid, and its personal files in step with it. It never fails the project.
func (d Doctor) me() Check {
	const id = "me"
	p, err := me.LoadOrNil(d.Me.Home)
	if err != nil {
		return Check{id, Warn, err.Error() + "; fix ~/.dashcrew/me.yaml"}
	}
	if p == nil {
		return Check{id, Skip, "no personal profile; dashcrew me init adds one"}
	}
	personal, err := fs.Sub(embedded.PersonalFS, "personal")
	if err != nil {
		return Check{id, Warn, err.Error()}
	}
	changes, err := me.Status(p, *d.Me, personal)
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
	detail := "role " + p.Role + ", stacks " + strings.Join(p.Stacks, ", ")
	if platform := me.RepoPlatform(d.ProjectDir); platform != "" && !contains(p.Stacks, platform) {
		detail += "; this project's platform " + platform + " is outside your stacks"
	}
	return Check{id, OK, detail}
}
