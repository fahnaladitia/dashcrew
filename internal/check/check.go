// Package check is the read-only merge gate for a pull request: the toolkit is current and the PR body has every
// heading of the generated template. It runs locally or as a required GitHub check.
package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

// PrCheck checks one project and one pull request body.
type PrCheck struct {
	Catalog    *catalog.Catalog
	ProjectDir string
	// GuardSource is passed to the doctor; see doctor.Doctor.
	GuardSource string
	// Environment supplies GITHUB_EVENT_PATH, whose pull request event carries the PR body in CI.
	Environment map[string]string
}

// Run returns the toolkit checks of the doctor, without the machine checks, then `pr-sections`.
// prBody wins; otherwise the body of the pull request event in CI; without either the section check is skipped.
// An event file that is not JSON is an error.
func (c PrCheck) Run(prBody *string) ([]doctor.Check, error) {
	checks := doctor.Doctor{Catalog: c.Catalog, ProjectDir: c.ProjectDir, GuardSource: c.GuardSource}.Run()
	body := prBody
	if body == nil {
		var err error
		if body, err = c.eventBody(); err != nil {
			return nil, err
		}
	}
	return append(checks, c.sections(body)), nil
}

func (c PrCheck) eventBody() (*string, error) {
	path, ok := c.Environment["GITHUB_EVENT_PATH"]
	if !ok {
		return nil, nil
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	event, err := dartjson.Decode(string(text))
	if err != nil {
		return nil, fmt.Errorf("GITHUB_EVENT_PATH %s: %w", path, err)
	}
	object, ok := event.(*omap.Map[any])
	if !ok {
		return nil, nil
	}
	pullRequest, ok := object.Value("pull_request").(*omap.Map[any])
	if !ok {
		return nil, nil
	}
	body, _ := pullRequest.Value("body").(string)
	return &body, nil
}

func (c PrCheck) sections(body *string) doctor.Check {
	const id = "pr-sections"
	template, err := os.ReadFile(filepath.Join(c.ProjectDir, filepath.FromSlash(sync.PrTemplatePath)))
	if err != nil {
		return doctor.Check{ID: id, Status: doctor.Fail, Detail: sync.PrTemplatePath + " missing; run dashcrew sync"}
	}
	if body == nil {
		return doctor.Check{ID: id, Status: doctor.Skip, Detail: "no PR body; pass --pr-body <file> or run on a pull_request event"}
	}
	var headings []string
	for _, line := range lines(string(template)) {
		if strings.HasPrefix(line, "## ") {
			headings = append(headings, strings.TrimSpace(line))
		}
	}
	present := map[string]bool{}
	for _, line := range strings.Split(*body, "\n") {
		present[strings.TrimSpace(line)] = true
	}
	var missing []string
	for _, heading := range headings {
		if !present[heading] {
			missing = append(missing, heading)
		}
	}
	if len(missing) > 0 {
		return doctor.Check{ID: id, Status: doctor.Fail, Detail: "missing: " + strings.Join(missing, ", ") + ` (write "None" under a heading that does not apply)`}
	}
	return doctor.Check{ID: id, Status: doctor.OK, Detail: strconv.Itoa(len(headings)) + " required sections present"}
}

// lines splits like Dart's `readAsLinesSync`: at "\n", "\r\n" or a lone "\r", with no empty last line.
func lines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
