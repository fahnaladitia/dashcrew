package commit

import (
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

func config(t *testing.T, commitYaml string) *profile.Profile {
	t.Helper()
	text := "toolkit: 1\nagents: [claude-code]\n"
	if commitYaml != "" {
		text += "project:\n  commit:\n" + commitYaml
	}
	p, err := profile.Parse(text, profile.KnownOptions{"flutter": omap.New[[]string]()})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHeaderFillsThePatternAndDropsAnEmptyScopeWithItsBrackets(t *testing.T) {
	cases := []struct {
		pattern string
		parts   Parts
		want    string
	}{
		{"", Parts{Type: "fix", Scope: "cart", Subject: "keep the total"}, "fix(cart): keep the total"},
		{"", Parts{Type: "fix", Subject: "keep the total"}, "fix: keep the total"},
		{"", Parts{Scope: "cart"}, "<type>(cart): <subject>"},
		{"'[<scope>] <type>: <subject>'", Parts{Type: "feat", Subject: "x"}, "feat: x"},
		{"'<type>: <scope> <subject>'", Parts{Type: "feat", Subject: "x"}, "feat: x"},
		{"'<type>: <subject> <ticket>'", Parts{Type: "feat", Subject: "x"}, "feat: x"},
		{"'<type>: <subject> <ticket>'", Parts{Type: "feat", Subject: "x", Ticket: "AB-1"}, "feat: x AB-1"},
	}
	for _, c := range cases {
		yaml := ""
		if c.pattern != "" {
			yaml = "    pattern: " + c.pattern + "\n    ticket: { from_branch: x }\n"
		}
		if got := Header(config(t, yaml).Commit, c.parts); got != c.want {
			t.Errorf("%s %+v = %q, want %q", c.pattern, c.parts, got, c.want)
		}
	}
}

func TestMessageJoinsBodyAndTicketTrailer(t *testing.T) {
	cfg := config(t, "    ticket: { from_branch: '([A-Z]+-[0-9]+)' }\n").Commit

	got := Message(cfg, Parts{Type: "fix", Subject: "keep the total", Body: "Line one.\nLine two.", Ticket: "ABC-12"})
	if want := "fix: keep the total\n\nLine one.\nLine two.\n\nRefs: ABC-12"; got != want {
		t.Errorf("%q", got)
	}
	if got := Message(cfg, Parts{Type: "fix", Subject: "x"}); got != "fix: x" {
		t.Errorf("%q", got)
	}
}

func TestLint(t *testing.T) {
	cases := []struct {
		name, yaml, message string
		want                []string
	}{
		{"good", "", "fix(cart): keep the total\n\nWhy it broke.\n\nSigned-off-by: Ana Lima\n", nil},
		{"good without scope", "", "docs: explain lint", nil},
		{"breaking marker", "", "feat(api)!: drop v1", nil},
		{"wrong type", "", "wip: stuff", []string{`type "wip" is not one of feat, fix, refactor, test, docs, chore, perf, build, ci`}},
		{"no pattern", "", "Fix the cart", []string{`subject line "Fix the cart" does not match <type>(<scope>): <subject>`}},
		{"empty", "", "\n\n", []string{"message is empty"}},
		{"long subject", "    subject_max: 20\n", "fix: a subject line longer than twenty", []string{"subject line is 38 characters, over 20"}},
		{"unicode counts characters", "    subject_max: 12\n", "fix: café ok", nil},
		{"placeholder left", "", "fix: <subject>\n\n<body>", []string{"placeholder <subject> is not filled", "placeholder <body> is not filled"}},
		{"line 2 not blank", "", "fix: x\nmore", []string{"line 2 must be blank, between the subject line and the body"}},
		{"missing required body", "    body: required\n    ticket: { from_branch: x }\n", "fix: x\n\nRefs: AB-1", []string{"body is required"}},
		{"body not allowed", "    body: none\n", "fix: x\n\nWhy.", []string{"body is not allowed"}},
		{"unknown trailer", "", "fix: x\n\nChange-Id: 12", []string{`trailer "Change-Id" is not allowed; allowed: Co-authored-by, Signed-off-by, Reviewed-by`}},
		{"extra and ticket trailers", "    trailers: [Ported-from]\n    ticket: { from_branch: x }\n", "fix: x\n\nRefs: AB-1\nported-from: abc", nil},
		{"ai co-author", "", "fix: x\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
			[]string{"email address on line 3: commit messages never show one", "AI attribution: AI co-author trailer on line 3"}},
		{"human co-author", "", "fix: x\n\nCo-authored-by: Kai Lee", nil},
		{"human co-author with email", "", "fix: x\n\nCo-authored-by: Kai Lee <kai@example.com>", []string{"email address on line 3: commit messages never show one"}},
		{"plain email in body", "", "fix: x\n\nAsk ana.lima+cart@mail.example.co.id about it.", []string{"email address on line 3: commit messages never show one"}},
		{"email in brackets in body", "", "fix: x\n\nReported by <ops@example.com>.", []string{"email address on line 3: commit messages never show one"}},
		{"email in subject", "", "fix: reply to ana@example.com", []string{"email address on line 1: commit messages never show one"}},
		{"sign-off email without allow list", "", "fix: x\n\nSigned-off-by: Ana <ana@example.com>", []string{"email address on line 3: commit messages never show one"}},
		{"sign-off email with allow list", "    allow_email_trailers: [signed-off-by]\n", "fix: x\n\nSigned-off-by: Ana <ana@example.com>", nil},
		{"allow list covers trailers only", "    allow_email_trailers: [Signed-off-by]\n",
			"fix: x\n\nSigned-off-by: Ana <ana@example.com>\nnot a trailer line\n\nCo-authored-by: Kai <kai@example.com>",
			[]string{"email address on line 3: commit messages never show one", "email address on line 6: commit messages never show one"}},
		{"look-alikes are not emails", "", "fix: thank @ana for the foo@ hint\n\nSee assets/icon@2x.png, lib/a@1.5x.webp, user@localhost and a@b.", nil},
		{"generated footer", "", "fix: x\n\nBody.\n\nGenerated with Claude Code", []string{"AI attribution: generated-with footer on line 5"}},
		{"robot line", "", "fix: x\n\n\U0001F916 Generated with [Claude Code](https://claude.com/claude-code)",
			[]string{"AI attribution: generated-with footer on line 3"}},
		{"robot only", "", "fix: x\n\n\U0001F916 by a bot", []string{"AI attribution: robot line on line 3"}},
		{"build output is not attribution", "", "build: regenerate\n\nFiles generated by build_runner.", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Lint(config(t, c.yaml).Commit, c.message); !reflect.DeepEqual(got, c.want) {
				t.Errorf("%q\n got %q\nwant %q", c.message, got, c.want)
			}
		})
	}
}

func TestDeriveType(t *testing.T) {
	p := config(t, "")
	prefixes := omap.Of[any]("feat", "feature/", "fix", "fix/", "hotfix", "fix/hot/")
	cases := map[string]string{
		"feature/cart":    "feat",
		"fix/cart-total":  "fix",
		"fix/hot/crash":   "fix",
		"chore/bump":      "",
		"":                "",
		"featurex/a":      "",
		"feature/x/fix/y": "feat",
	}
	for branch, want := range cases {
		if got, _ := DeriveType(p.Commit, prefixes, branch); got != want {
			t.Errorf("%q = %q, want %q", branch, got, want)
		}
	}
	if _, reason := DeriveType(p.Commit, prefixes, "chore/bump"); reason != "no branch prefix of chore/bump maps to a type; pass --type" {
		t.Errorf("reason %q", reason)
	}
}

func TestDeriveScope(t *testing.T) {
	p := config(t, "    scopes: { 'lib/features/cart/**': cart, 'lib/core/**': core, 'test/*cart*': cart }\n")
	cases := []struct {
		files      []string
		scope, why string
	}{
		{[]string{"lib/features/cart/a.dart", "lib/features/cart/b.dart", "test/cart_test.dart", "README.md"}, "cart", "3 of 4 files under lib/features/cart, test/*cart*"},
		{[]string{"lib/core/a.dart", "lib/features/cart/a.dart"}, "cart", "1 of 2 files under lib/features/cart"},
		{[]string{"lib/core/a.dart", "lib/core/b.dart", "lib/features/cart/a.dart"}, "core", "2 of 3 files under lib/core"},
		{[]string{"README.md"}, "", "no scope glob matches the changed files"},
	}
	for _, c := range cases {
		scope, why := DeriveScope(p.Commit, c.files)
		if scope != c.scope || why != c.why {
			t.Errorf("%v = %q (%s), want %q (%s)", c.files, scope, why, c.scope, c.why)
		}
	}
	if scope, why := DeriveScope(config(t, "").Commit, []string{"a"}); scope != "" || why != "no scopes in project.commit" {
		t.Errorf("no scopes: %q %q", scope, why)
	}
}

func TestDeriveTicket(t *testing.T) {
	p := config(t, "    ticket: { from_branch: '([A-Z]+-[0-9]+)' }\n")
	if got := DeriveTicket(p.Commit, "fix/ABC-123-cart"); got != "ABC-123" {
		t.Errorf("%q", got)
	}
	if got := DeriveTicket(p.Commit, "fix/cart"); got != "" {
		t.Errorf("%q", got)
	}
	whole := config(t, "    ticket: { from_branch: '[0-9]+' }\n")
	if got := DeriveTicket(whole.Commit, "fix/42-cart"); got != "42" {
		t.Errorf("%q", got)
	}
	if got := DeriveTicket(config(t, "").Commit, "fix/ABC-1"); got != "" {
		t.Errorf("no ticket config: %q", got)
	}
}

var quoteCases = []string{
	"plain",
	"fix(cart): keep the total",
	"it's \"quoted\"",
	"costs $5 and ${HOME} and $(id)",
	"run `id` now",
	"line one\nline two\n",
	"back\\slash and \\n",
	"bang! !! !$",
	"glob * ? [a] ~ ~/x =cmd",
	"café ünïcødé 日本語 \U0001F600",
	"",
	"'",
	"''''",
	"semi; amp & pipe | <in >out",
	"tab\there",
}

func TestQuoteRoundTripsThroughTheShells(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Quote targets POSIX shells, which this test runs")
	}
	for _, shell := range [][]string{{"sh", "-c"}, {"bash", "--norc", "--noprofile", "-c"}, {"zsh", "-f", "-c"}} {
		if _, err := exec.LookPath(shell[0]); err != nil {
			t.Logf("%s not installed", shell[0])
			continue
		}
		for _, text := range quoteCases {
			args := append(append([]string{}, shell[1:]...), "printf '%s' "+Quote(text))
			out, err := exec.Command(shell[0], args...).Output()
			if err != nil {
				t.Errorf("%s %q: %v", shell[0], text, err)
				continue
			}
			if string(out) != text {
				t.Errorf("%s %q came back %q", shell[0], text, out)
			}
		}
	}
}

func TestQuoteLeavesPlainPathsBare(t *testing.T) {
	for text, want := range map[string]string{
		"lib/a.dart":       "lib/a.dart",
		"lib/my file.dart": "'lib/my file.dart'",
		"=x":               "'=x'",
		"a=b":              "a=b",
		"":                 "''",
		"don't":            `'don'\''t'`,
	} {
		if got := Quote(text); got != want {
			t.Errorf("Quote(%q) = %s, want %s", text, got, want)
		}
	}
	if got := QuoteArg("fix"); got != "'fix'" {
		t.Errorf("QuoteArg %s", got)
	}
	if !strings.HasPrefix(QuoteArg("a\nb"), "'a\nb") {
		t.Error("newline")
	}
}

func TestRedactEmails(t *testing.T) {
	for in, want := range map[string]string{
		"fix/ana@example.com-cart": "fix/<email>-cart",
		"assets/icon@2x.png":       "assets/icon@2x.png",
		"ana@example.com":          "<email>",
		"by <ana@example.com>, ok": "by <<email>>, ok",
		"x ana@example.com y":      "x <email> y",
		"@ana":                     "@ana",
	} {
		if got := RedactEmails(in); got != want {
			t.Errorf("RedactEmails(%q) = %q, want %q", in, got, want)
		}
	}
}
