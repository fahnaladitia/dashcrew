package commit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

// DeriveType maps the branch to a type through `project.branch_prefix` (the longest matching prefix wins) and
// `type_from_branch`. An empty type comes with the reason the member reads.
func DeriveType(cfg *profile.Commit, branchPrefix *omap.Map[any], branch string) (string, string) {
	key, longest := "", 0
	if branchPrefix != nil {
		for _, name := range branchPrefix.Keys() {
			prefix, _ := branchPrefix.Value(name).(string)
			if prefix != "" && strings.HasPrefix(branch, prefix) && len(prefix) > longest {
				key, longest = name, len(prefix)
			}
		}
	}
	if typ, ok := cfg.TypeFromBranch.Get(key); ok && key != "" {
		return typ, "branch " + branch
	}
	if branch == "" {
		return "", "no branch is checked out; pass --type"
	}
	return "", "no branch prefix of " + branch + " maps to a type; pass --type"
}

// DeriveScope gives each file the scope of the first glob it matches and returns the scope with the most files;
// a tie goes to the scope listed first. No match drops the scope.
func DeriveScope(cfg *profile.Commit, files []string) (string, string) {
	if cfg.Scopes.Len() == 0 {
		return "", "no scopes in project.commit"
	}
	type tally struct {
		count int
		globs []string
	}
	counts := map[string]*tally{}
	var order []string
	for _, glob := range cfg.Scopes.Keys() {
		if scope := cfg.Scopes.Value(glob); counts[scope] == nil {
			counts[scope] = &tally{}
			order = append(order, scope)
		}
	}
	for _, file := range files {
		for _, glob := range cfg.Scopes.Keys() {
			if re, err := profile.GlobRegexp(glob); err == nil && re.MatchString(file) {
				t := counts[cfg.Scopes.Value(glob)]
				t.count++
				if !contains(t.globs, glob) {
					t.globs = append(t.globs, glob)
				}
				break
			}
		}
	}
	best := ""
	for _, scope := range order {
		if counts[scope].count > 0 && (best == "" || counts[scope].count > counts[best].count) {
			best = scope
		}
	}
	if best == "" {
		return "", "no scope glob matches the changed files"
	}
	var where []string
	for _, glob := range counts[best].globs {
		where = append(where, strings.TrimSuffix(glob, "/**"))
	}
	return best, fmt.Sprintf("%d of %d files under %s", counts[best].count, len(files), strings.Join(where, ", "))
}

// DeriveTicket is the first group of `ticket.from_branch` in the branch, else its whole match; empty without one.
func DeriveTicket(cfg *profile.Commit, branch string) string {
	if cfg.Ticket == nil {
		return ""
	}
	match := regexp.MustCompile(cfg.Ticket.FromBranch).FindStringSubmatch(branch)
	switch {
	case match == nil:
		return ""
	case len(match) > 1 && match[1] != "":
		return match[1]
	}
	return match[0]
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9._/@%+,-][A-Za-z0-9._/@%+=:,-]*$`)

// Quote leaves a word of safe characters bare and single-quotes anything else, which bash and zsh both read
// literally: no `$`, backtick, `!` or glob expansion, and newlines kept.
func Quote(text string) string {
	if shellSafe.MatchString(text) {
		return text
	}
	return QuoteArg(text)
}

// QuoteArg always single-quotes; a single quote inside closes the quotes, adds an escaped quote and reopens.
func QuoteArg(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}
