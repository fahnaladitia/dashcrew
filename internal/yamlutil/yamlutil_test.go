package yamlutil

import (
	"math"
	"reflect"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

func TestParseScalarsLikeDartYaml(t *testing.T) {
	cases := []struct {
		yaml string
		want any
	}{
		{"x: 1", 1},
		{"x: 0123", 123},
		{"x: 0x1f", 31},
		{"x: 0o17", 15},
		{"x: -7", -7},
		{"x: 1.5", 1.5},
		{"x: 1e3", 1000.0},
		{"x: .5", 0.5},
		{"x: 1_000", "1_000"},
		{"x: 0123abc", "0123abc"},
		{"x: true", true},
		{"x: False", false},
		{"x: yes", "yes"},
		{"x: on", "on"},
		{"x: ~", nil},
		{"x: null", nil},
		{"x:", nil},
		{"x: '1'", "1"},
		{"x: \"true\"", "true"},
		{"x: 2024-01-01", "2024-01-01"},
		{"x: !!str 12", "12"},
		{"x: nulls", "nulls"},
	}
	for _, c := range cases {
		parsed, err := Parse(c.yaml)
		if err != nil {
			t.Fatalf("%s: %v", c.yaml, err)
		}
		got := parsed.(*omap.Map[any]).Value("x")
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %#v, want %#v", c.yaml, got, c.want)
		}
	}
}

func TestParseInfinityAndNaN(t *testing.T) {
	parsed, err := Parse("a: .inf\nb: -.Inf\nc: .NaN\n")
	if err != nil {
		t.Fatal(err)
	}
	m := parsed.(*omap.Map[any])
	if v := m.Value("a").(float64); !math.IsInf(v, 1) {
		t.Errorf("a: %v", v)
	}
	if v := m.Value("b").(float64); !math.IsInf(v, -1) {
		t.Errorf("b: %v", v)
	}
	if v := m.Value("c").(float64); !math.IsNaN(v) {
		t.Errorf("c: %v", v)
	}
}

func TestParseKeepsKeyOrderAndStringifiesKeys(t *testing.T) {
	parsed, err := Parse("b: 1\na: [x, {c: d}]\n1: one\ntrue: t\n")
	if err != nil {
		t.Fatal(err)
	}
	m := parsed.(*omap.Map[any])
	if got := m.Keys(); !reflect.DeepEqual(got, []string{"b", "a", "1", "true"}) {
		t.Errorf("keys %v", got)
	}
	list := m.Value("a").([]any)
	if list[0] != "x" || list[1].(*omap.Map[any]).Value("c") != "d" {
		t.Errorf("list %#v", list)
	}
}

func TestParseRejectsDuplicateKeys(t *testing.T) {
	if _, err := Parse("a: 1\na: 2\n"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParseEmptyIsNil(t *testing.T) {
	parsed, err := Parse("")
	if err != nil || parsed != nil {
		t.Fatalf("got %#v, %v", parsed, err)
	}
	parsed, err = Parse("# only a comment\n")
	if err != nil || parsed != nil {
		t.Fatalf("got %#v, %v", parsed, err)
	}
}

func TestDartString(t *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{nil, "null"},
		{true, "true"},
		{3, "3"},
		{1.5, "1.5"},
		{2.0, "2.0"},
		{"s", "s"},
		{[]any{"a", 1}, "[a, 1]"},
		{omap.Of[any]("a", 1, "b", []any{}), "{a: 1, b: []}"},
		{map[string]any{"k": "v"}, "{k: v}"},
	}
	for _, c := range cases {
		if got := DartString(c.value); got != c.want {
			t.Errorf("%#v: got %q, want %q", c.value, got, c.want)
		}
	}
}

func TestEqualFollowsDartEquality(t *testing.T) {
	cases := []struct {
		a, b any
		want bool
	}{
		{1, 1, true},
		{1, 1.0, true},
		{"a", "a", true},
		{"1", 1, false},
		{nil, nil, true},
		{nil, false, false},
		{true, true, true},
		{[]any{1}, []any{1}, false},
		{omap.New[any](), omap.New[any](), false},
	}
	for _, c := range cases {
		if got := Equal(c.a, c.b); got != c.want {
			t.Errorf("Equal(%#v, %#v) = %v", c.a, c.b, got)
		}
	}
}

func TestLookupPath(t *testing.T) {
	context := map[string]any{
		"policy": omap.Of[any]("agent_commit", false),
		"stack":  map[string]any{"state": map[string]any{"use": "bloc"}},
		"list":   []any{"a"},
	}
	if got := LookupPath(context, "policy.agent_commit"); got != false {
		t.Errorf("policy.agent_commit = %#v", got)
	}
	if got := LookupPath(context, "stack.state.use"); got != "bloc" {
		t.Errorf("stack.state.use = %#v", got)
	}
	if got := LookupPath(context, "stack.di.use"); got != nil {
		t.Errorf("stack.di.use = %#v", got)
	}
	if got := LookupPath(context, "list.0"); got != nil {
		t.Errorf("list.0 = %#v", got)
	}
}
