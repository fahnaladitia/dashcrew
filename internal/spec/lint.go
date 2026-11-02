package spec

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Base is a spec file as a git ref holds it, for Lint to compare with.
type Base struct {
	Doc  Doc
	Text string
	Ref  string
}

// Lint is every problem of doc: its format problems, duplicate IDs, gaps in the IDs of a new spec, and open
// questions without an owner on an approved one. With base, also an ID that vanished, a renumbered or reused ID,
// and an approved file changed without a rev increase. Problems come by line, the file-wide ones first.
func Lint(doc Doc, base *Base, isNew bool) []Problem {
	problems := append([]Problem{}, doc.Problems...)
	first := map[string]int{}
	numbers := map[string][]int{}
	for _, id := range doc.IDs {
		if line, ok := first[id.String()]; ok {
			problems = append(problems, Problem{id.Line, fmt.Sprintf("%s is defined twice (first on line %d)", id, line)})
			continue
		}
		first[id.String()] = id.Line
		numbers[id.Kind] = append(numbers[id.Kind], id.N)
	}
	if isNew {
		problems = append(problems, gaps(doc, numbers)...)
	}
	if doc.Status == Approved {
		for _, q := range doc.Questions {
			if q.Owner == "" {
				problems = append(problems, Problem{q.Line, "open question without an owner on an approved spec; end it with owner: <role>"})
			}
		}
	}
	if base != nil {
		problems = append(problems, against(doc, *base)...)
	}
	sort.SliceStable(problems, func(i, j int) bool { return problems[i].Line < problems[j].Line })
	return problems
}

// gaps reports, per kind, the numbers a new spec skips; the problem is on the line of the first ID after a gap.
func gaps(doc Doc, numbers map[string][]int) []Problem {
	var problems []Problem
	for _, kind := range []string{"AC", "C", "D"} {
		list := numbers[kind]
		if len(list) == 0 {
			continue
		}
		have := map[int]bool{}
		highest := 0
		for _, n := range list {
			have[n] = true
			highest = max(highest, n)
		}
		var missing []string
		for n := 1; n <= highest; n++ {
			if !have[n] {
				missing = append(missing, kind+"-"+strconv.Itoa(n))
			}
		}
		if len(missing) == 0 {
			continue
		}
		line := 0
		for _, id := range doc.IDs {
			if id.Kind == kind && id.N > 1 && !have[id.N-1] {
				line = id.Line
				break
			}
		}
		problems = append(problems, Problem{line, fmt.Sprintf("%s IDs skip %s: a new spec numbers them from %s-1 without gaps",
			kind, strings.Join(missing, ", "), kind)})
	}
	return problems
}

func against(doc Doc, base Base) []Problem {
	var problems []Problem
	now := map[string]ID{}
	for _, id := range doc.IDs {
		if _, ok := now[id.String()]; !ok {
			now[id.String()] = id
		}
	}
	was := map[string]ID{}
	for _, id := range base.Doc.IDs {
		if _, ok := was[id.String()]; ok {
			continue
		}
		was[id.String()] = id
		if _, ok := now[id.String()]; !ok {
			problems = append(problems, Problem{0, fmt.Sprintf("%s is gone (it was on line %d at %s); keep it struck through: `~~%s~~ dropped: <why>`",
				id, id.Line, base.Ref, id)})
		}
	}
	for _, id := range doc.IDs {
		if id.Dropped || now[id.String()].Line != id.Line {
			continue
		}
		old, existed := was[id.String()]
		if existed && old.Dropped {
			problems = append(problems, Problem{id.Line, fmt.Sprintf("%s was dropped at %s; a dropped ID is never reused, number a new one", id, base.Ref)})
			continue
		}
		if existed && old.Text == id.Text {
			continue
		}
		for _, other := range base.Doc.IDs {
			if other.Kind == id.Kind && other.N != id.N && !other.Dropped && other.Text != "" && other.Text == id.Text {
				problems = append(problems, Problem{id.Line, fmt.Sprintf("%s holds the text of %s at %s; IDs are never renumbered", id, other, base.Ref)})
				break
			}
		}
	}
	if base.Doc.Status == Approved && doc.Rev <= base.Doc.Rev && changedBeyondStatus(base.Text, doc) {
		problems = append(problems, Problem{doc.RevLine, fmt.Sprintf(
			"the file was approved at %s and changed without a rev increase; a member runs dashcrew spec revise first", base.Ref)})
	}
	return problems
}

// changedBeyondStatus is true when doc's text differs from base in more than its status line, or its new status is
// not superseded or approved. Lint keeps the text it parsed in Text for this comparison.
func changedBeyondStatus(baseText string, doc Doc) bool {
	if doc.text == baseText {
		return false
	}
	if doc.Status != Superseded && doc.Status != Approved {
		return true
	}
	return withoutStatus(doc.text) != withoutStatus(baseText)
}

func withoutStatus(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := lines[:0:0]
	for _, line := range lines {
		if !strings.HasPrefix(line, "status:") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
