// Package ledger writes the release ledger, `<plans>/<version>/README.md` (`dashcrew release status`): the tasks
// planned for a release, the ports that landed on its line and the ports still pending. It only reads git.
package ledger

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/commit"
	"github.com/fahnaladitia/dashcrew/internal/lines"
	"github.com/fahnaladitia/dashcrew/internal/ports"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

// Marker opens every ledger; the file is regenerated whole, so a hand edit is lost.
const Marker = "<!-- dashcrew:release-status - regenerate with dashcrew release status <version>; never edit by hand -->"

// Status renders the ledger of version. all is lines.List's result.
func Status(dir string, p *profile.Profile, all []lines.Line, version string) (string, error) {
	plans, _ := p.Paths.Value("plans").(string)
	prs, _ := p.Paths.Value("pr").(string)
	var b strings.Builder
	b.WriteString(Marker + "\n# Release " + version + "\n\n")

	index := -1
	for i := range all {
		if all[i].Kind != profile.MainLine && all[i].Version == version {
			index = i
			break
		}
	}
	if index < 0 {
		b.WriteString("No line branch holds " + version + "; only its tasks are listed.\n")
	} else {
		line := all[index]
		_, label := lines.Previous(dir, all, line)
		since := ""
		if label != "" {
			since = ", since `" + label + "`"
		}
		b.WriteString(fmt.Sprintf("Line: `%s` (%s)%s.\n", line.Branch, line.Kind, since))
	}

	b.WriteString("\n## Tasks\n\n")
	tasks, err := tasks(dir, plans, version)
	if err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		b.WriteString("No task folders in `" + path.Join(plans, version) + "/`.\n")
	} else {
		b.WriteString("| Task | Branch | PR description |\n|---|---|---|\n")
		for _, task := range tasks {
			branch := branchOf(dir, task, version)
			if branch == "" {
				b.WriteString("| `" + task + "` | none found | none |\n")
				continue
			}
			pr := "`" + path.Join(prs, branch) + ".md`"
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(prs), filepath.FromSlash(branch)+".md")); err != nil {
				pr += " (not written yet)"
			}
			b.WriteString("| `" + task + "` | `" + branch + "` | " + pr + " |\n")
		}
	}

	b.WriteString("\n## Ported here\n\n")
	if index < 0 {
		b.WriteString("None: no line branch.\n")
	} else {
		ported, err := portedOn(dir, p.Commit, all, all[index])
		if err != nil {
			return "", err
		}
		if len(ported) == 0 {
			b.WriteString("None.\n")
		}
		for _, line := range ported {
			b.WriteString("- " + line + "\n")
		}
	}

	b.WriteString("\n## Pending ports\n\n")
	if index < 0 {
		b.WriteString("None: no line branch.\n")
		return b.String(), nil
	}
	var pairs [][2]lines.Line
	if index > 0 {
		pairs = append(pairs, [2]lines.Line{all[index-1], all[index]})
	}
	if index+1 < len(all) {
		pairs = append(pairs, [2]lines.Line{all[index], all[index+1]})
	}
	if len(pairs) == 0 {
		b.WriteString("None: no other line.\n")
	}
	for _, pair := range pairs {
		result, err := ports.Check(dir, p.Commit, all, pair[0], pair[1])
		if err != nil {
			return "", err
		}
		name := "`" + pair[0].Branch + " -> " + pair[1].Branch + "`"
		if result.Problems() == 0 {
			b.WriteString("- " + name + ": none pending\n")
			continue
		}
		for _, fix := range result.Fixes {
			if fix.Status != ports.Missing && fix.Status != ports.Lost {
				continue
			}
			text := fix.Status.String() + " `" + short(fix.Hash) + "` " + fix.Subject
			if fix.Status == ports.Lost {
				text += " (reverted by `" + short(fix.RevertedBy) + "`)"
			}
			b.WriteString("- " + name + ": " + text + "\n")
		}
	}
	return b.String(), nil
}

func short(sha string) string { return sha[:min(7, len(sha))] }

// tasks are the folder names under `<plans>/<version>/`, sorted.
func tasks(dir, plans, version string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(plans), version))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() {
			out = append(out, entry.Name())
		}
	}
	return out, nil
}

// branchOf is the local branch of a task folder: a branch named task, or ending in `/<task>`; one that records
// version wins. Empty when none.
func branchOf(dir, task, version string) string {
	out, err := exec.Command("git", "-C", dir, "for-each-ref", "--format=%(refname:short)", "refs/heads").Output()
	if err != nil {
		return ""
	}
	var matches []string
	for _, branch := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if branch == task || strings.HasSuffix(branch, "/"+task) {
			matches = append(matches, branch)
		}
	}
	sort.Strings(matches)
	for _, branch := range matches {
		if lines.RecordedRelease(dir, branch) == version {
			return branch
		}
	}
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// portedOn lists the commits of line since the previous release that carry `Ported-from:`, merges included.
func portedOn(dir string, cfg *profile.Commit, all []lines.Line, line lines.Line) ([]string, error) {
	ref, _ := lines.Previous(dir, all, line)
	revRange := line.Head
	if ref != "" {
		revRange = ref + ".." + line.Head
	}
	out, err := exec.Command("git", "-C", dir, "log", "--reverse", "-z", "--format=%H%n%B", revRange, "--").Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", revRange, err)
	}
	var found []string
	for _, entry := range strings.Split(string(out), "\x00") {
		hash, message, ok := strings.Cut(entry, "\n")
		if !ok || hash == "" {
			continue
		}
		parsed := commit.Parse(cfg, message)
		if from := parsed.Values(commit.PortedFromTrailer); len(from) > 0 {
			found = append(found, "`"+short(hash)+"` "+parsed.Subject+" (Ported-from: "+strings.Join(from, ", ")+")")
		}
	}
	return found, nil
}

// Write replaces `<plans>/<version>/README.md` and nothing else; it returns the path relative to dir.
func Write(dir, plans, version, text string) (string, error) {
	if version == "" || version == "." || version == ".." || strings.ContainsAny(version, `/\`) {
		return "", fmt.Errorf("release %q is not a folder name", version)
	}
	relative := path.Join(plans, version, "README.md")
	target := filepath.Join(dir, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	return relative, os.WriteFile(target, []byte(text), 0o644)
}
