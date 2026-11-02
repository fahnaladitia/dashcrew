package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/lines"
	"github.com/fahnaladitia/dashcrew/internal/spec"
)

func specCommand(cli cliIO) *command {
	branch := newCommand("spec", "Feature specs (PRD and TRD) in the knowledge repo: scaffold, lint, revise and status. Never runs git write commands.")

	newSpec := newCommand("new", "Write specs/<release>/<feature>/prd.md and trd.md into the knowledge clone from its templates,\n"+
		"with status draft and rev 1. Refuses a folder that exists; writes a missing template first. Runs no git.")
	newSpec.invocation = "dashcrew spec new <feature> [arguments]"
	newSpec.option("project", "p", "Project root holding .ai/project.yaml; its branch gives the default release.", ptr("."))
	newSpec.option("clone", "", "Knowledge clone to write into; defaults to the profile's ~/.dashcrew/knowledge/<name>.", nil)
	newSpec.option("release", "", "Release folder; defaults to the release the current branch records, else unreleased.", nil)
	newSpec.run = func(r *results) (int, error) {
		if len(r.rest) != 1 {
			return 0, &usageError{newSpec, "Pass one feature name, such as notes-sync."}
		}
		clone, projectDir, err := knowledgeClone(r)
		if err != nil {
			return 0, err
		}
		release := spec.Unreleased
		if value := r.option("release"); value != nil {
			release = *value
		} else if recorded := lines.RecordedRelease(projectDir, lines.CurrentBranch(projectDir)); recorded != "" {
			release = recorded
		}
		written, err := spec.New(clone, r.rest[0], release)
		for _, path := range written {
			cli.out("wrote   " + path)
		}
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		cli.out("note: in " + clone + "; fill both files with the spec workflow, then run dashcrew spec lint.")
		return 0, nil
	}

	lint := newCommand("lint", "Check spec files: front matter, IDs (AC-n, D-n, C-n) and the open questions of an approved spec.\n"+
		"With --against, compare with a git ref: a vanished, renumbered or reused ID, and an approved file changed\n"+
		"without a rev increase. Exits 1 on any problem. Writes nothing.")
	lint.invocation = "dashcrew spec lint [<path>] [arguments]"
	lint.option("project", "p", "Project root holding .ai/project.yaml; finds the clone when no <path> is given.", ptr("."))
	lint.option("clone", "", "Knowledge clone whose specs/ to check when no <path> is given; defaults to the profile's.", nil)
	lint.option("against", "", "Git ref to compare each file with, such as origin/main.", nil)
	lint.run = func(r *results) (int, error) {
		if len(r.rest) > 1 {
			return 0, &usageError{lint, "Pass at most one path: a spec file or a folder."}
		}
		var path string
		if len(r.rest) == 1 {
			path = r.rest[0]
		} else {
			clone, _, err := knowledgeClone(r)
			if err != nil {
				return 0, err
			}
			path = filepath.Join(clone, spec.Dir)
		}
		files, err := spec.FilesUnder(path)
		if err != nil {
			return 0, &exitError{1, "No spec file or folder at " + path + "."}
		}
		against := ""
		if value := r.option("against"); value != nil {
			against = *value
		}
		results, err := spec.LintFiles(files, against)
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		problems := printSpecProblems(results, func(where, message string) { cli.out(where + ": " + message) })
		cli.out(fmt.Sprintf("%s checked, %s", count(len(results), "spec file"), count(problems, "problem")))
		if problems > 0 {
			return 1, nil
		}
		return 0, nil
	}

	revise := newCommand("revise", "Set an approved spec back to review and raise rev in prd.md and trd.md, so agents may edit it again.\n"+
		"A member runs it; it prints the git commands for the pull request and never runs them.")
	revise.invocation = "dashcrew spec revise <path>"
	revise.run = func(r *results) (int, error) {
		if len(r.rest) != 1 {
			return 0, &usageError{revise, "Pass the spec folder or one of its files."}
		}
		revised, err := spec.Revise(r.rest[0])
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		for i, file := range revised.Files {
			cli.out(fmt.Sprintf("revised %s: approved rev %d -> review rev %d", file, revised.From[i], revised.Rev))
		}
		feature := filepath.Base(filepath.FromSlash(revised.Folder))
		release := filepath.Base(filepath.Dir(filepath.FromSlash(revised.Folder)))
		branchName := "spec-revise-" + feature + "-rev" + strconv.Itoa(revised.Rev)
		cli.out("next: commit on a branch and open a pull request; its merge with status: approved approves rev " + strconv.Itoa(revised.Rev) + ":")
		for _, line := range []string{
			"checkout -b " + branchName,
			"add " + revised.Folder,
			fmt.Sprintf(`commit -m "spec: revise %s/%s to rev %d"`, release, feature, revised.Rev),
			"push -u origin " + branchName,
		} {
			cli.out("  git -C " + revised.Root + " " + line)
		}
		return 0, nil
	}
	status := newCommand("status", "Compare the spec the current branch records (git config branch.<name>.dashcrew-spec) with the\n"+
		"knowledge clone: its current rev and status, the ACs this repo's plans claim and the ones none claims.\n"+
		"Other repos the spec names are listed as not checked. Writes nothing.")
	status.invocation = "dashcrew spec status [arguments]"
	status.option("project", "p", "Project root holding .ai/project.yaml; its branch and plans are read.", ptr("."))
	status.option("clone", "", "Knowledge clone holding the spec; defaults to the profile's ~/.dashcrew/knowledge/<name>.", nil)
	status.run = func(r *results) (int, error) {
		if len(r.rest) != 0 {
			return 0, &usageError{status, "spec status takes no arguments; it reads the current branch."}
		}
		projectDir, p, err := loadKnowledgeProfile(r)
		if err != nil {
			return 0, err
		}
		clone, _, err := knowledgeClone(r)
		if err != nil {
			return 0, err
		}
		branchName := lines.CurrentBranch(projectDir)
		if branchName == "" {
			return 0, &exitError{1, "Not on a branch; check out the task branch first."}
		}
		value := lines.RecordedSpec(projectDir, branchName)
		if value == "" || value == "none" {
			cli.out(branchName + " records no spec. To link one: git config " + lines.SpecConfigKey(branchName) + " specs/<release>/<feature>@rev<n>")
			return 0, nil
		}
		ref, ok := spec.ParseRef(value)
		if !ok {
			return 0, &exitError{1, lines.SpecConfigKey(branchName) + " is " + value + ", not specs/<release>/<feature>@rev<n>."}
		}
		state, err := spec.Load(clone, ref.Folder)
		if err != nil {
			return 0, &exitError{1, err.Error() + "; run dashcrew knowledge pull, or fix the branch config."}
		}
		plans, _ := p.Paths.Value("plans").(string)
		claims, err := spec.Claims(projectDir, plans, ref.Folder)
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		row := func(label, text string) { cli.out(fmt.Sprintf("%-12s%s", label, text)) }
		row("spec", ref.String()+", recorded on "+branchName)
		current := fmt.Sprintf("rev %d, %s", state.Rev, orNone(state.Status))
		if state.Rev == ref.Rev {
			current += " (same rev as recorded)"
		}
		row("clone", current)
		for _, warning := range spec.Drift(ref, state, lines.OwnCommits(projectDir, branchName)) {
			row("warning", warning)
		}
		inSpec := map[string]bool{}
		var unclaimed []string
		for _, id := range state.ACs {
			inSpec[id] = true
			if len(claims[id]) == 0 {
				unclaimed = append(unclaimed, id)
			}
		}
		ids := make([]string, 0, len(claims))
		for id := range claims {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return acNumber(ids[i]) < acNumber(ids[j]) })
		label := "claimed"
		if len(ids) == 0 {
			row(label, "none; no plan under "+plans+"/ names "+ref.Folder)
		}
		for _, id := range ids {
			text := fmt.Sprintf("%-6s%s", id, strings.Join(claims[id], ", "))
			if !inSpec[id] {
				text += " (not a live AC of the spec)"
			}
			row(label, text)
			label = ""
		}
		if len(unclaimed) == 0 {
			unclaimed = []string{"none"}
		}
		row("unclaimed", strings.Join(unclaimed, ", "))
		names := spec.RepoNames(projectDir)
		var others []string
		for _, repo := range state.Repos {
			if !slices.Contains(names, repo) {
				others = append(others, repo)
			}
		}
		if len(others) > 0 {
			row("not checked", strings.Join(others, ", ")+" (other repos; run spec status there)")
		}
		return 0, nil
	}
	return branch.add(newSpec).add(lint).add(revise).add(status)
}

func orNone(status string) string {
	if status == "" {
		return "no status"
	}
	return status
}

// acNumber is the number of an `AC-n` ID, for sorting.
func acNumber(id string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "AC-"))
	return n
}

// printSpecProblems prints one line per problem through line, where is `<file>[:<line>]`, and returns how many.
func printSpecProblems(results []spec.FileResult, line func(where, message string)) int {
	problems := 0
	for _, result := range results {
		for _, p := range result.Problems {
			where := result.Path
			if p.Line > 0 {
				where += ":" + strconv.Itoa(p.Line)
			}
			line(where, p.Message)
			problems++
		}
	}
	return problems
}

// specsOf lints the clone's specs/ folder for knowledge doctor; no folder or no file is no result.
func specsOf(clone string) ([]spec.FileResult, error) {
	dir := filepath.Join(clone, spec.Dir)
	if _, err := os.Stat(dir); err != nil {
		return nil, nil
	}
	files, err := spec.FilesUnder(dir)
	if err != nil {
		return nil, err
	}
	return spec.LintFiles(files, "")
}
