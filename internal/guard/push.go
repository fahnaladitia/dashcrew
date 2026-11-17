package guard

import (
	"errors"
	"slices"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// PushRule is `push` in `.ai/guard.json`, written for `policy.agent_push`: the agent may push a branch it names,
// never a protected one (`project.base_branches`), never by force, never a deletion and never everything at once.
// A push that names no branch is blocked too: the guard cannot see which branch is checked out after a `cd`.
type PushRule struct {
	Protected []string
}

const nameTheBranch = "git push must name the task branch: git push -u origin <branch>."

// check is the block reason for `git push` with args (the words after `push`), or empty when it may run.
func (r *PushRule) check(args []string) string {
	var positional []string
	for i := 0; i < len(args); i++ {
		word := args[i]
		switch {
		case word == "--":
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(word, "--"):
			name, _, hasValue := strings.Cut(word[2:], "=")
			switch name {
			case "force", "force-with-lease", "force-if-includes":
				return reserved("git push --force")
			case "delete":
				return reserved("deleting a remote branch")
			case "all", "branches", "mirror", "tags", "prune":
				return reserved("git push --" + name)
			case "repo", "receive-pack", "exec", "push-option":
				if !hasValue {
					i++
				}
			}
		case strings.HasPrefix(word, "-") && len(word) > 1:
			cluster := word[1:]
			for j, letter := range cluster {
				if letter == 'f' {
					return reserved("git push --force")
				}
				if letter == 'd' {
					return reserved("deleting a remote branch")
				}
				if letter == 'o' {
					if j == len(cluster)-1 {
						i++
					}
					break
				}
			}
		default:
			positional = append(positional, word)
		}
	}
	if len(positional) < 2 {
		return nameTheBranch
	}
	for _, refspec := range positional[1:] {
		if reason := r.refspec(refspec); reason != "" {
			return reason
		}
	}
	return ""
}

// refspec is the block reason for one `<src>:<dst>` (or `<branch>`) of a push, or empty.
func (r *PushRule) refspec(refspec string) string {
	if strings.HasPrefix(refspec, "+") {
		return reserved("git push --force")
	}
	source, destination, hasColon := strings.Cut(refspec, ":")
	if !hasColon {
		destination = source
	} else if source == "" {
		return reserved("deleting a remote branch")
	}
	if strings.Contains(refspec, "*") || (strings.HasPrefix(destination, "refs/") && !strings.HasPrefix(destination, "refs/heads/")) {
		return "git push may send only a branch, named outright. " + nameTheBranch
	}
	branch := strings.TrimPrefix(destination, "refs/heads/")
	if branch == "HEAD" || branch == "@" {
		return nameTheBranch
	}
	if slices.Contains(r.Protected, branch) {
		return "pushing to " + branch + " is reserved for the user. Push your task branch: git push -u origin <branch>."
	}
	return ""
}

func reserved(what string) string { return what + " is reserved for the user. Ask them to run it." }

// toJSON is the `push` object of `.ai/guard.json`.
func (r *PushRule) toJSON() *omap.Map[any] {
	return omap.Of[any]("protected", toAny(r.Protected))
}

// pushRuleOf reads the optional `push` key; nil when absent, an error when it is not `{"protected": [strings]}`.
func pushRuleOf(object *omap.Map[any]) (*PushRule, error) {
	raw := object.Value("push")
	if raw == nil {
		return nil, nil
	}
	push, ok := raw.(*omap.Map[any])
	if !ok {
		return nil, errors.New("push is not a map")
	}
	protected, ok := stringItems(push.Value("protected"))
	if !ok {
		return nil, errors.New("push.protected is not a list of strings")
	}
	return &PushRule{Protected: protected}, nil
}
