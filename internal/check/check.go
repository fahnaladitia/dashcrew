// Package check is the read-only merge gate for a pull request: the toolkit is current, the PR body has every
// heading of the generated template and every commit follows `project.commit`. It runs locally or as a required
// GitHub check.
package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/commit"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

// PrCheck checks one project and one pull request body.
type PrCheck struct {
	Catalog    *catalog.Catalog
	ProjectDir string
	// Environment supplies GITHUB_EVENT_PATH, whose pull request event carries the PR body and base in CI.
	Environment map[string]string
	// Base is `--base`: the commits of `<base>..HEAD` are linted. Without it, the pull request event's base and head.
	Base *string
}

// Run returns the toolkit checks of the doctor, without the machine checks, then `pr-sections` and `commits`.
// prBody wins; otherwise the body of the pull request event in CI; without either the section check is skipped.
// An event file that is not JSON is an error.
func (c PrCheck) Run(prBody *string) ([]doctor.Check, error) {
	checks := doctor.Doctor{Catalog: c.Catalog, ProjectDir: c.ProjectDir}.Run()
	pullRequest, err := c.pullRequest()
	if err != nil {
		return nil, err
	}
	body := prBody
	if body == nil && pullRequest != nil {
		text, _ := pullRequest.Value("body").(string)
		body = &text
	}
	return append(checks, c.sections(body), c.commits(pullRequest)), nil
}

// pullRequest is the `pull_request` object of the event in CI; nil outside a pull request event.
func (c PrCheck) pullRequest() (*omap.Map[any], error) {
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
	pullRequest, _ := object.Value("pull_request").(*omap.Map[any])
	return pullRequest, nil
}

// commits lints the PR's commits, merges skipped: `<base>..HEAD` with --base, else the event's base and head sha.
func (c PrCheck) commits(pullRequest *omap.Map[any]) doctor.Check {
	const id = "commits"
	revRange, shown := "", ""
	switch {
	case c.Base != nil:
		revRange = *c.Base + "..HEAD"
		shown = revRange
	case pullRequest != nil:
		baseSha, headSha := sha(pullRequest, "base"), sha(pullRequest, "head")
		if baseSha != "" && headSha != "" {
			revRange = baseSha + ".." + headSha
			shown = baseSha[:min(7, len(baseSha))] + ".." + headSha[:min(7, len(headSha))]
		}
	}
	if revRange == "" {
		return doctor.Check{ID: id, Status: doctor.Skip, Detail: "no base; pass --base <branch> or run on a pull_request event"}
	}
	p, err := sync.Sync{Catalog: c.Catalog, ProjectDir: c.ProjectDir}.Profile()
	if err != nil {
		return doctor.Check{ID: id, Status: doctor.Skip, Detail: "needs a valid profile"}
	}
	repo, err := commit.Open(c.ProjectDir)
	if err != nil {
		return doctor.Check{ID: id, Status: doctor.Fail, Detail: err.Error()}
	}
	problems, count, err := commit.LintRange(repo, p.Commit, revRange)
	if err != nil {
		return doctor.Check{ID: id, Status: doctor.Fail, Detail: strings.ReplaceAll(err.Error(), revRange, shown) + "; a CI checkout needs fetch-depth: 0"}
	}
	if len(problems) > 0 {
		return doctor.Check{ID: id, Status: doctor.Fail, Detail: strings.Join(problems, "; ")}
	}
	return doctor.Check{ID: id, Status: doctor.OK, Detail: commit.Follow(count, shown, p.Commit.Pattern)}
}

// sha is `pull_request.<side>.sha` of the event; empty when missing.
func sha(pullRequest *omap.Map[any], side string) string {
	object, _ := pullRequest.Value(side).(*omap.Map[any])
	if object == nil {
		return ""
	}
	value, _ := object.Value("sha").(string)
	return value
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
