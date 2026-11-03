package commit

import (
	"fmt"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

// Input is what the member or agent passes to suggest; nil leaves the part to derivation or a placeholder.
type Input struct {
	Subject, Body, Type, Scope *string
	// PortedFrom are the commits a port carries; each adds a `Ported-from:` trailer.
	PortedFrom []string
}

// Suggestion is everything suggest derived, the message and the commands, for text or JSON output.
type Suggestion struct {
	Branch                    string
	Type, TypeReason          string
	Scope, ScopeReason        string
	Ticket, TicketReason      string
	HasTicket                 bool
	Files, Untracked          []string
	SinceVerify               []string
	SinceKnown                bool
	Header, Message           string
	Filled                    bool
	AddCommand, CommitCommand string
	Problems                  []string
}

// Suggest reads the branch, the changes and the verify record of repo and builds the commit command. It never
// writes to the repository, and no email address from the branch or the git config reaches its output. relative turns a top-level path into one the printed command can use from the caller's
// folder. A message with a subject is linted; its problems are in Problems.
func Suggest(repo Repo, cfg *profile.Commit, branchPrefix *omap.Map[any], in Input, relative func(string) string) (*Suggestion, error) {
	branch := repo.Branch()
	s := &Suggestion{Branch: RedactEmails(branch), HasTicket: cfg.Ticket != nil}
	var err error
	if s.Files, s.Untracked, err = repo.Changes(); err != nil {
		return nil, err
	}
	if s.SinceVerify, s.SinceKnown, err = repo.SinceVerify(); err != nil {
		return nil, err
	}

	s.Type, s.TypeReason = DeriveType(cfg, branchPrefix, branch)
	s.TypeReason = RedactEmails(s.TypeReason)
	if in.Type != nil {
		if !contains(cfg.Types, *in.Type) {
			return nil, fmt.Errorf("--type %s is not one of %s", *in.Type, strings.Join(cfg.Types, ", "))
		}
		s.Type, s.TypeReason = *in.Type, "--type"
	}
	s.Scope, s.ScopeReason = DeriveScope(cfg, s.Files)
	if in.Scope != nil {
		s.Scope, s.ScopeReason = *in.Scope, "--scope"
	}
	if s.HasTicket {
		switch s.Ticket = DeriveTicket(cfg, branch); {
		case HasEmail(s.Ticket):
			s.Ticket, s.TicketReason = "", "the branch match holds an email address; left out"
		case s.Ticket != "":
			s.TicketReason = "branch " + s.Branch
		default:
			s.TicketReason = "none in the branch name"
		}
	}

	parts := Parts{Type: s.Type, Scope: s.Scope, Ticket: s.Ticket, PortedFrom: in.PortedFrom}
	if in.Subject != nil {
		parts.Subject = *in.Subject
		s.Filled = true
	}
	if in.Body != nil {
		parts.Body = *in.Body
	}
	s.Header = Header(cfg, parts)
	s.Message = Message(cfg, parts)

	args := []string{"git", "commit", "-m", QuoteArg(s.Header)}
	switch {
	case strings.TrimSpace(parts.Body) != "":
		args = append(args, "-m", QuoteArg(strings.TrimSpace(parts.Body)))
	case !s.Filled && cfg.Body != "none":
		args = append(args, "-m", QuoteArg("<body>"))
	}
	if trailers := TrailerLines(cfg, parts); len(trailers) > 0 {
		args = append(args, "-m", QuoteArg(strings.Join(trailers, "\n")))
	}
	s.CommitCommand = strings.Join(args, " ")
	if len(s.Files) > 0 {
		add := []string{"git", "add", "--"}
		for _, file := range s.Files {
			add = append(add, Quote(relative(file)))
		}
		s.AddCommand = strings.Join(add, " ")
	}
	if s.Filled {
		s.Problems = Lint(cfg, s.Message)
	}
	return s, nil
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func count(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// Lines is the text output: the derived parts with their reasons, the commands, then the untracked files.
func (s *Suggestion) Lines() []string {
	row := func(label, value, reason string) string {
		return fmt.Sprintf("%-9s%-15s(%s)", label, orDash(value), reason)
	}
	lines := []string{row("type:", s.Type, s.TypeReason), row("scope:", s.Scope, s.ScopeReason)}
	if s.HasTicket {
		lines = append(lines, row("ticket:", s.Ticket, s.TicketReason))
	}
	files := "files:   " + fmt.Sprintf("%d changed", len(s.Files))
	if len(s.Untracked) > 0 {
		files += fmt.Sprintf(", %d untracked (listed below, not included)", len(s.Untracked))
	}
	lines = append(lines, files)
	if s.SinceKnown {
		if len(s.SinceVerify) == 0 {
			lines = append(lines, "since verify: no file changed after the last verify")
		} else {
			lines = append(lines, "since verify: "+count(len(s.SinceVerify), "file")+" changed after the last verify")
			for _, file := range s.SinceVerify {
				lines = append(lines, "  "+file)
			}
		}
	}
	lines = append(lines, "")
	if s.AddCommand == "" {
		lines = append(lines, "nothing to commit: no tracked file changed")
	} else {
		lines = append(lines, s.AddCommand+" && "+s.CommitCommand)
	}
	if len(s.Untracked) > 0 {
		lines = append(lines, "", "untracked, not included; add the ones that belong to this commit on purpose:")
		for _, file := range s.Untracked {
			lines = append(lines, "  "+file)
		}
	}
	return lines
}

// JSON is the derived parts for other tools; since_verify is null when unknown, message null without a subject.
func (s *Suggestion) JSON() *omap.Map[any] {
	list := func(items []string) []any {
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out
	}
	orNil := func(value string, ok bool) any {
		if !ok {
			return nil
		}
		return value
	}
	var since any
	if s.SinceKnown {
		since = list(s.SinceVerify)
	}
	return omap.Of[any](
		"branch", s.Branch,
		"type", s.Type, "type_reason", s.TypeReason,
		"scope", s.Scope, "scope_reason", s.ScopeReason,
		"ticket", orNil(s.Ticket, s.HasTicket),
		"files", list(s.Files),
		"untracked", list(s.Untracked),
		"since_verify", since,
		"header", s.Header,
		"message", orNil(s.Message, s.Filled),
		"add", orNil(s.AddCommand, s.AddCommand != ""),
		"commit", s.CommitCommand,
		"problems", list(s.Problems),
	)
}
