// Package guardinit is `dashcrew guard init`: the guard alone, without a profile. It writes the default policy, the
// shim and each agent's hook entry, the same files and entries sync writes, so a later sync takes them over.
package guardinit

import (
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/filemode"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/projectfiles"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

// Init writes the guard of one project for Agents.
type Init struct {
	ProjectDir string
	Agents     []string
	// Force replaces a policy or shim that guard init did not write.
	Force bool
}

// Error is a refusal; nothing was written.
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

// Run writes what is missing or different and returns those files, relative to the project, in writing order.
// It refuses, writing nothing, in a project with a profile, for an agent dashcrew does not support, and over a
// policy or shim it did not write unless Force.
func (i Init) Run() ([]string, error) {
	project := projectfiles.New(i.ProjectDir)
	if project.Exists(".ai/project.yaml") {
		return nil, &Error{"This project has .ai/project.yaml: dashcrew sync writes and updates its guard."}
	}
	var selected []string
	seen := map[string]bool{}
	minimum := guard.InitPolicy().Minimum()
	for _, agent := range i.Agents {
		if seen[agent] {
			continue
		}
		seen[agent] = true
		if adapters.HookFile(agent) == "" {
			return nil, &Error{adapters.Unsupported(agent).Error()}
		}
		selected = append(selected, agent)
		minimum = higher(minimum, adapters.GuardMinimum(agent))
	}

	var foreign []string
	for _, path := range []string{guard.InitPolicyPath, guard.ShimPath} {
		if !project.Exists(path) {
			continue
		}
		content, err := project.Read(path)
		if err != nil {
			return nil, err
		}
		if !guard.WrittenByInit(path, content) {
			foreign = append(foreign, path)
		} else if path == guard.ShimPath {
			// An agent of an earlier run may need a newer guard than the agents of this one.
			minimum = higher(minimum, guard.ShimMinimum(content))
		}
	}
	if len(foreign) > 0 && !i.Force {
		return nil, &Error{"Not written by dashcrew guard init (rerun with --force to overwrite):\n  " + strings.Join(foreign, "\n  ")}
	}

	var written []string
	for _, file := range []struct{ path, content string }{
		{guard.InitPolicyPath, guard.InitPolicyText()},
		{guard.ShimPath, guard.Shim(minimum)},
	} {
		changed, err := project.WriteIfChanged(file.path, file.content)
		if err != nil {
			return written, err
		}
		if changed {
			written = append(written, file.path)
		}
	}
	if err := filemode.SetExecutable(project.Path(guard.ShimPath), true); err != nil {
		return written, err
	}
	for _, agent := range selected {
		path, changed, err := adapters.MergeGuardHook(agent, i.ProjectDir)
		if err != nil {
			return written, err
		}
		if changed {
			written = append(written, path)
		}
	}
	return written, nil
}

func higher(a, b string) string {
	if version.Compare(b, a) > 0 {
		return b
	}
	return a
}

// trustSteps are what an agent needs from its user before it runs a project's hooks.
var trustSteps = map[string]string{
	"codex":       "Codex: trust the project when Codex asks, then approve the dashcrew hook in /hooks; until both, Codex ignores .codex/.",
	"antigravity": "Antigravity: trust the folder when agy asks; agy loads .agents/hooks.json only in a trusted folder.",
}

// Notes are the trust steps of the agents that need one, in the order given.
func Notes(selected []string) []string {
	var notes []string
	for _, agent := range selected {
		if step, ok := trustSteps[agent]; ok {
			notes = append(notes, step)
		}
	}
	return notes
}
