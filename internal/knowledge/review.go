package knowledge

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TemplatePath is the page template a missing target page is created from.
const TemplatePath = "templates/page.md"

// Review walks the clone's inbox one proposal at a time and asks what to do with each: accept merges it into its
// target page with a verified marker and removes it, edit runs Edit on it and asks again, skip leaves it, delete
// removes it. A malformed proposal is reported and left in place. It never runs git.
type Review struct {
	Clone string
	// Now dates the verified markers.
	Now func() time.Time
	// Ask prints a prompt and returns the answer; false at the end of input, which ends the walk.
	Ask func(prompt string) (string, bool)
	// Edit opens a proposal in the user's editor.
	Edit func(path string) error
	Out  func(line string)
}

// ReviewResult counts what the walk did.
type ReviewResult struct {
	Accepted, Skipped, Deleted, Malformed int
}

// Inbox lists the proposals in the clone's inbox, slash-separated and oldest first: names start with the date.
func Inbox(clone string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(clone, InboxDir))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			out = append(out, InboxDir+"/"+entry.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// Run walks the inbox, regenerates README.md, and prints the git commands that turn the change into a pull request.
func (r Review) Run() (ReviewResult, error) {
	var result ReviewResult
	proposals, err := Inbox(r.Clone)
	if errors.Is(err, fs.ErrNotExist) {
		return result, fmt.Errorf("%s has no inbox/; is it a knowledge clone? Run dashcrew knowledge pull", r.Clone)
	}
	if err != nil {
		return result, err
	}
	date := r.Now().UTC().Format("2006-01-02")
	if len(proposals) == 0 {
		r.Out("inbox/ is empty")
	}
	// created are the pages this run made from the template; their sections no proposal filled are pruned.
	var created []string
	for i, path := range proposals {
		label := "[" + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(proposals)) + "] " + path
		done, err := r.review(label, path, date, &result, &created)
		if err != nil {
			return result, err
		}
		if !done {
			r.Out("no more answers; " + plural(len(proposals)-i, "proposal") + " left in inbox/")
			break
		}
	}

	for _, page := range created {
		path := filepath.Join(r.Clone, filepath.FromSlash(page))
		data, err := os.ReadFile(path)
		if err != nil {
			return result, err
		}
		if pruned := PruneEmptySections(string(data)); pruned != string(data) {
			if err := os.WriteFile(path, []byte(pruned), 0o644); err != nil {
				return result, err
			}
		}
	}

	readmeChanged, err := r.updateReadme()
	if err != nil {
		return result, err
	}
	r.Out(fmt.Sprintf("accepted %d, skipped %d, deleted %d, malformed %d",
		result.Accepted, result.Skipped, result.Deleted, result.Malformed))
	if result.Accepted == 0 && result.Deleted == 0 && !readmeChanged {
		r.Out("nothing to commit")
		return result, nil
	}
	branch := "knowledge-review-" + date
	r.Out("next: commit on a branch and open a pull request, so the team reviews the change:")
	for _, line := range []string{
		"checkout -b " + branch,
		"add -A",
		`commit -m "knowledge: review inbox ` + date + `"`,
		"push -u origin " + branch,
	} {
		r.Out("  git -C " + r.Clone + " " + line)
	}
	return result, nil
}

// review asks about one proposal until it is accepted, skipped, deleted or found malformed; false at the end of input.
func (r Review) review(label, path, date string, result *ReviewResult, created *[]string) (bool, error) {
	file := filepath.Join(r.Clone, filepath.FromSlash(path))
	for {
		data, err := os.ReadFile(file)
		if err != nil {
			return false, err
		}
		p, err := ParseProposal(string(data))
		if err != nil {
			r.Out(label + " is malformed, left in inbox/: " + err.Error())
			result.Malformed++
			return true, nil
		}
		r.Out(label)
		r.Out(fmt.Sprintf("  into %s under ## %s, from %s@%s on %s", p.PagePath(), p.Heading, p.Repo, p.ShortCommit(), p.Branch))
		for _, line := range strings.Split(p.Content, "\n") {
			r.Out(strings.TrimRight("  "+line, " "))
		}
		for answered := false; !answered; {
			answer, ok := r.Ask("accept, edit, skip or delete? [a/e/s/d] ")
			if !ok {
				return false, nil
			}
			switch strings.ToLower(strings.TrimSpace(answer)) {
			case "a", "accept":
				message, isNew, err := r.accept(p, date)
				if err != nil {
					return false, err
				}
				if isNew {
					*created = append(*created, p.PagePath())
				}
				if err := os.Remove(file); err != nil {
					return false, err
				}
				r.Out(message)
				result.Accepted++
				return true, nil
			case "e", "edit":
				if r.Edit == nil {
					r.Out("edit failed: no editor")
					continue
				}
				if err := r.Edit(file); err != nil {
					r.Out("edit failed: " + err.Error())
					continue
				}
				answered = true
			case "s", "skip":
				r.Out("skipped")
				result.Skipped++
				return true, nil
			case "d", "delete":
				if err := os.Remove(file); err != nil {
					return false, err
				}
				r.Out("deleted " + path)
				result.Deleted++
				return true, nil
			default:
				r.Out("answer a, e, s or d")
			}
		}
	}
}

// accept merges p into its page, creating the page from the template when it is missing; created tells which.
func (r Review) accept(p Proposal, date string) (message string, created bool, err error) {
	page := filepath.Join(r.Clone, filepath.FromSlash(p.PagePath()))
	var text string
	data, err := os.ReadFile(page)
	switch {
	case err == nil:
		text = string(data)
		message = "merged into " + p.PagePath() + " under ## " + p.Heading
	case errors.Is(err, fs.ErrNotExist):
		template, err := os.ReadFile(filepath.Join(r.Clone, filepath.FromSlash(TemplatePath)))
		source := TemplatePath
		if errors.Is(err, fs.ErrNotExist) {
			template, err, source = []byte(pageTemplate), nil, "the built-in template"
		}
		if err != nil {
			return "", false, err
		}
		text = NewPage(string(template), p)
		message = "created " + p.PagePath() + " from " + source + ", merged under ## " + p.Heading
		created = true
		if err := os.MkdirAll(filepath.Dir(page), 0o755); err != nil {
			return "", false, err
		}
	default:
		return "", false, err
	}
	return message, created, os.WriteFile(page, []byte(Merge(text, p, date)), 0o644)
}

// updateReadme rewrites the index in README.md; true when it changed.
func (r Review) updateReadme() (bool, error) {
	path := filepath.Join(r.Clone, "README.md")
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	text, err := UpdateReadme(string(old), r.Clone)
	if err != nil {
		return false, err
	}
	if text == string(old) {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return false, err
	}
	r.Out("wrote README.md")
	return true, nil
}
