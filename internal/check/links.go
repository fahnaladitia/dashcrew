package check

import (
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/deps"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

// Links is `dashcrew check --links`: the committed state of every dependency in `project.links`. An error when the
// platform has no link file.
func Links(dir string, p *profile.Profile, c *catalog.Catalog) ([]deps.Result, error) {
	spec := c.PlatformFor(p)
	if spec.Link == nil || *spec.Link == "" {
		return nil, &linkFileError{spec.Name}
	}
	return deps.Check(dir, p, *spec.Link), nil
}

type linkFileError struct{ platform string }

func (e *linkFileError) Error() string {
	return "platform " + e.platform + " has no link file; project.links needs pubspec_overrides.yaml or go.work"
}

// Line is one result as `check --links` prints it.
func Line(result deps.Result) string {
	label := map[deps.Status]string{deps.OK: "ok", deps.Refused: "refused", deps.Unreachable: "fail", deps.Absent: "absent"}[result.Status]
	name := result.Package
	if name == "" {
		name = "link file"
	}
	return label + "  " + name + ": " + result.Detail
}

// Failed is true when any result blocks the merge.
func Failed(results []deps.Result) bool {
	for _, result := range results {
		if result.Status == deps.Refused || result.Status == deps.Unreachable {
			return true
		}
	}
	return false
}

// links is the `links` check of a pull request, on for profiles with `project.links`.
func (c PrCheck) links(p *profile.Profile) doctor.Check {
	const id = "links"
	results, err := Links(c.ProjectDir, p, c.Catalog)
	if err != nil {
		return doctor.Check{ID: id, Status: doctor.Fail, Detail: err.Error()}
	}
	var problems, fine []string
	for _, result := range results {
		if result.Status == deps.Refused || result.Status == deps.Unreachable {
			problems = append(problems, Line(result))
		} else {
			fine = append(fine, result.Package)
		}
	}
	if len(problems) > 0 {
		return doctor.Check{ID: id, Status: doctor.Fail, Detail: strings.Join(problems, "; ")}
	}
	return doctor.Check{ID: id, Status: doctor.OK, Detail: strings.Join(fine, ", ") + " committed at merged or released versions"}
}
