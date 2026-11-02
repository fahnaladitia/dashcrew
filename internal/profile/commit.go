package profile

import (
	"regexp"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// Commit is `project.commit`: the message pattern `dashcrew commit suggest` builds and `dashcrew commit lint` checks.
type Commit struct {
	// Pattern holds the placeholders <type>, <scope>, <subject> and <ticket>; <subject> is required.
	Pattern string
	Types   []string
	// TypeFromBranch maps a `project.branch_prefix` key to a type.
	TypeFromBranch *omap.Map[string]
	// Scopes maps a path glob to a scope in profile order; a file takes the first glob it matches.
	Scopes *omap.Map[string]
	// Ticket is nil without `ticket`.
	Ticket     *Ticket
	SubjectMax int
	Language   string
	// Body is optional, required or none.
	Body string
	// Trailers are allowed on top of Co-authored-by, Signed-off-by, Reviewed-by and the ticket trailer.
	Trailers []string
	// AllowEmailTrailers are the only trailers whose value may hold an email address (a DCO `Signed-off-by`, say);
	// an email anywhere else fails lint.
	AllowEmailTrailers []string
	// PortTrailers allows `Ported-from: <sha>` and `Port: yes` and checks their shape; set by `project.release_lines`.
	PortTrailers bool
}

// Ticket reads a ticket id from the branch name: the first group of FromBranch, else the whole match.
type Ticket struct {
	FromBranch string
	// Trailer is the trailer the ticket goes in; empty when only the pattern's <ticket> carries it.
	Trailer string
}

var (
	commitKeys        = []string{"pattern", "types", "type_from_branch", "scopes", "ticket", "subject_max", "language", "body", "trailers", "allow_email_trailers"}
	commitPlaceholder = regexp.MustCompile(`<([^<>\s]*)>`)
	commitWord        = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	scopeWord         = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	// TrailerToken is a git trailer key.
	TrailerToken = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*$`)
	// CommitPlaceholders are the placeholders a pattern may hold, each at most once.
	CommitPlaceholders = []string{"type", "scope", "subject", "ticket"}
	commitBodies       = []string{"optional", "required", "none"}
)

// DefaultCommitTypes are `project.commit.types` when the profile leaves them out.
var DefaultCommitTypes = []string{"feat", "fix", "refactor", "test", "docs", "chore", "perf", "build", "ci"}

// parseCommit reads `project.commit` with strict keys and removes it from project, so the template context stays as
// it was. Default type_from_branch entries whose branch prefix or type the profile lacks are dropped, not errors.
func parseCommit(project *omap.Map[any]) (*Commit, error) {
	value := project.Value("commit")
	project.Delete("commit")
	config := omap.New[any]()
	if value != nil {
		var ok bool
		if config, ok = value.(*omap.Map[any]); !ok {
			return nil, fail("project.commit", "expected a map")
		}
	}
	for _, key := range config.Keys() {
		if !contains(commitKeys, key) {
			return nil, fail("project.commit."+key, "unknown key")
		}
	}
	commit := &Commit{Pattern: "<type>(<scope>): <subject>", Types: DefaultCommitTypes, SubjectMax: 72, Language: "en", Body: "optional",
		Scopes: omap.New[string](), Trailers: []string{}, AllowEmailTrailers: []string{}}

	if config.Has("pattern") {
		commit.Pattern, _ = config.Value("pattern").(string)
	}
	if config.Has("types") {
		types, ok := StringList(config.Value("types"))
		if !ok || len(types) == 0 || !all(types, commitWord) {
			return nil, fail("project.commit.types", "required, a non-empty list of words")
		}
		commit.Types = types
	}
	var err error
	if commit.TypeFromBranch, err = parseTypeFromBranch(config, commit.Types, project.Value("branch_prefix")); err != nil {
		return nil, err
	}
	if config.Has("scopes") {
		scopes, ok := config.Value("scopes").(*omap.Map[any])
		if !ok {
			return nil, fail("project.commit.scopes", "expected a map of path glob to scope")
		}
		for _, glob := range scopes.Keys() {
			scope, ok := scopes.Value(glob).(string)
			if _, err := GlobRegexp(glob); err != nil || glob == "" {
				return nil, fail("project.commit.scopes."+glob, "expected a path glob")
			}
			if !ok || !scopeWord.MatchString(scope) {
				return nil, fail("project.commit.scopes."+glob, "expected a scope word")
			}
			commit.Scopes.Set(glob, scope)
		}
	}
	if commit.Ticket, err = parseTicket(config.Value("ticket"), config.Has("ticket"), commit.Pattern); err != nil {
		return nil, err
	}
	if err := checkPattern(commit.Pattern, commit.Ticket != nil); err != nil {
		return nil, err
	}
	if config.Has("subject_max") {
		max, ok := config.Value("subject_max").(int)
		if !ok || max < 1 {
			return nil, fail("project.commit.subject_max", "expected a positive number")
		}
		commit.SubjectMax = max
	}
	if config.Has("language") {
		language, ok := config.Value("language").(string)
		if !ok || strings.TrimSpace(language) == "" {
			return nil, fail("project.commit.language", "expected a language code")
		}
		commit.Language = language
	}
	if config.Has("body") {
		body, ok := config.Value("body").(string)
		if !ok || !contains(commitBodies, body) {
			return nil, fail("project.commit.body", "expected one of "+strings.Join(commitBodies, ", "))
		}
		commit.Body = body
	}
	if config.Has("trailers") {
		trailers, ok := StringList(config.Value("trailers"))
		if !ok || !all(trailers, TrailerToken) {
			return nil, fail("project.commit.trailers", "expected a list of trailer tokens")
		}
		commit.Trailers = trailers
	}
	if config.Has("allow_email_trailers") {
		trailers, ok := StringList(config.Value("allow_email_trailers"))
		if !ok || !all(trailers, TrailerToken) {
			return nil, fail("project.commit.allow_email_trailers", "expected a list of trailer tokens")
		}
		commit.AllowEmailTrailers = trailers
	}
	return commit, nil
}

func parseTypeFromBranch(config *omap.Map[any], types []string, branchPrefix any) (*omap.Map[string], error) {
	prefixes, _ := branchPrefix.(*omap.Map[any])
	hasPrefix := func(key string) bool { return prefixes != nil && prefixes.Has(key) }
	out := omap.New[string]()
	if !config.Has("type_from_branch") {
		for _, pair := range [][2]string{{"feat", "feat"}, {"fix", "fix"}, {"hotfix", "fix"}} {
			if hasPrefix(pair[0]) && contains(types, pair[1]) {
				out.Set(pair[0], pair[1])
			}
		}
		return out, nil
	}
	mapping, ok := config.Value("type_from_branch").(*omap.Map[any])
	if !ok {
		return nil, fail("project.commit.type_from_branch", "expected a map of branch_prefix key to type")
	}
	for _, key := range mapping.Keys() {
		value, _ := mapping.Value(key).(string)
		if !hasPrefix(key) {
			return nil, fail("project.commit.type_from_branch."+key, `no branch_prefix "`+key+`"`)
		}
		if !contains(types, value) {
			return nil, fail("project.commit.type_from_branch."+key, `type "`+value+`" is not in types`)
		}
		out.Set(key, value)
	}
	return out, nil
}

func parseTicket(value any, present bool, pattern string) (*Ticket, error) {
	if !present {
		return nil, nil
	}
	config, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fail("project.commit.ticket", "expected a map with from_branch and an optional trailer")
	}
	for _, key := range config.Keys() {
		if key != "from_branch" && key != "trailer" {
			return nil, fail("project.commit.ticket."+key, "unknown key")
		}
	}
	from, ok := config.Value("from_branch").(string)
	if _, err := regexp.Compile(from); !ok || from == "" || err != nil {
		return nil, fail("project.commit.ticket.from_branch", "required, a regular expression")
	}
	ticket := &Ticket{FromBranch: from}
	if !strings.Contains(pattern, "<ticket>") {
		ticket.Trailer = "Refs"
	}
	if config.Has("trailer") {
		trailer, ok := config.Value("trailer").(string)
		if !ok || !TrailerToken.MatchString(trailer) {
			return nil, fail("project.commit.ticket.trailer", "expected a trailer token")
		}
		ticket.Trailer = trailer
	}
	return ticket, nil
}

func checkPattern(pattern string, hasTicket bool) error {
	if !strings.Contains(pattern, "<subject>") {
		return fail("project.commit.pattern", "required, a pattern with <subject>")
	}
	seen := map[string]bool{}
	for _, match := range commitPlaceholder.FindAllStringSubmatch(pattern, -1) {
		name := match[1]
		if !contains(CommitPlaceholders, name) {
			return fail("project.commit.pattern", "unknown placeholder <"+name+">, expected type, scope, subject or ticket")
		}
		if seen[name] {
			return fail("project.commit.pattern", "<"+name+"> appears twice")
		}
		seen[name] = true
	}
	if seen["ticket"] && !hasTicket {
		return fail("project.commit.pattern", "<ticket> needs ticket.from_branch")
	}
	return nil
}

// GlobRegexp compiles a path glob: `**` crosses folders, `*` and `?` stay within one.
func GlobRegexp(glob string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch {
		case strings.HasPrefix(glob[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			b.WriteString(".*")
			i++
		case glob[i] == '*':
			b.WriteString("[^/]*")
		case glob[i] == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(glob[i : i+1]))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func all(list []string, pattern *regexp.Regexp) bool {
	for _, item := range list {
		if !pattern.MatchString(item) {
			return false
		}
	}
	return true
}
