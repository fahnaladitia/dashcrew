package commit

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

func ptr(s string) *string { return &s }

func cartRepo(t *testing.T) tempRepo {
	r := newTempRepo(t)
	r.git("checkout", "-q", "-b", "fix/ABC-123-cart-total")
	r.write("lib/features/cart/cart_cubit.dart", "a\n")
	r.write("lib/features/cart/cart_page.dart", "b\n")
	r.git("add", "lib/features/cart/cart_cubit.dart")
	r.write("a.txt", "edited\n")
	r.write("notes.md", "draft\n")
	return r
}

const cartCommit = "    scopes: { 'lib/features/cart/**': cart }\n    ticket: { from_branch: '([A-Z]+-[0-9]+)' }\n"

func suggest(t *testing.T, r tempRepo, in Input) *Suggestion {
	t.Helper()
	p := config(t, cartCommit)
	s, err := Suggest(r.open(), p.Commit, p.Project.Value("branch_prefix").(*omap.Map[any]), in, func(path string) string { return path })
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSuggestTemplateListsFilesExplicitlyAndUntrackedApart(t *testing.T) {
	r := cartRepo(t)
	r.git("add", "-N", "lib/features/cart/cart_page.dart")
	statusBefore := r.git("status", "--porcelain")

	s := suggest(t, r, Input{})

	want := []string{
		"type:    fix            (branch fix/ABC-123-cart-total)",
		"scope:   cart           (2 of 3 files under lib/features/cart)",
		"ticket:  ABC-123        (branch fix/ABC-123-cart-total)",
		"files:   3 changed, 1 untracked (listed below, not included)",
		"",
		"git add -- a.txt lib/features/cart/cart_cubit.dart lib/features/cart/cart_page.dart",
		"git commit -m 'fix(cart): <subject>' -m '<body>' -m 'Refs: ABC-123'",
		"",
		"untracked, not included; add the ones that belong to this commit on purpose:",
		"  notes.md",
	}
	if got := s.Lines(); !reflect.DeepEqual(got, want) {
		t.Errorf("lines\n%s", strings.Join(got, "\n"))
	}
	if s.Problems != nil {
		t.Errorf("a template is not linted: %v", s.Problems)
	}
	if got := r.git("status", "--porcelain"); got != statusBefore {
		t.Errorf("suggest changed the status: %q", got)
	}
}

func TestSuggestWithSubjectQuotesAndLints(t *testing.T) {
	r := cartRepo(t)
	subject := "keep the $total `right` when it's \"0\""

	s := suggest(t, r, Input{Subject: ptr(subject), Body: ptr("Line one.\nLine two.")})

	if s.Problems != nil {
		t.Errorf("problems %v", s.Problems)
	}
	args := shellWords(t, strings.TrimPrefix(s.CommitCommand, "git commit "))
	if want := []string{"-m", "fix(cart): " + subject, "-m", "Line one.\nLine two.", "-m", "Refs: ABC-123"}; !reflect.DeepEqual(args, want) {
		t.Errorf("args %q", args)
	}

	long := suggest(t, r, Input{Subject: ptr(strings.Repeat("x", 80)), Type: ptr("docs"), Scope: ptr("")})
	if !strings.HasPrefix(long.Header, "docs: xxx") || len(long.Problems) != 1 || !strings.HasPrefix(long.Problems[0], "subject line is 86 characters") {
		t.Errorf("long %q %v", long.Header, long.Problems)
	}
}

// shellWords lets bash split a command line and prints each word NUL-terminated.
func shellWords(t *testing.T, line string) []string {
	t.Helper()
	out, err := exec.Command("bash", "--norc", "--noprofile", "-c", "printf '%s\\0' "+line).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
}

func TestSuggestUnknownTypeAndNothingToCommit(t *testing.T) {
	r := newTempRepo(t)
	p := config(t, "")
	if _, err := Suggest(r.open(), p.Commit, nil, Input{Type: ptr("wip")}, func(s string) string { return s }); err == nil ||
		err.Error() != "--type wip is not one of feat, fix, refactor, test, docs, chore, perf, build, ci" {
		t.Errorf("err %v", err)
	}
	r.write("new.txt", "x\n")
	s, err := Suggest(r.open(), p.Commit, nil, Input{}, func(s string) string { return s })
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Join(s.Lines(), "\n")
	if !strings.Contains(lines, "type:    -              (no branch prefix of main maps to a type; pass --type)") ||
		!strings.Contains(lines, "nothing to commit: no tracked file changed") || s.AddCommand != "" {
		t.Errorf("%s", lines)
	}
}

func TestSuggestReportsFilesChangedSinceVerify(t *testing.T) {
	r := cartRepo(t)
	if _, err := r.open().RecordVerify(); err != nil {
		t.Fatal(err)
	}
	if s := suggest(t, r, Input{}); !reflect.DeepEqual(s.SinceVerify, []string{}) || !s.SinceKnown {
		t.Errorf("right after verify %v %v", s.SinceVerify, s.SinceKnown)
	}
	r.write("lib/features/cart/cart_page.dart", "edited by hand\n")
	s := suggest(t, r, Input{})
	lines := strings.Join(s.Lines(), "\n")
	if !strings.Contains(lines, "since verify: 1 file changed after the last verify\n  lib/features/cart/cart_page.dart\n") {
		t.Errorf("%s", lines)
	}
	encoded, err := dartjson.Encode(s.JSON())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded, `"since_verify":["lib/features/cart/cart_page.dart"]`) || !strings.Contains(encoded, `"message":null`) {
		t.Errorf("%s", encoded)
	}
}

func TestSuggestNeverShowsAnEmailFromTheBranchOrTheGitConfig(t *testing.T) {
	r := cartRepo(t)
	r.git("config", "user.email", "ana@example.com")
	r.git("config", "user.name", "Ana <ana@example.com>")
	t.Setenv("GIT_AUTHOR_EMAIL", "ana@example.com")
	t.Setenv("GIT_COMMITTER_EMAIL", "ana@example.com")
	r.git("checkout", "-q", "-b", "fix/ana@example.com-ABC-1-cart")
	for name, yaml := range map[string]string{
		"ticket id":         cartCommit,
		"ticket as address": "    ticket: { from_branch: '[a-z]+@[a-z.]+' }\n",
	} {
		p := config(t, yaml)
		for _, in := range []Input{{}, {Subject: ptr("keep the total"), Body: ptr("Why.")}} {
			s, err := Suggest(r.open(), p.Commit, p.Project.Value("branch_prefix").(*omap.Map[any]), in, func(path string) string { return path })
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := dartjson.Encode(s.JSON())
			if err != nil {
				t.Fatal(err)
			}
			for _, out := range append(s.Lines(), encoded, s.Message) {
				if HasEmail(out) {
					t.Errorf("%s: email in %q", name, out)
				}
			}
			if s.Type != "fix" || s.Problems != nil {
				t.Errorf("%s: type %q, problems %v", name, s.Type, s.Problems)
			}
		}
	}
}
