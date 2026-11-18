package catalog

import (
	"reflect"
	"strings"
	"testing"
)

// TestGoIsBuiltIn: `platform: go` resolves to platforms/go/ with its http and db packs and its backend guard rules,
// without a project platform.
func TestGoIsBuiltIn(t *testing.T) {
	c, err := LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := c.Platforms["go"]
	if !ok {
		t.Fatalf("no built-in go platform; platforms: %v", keys(c.Platforms))
	}
	if spec.Root != "platforms/go" || !reflect.DeepEqual(spec.Detect, []string{"go.mod"}) {
		t.Errorf("go platform at %q, detect %v", spec.Root, spec.Detect)
	}
	if spec.Guard.Empty() {
		t.Error("the go platform lost its backend guard rules")
	}
	resolved := c.ResolvedFor("go")
	for family, want := range map[string][]string{
		"http": {"chi", "echo", "fiber", "gin", "nethttp", "none"},
		"db":   {"gorm", "none", "pgx", "sqlc"},
	} {
		options, ok := resolved.Packs.Get(family)
		if !ok {
			t.Errorf("no %s packs for go", family)
			continue
		}
		if got := options.Keys(); !reflect.DeepEqual(sorted(got), want) {
			t.Errorf("%s options %v, want %v", family, got, want)
		}
	}
}

func TestBuiltInPlatformsCarryNoStarterHeader(t *testing.T) {
	c, err := LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	for name, spec := range c.Platforms {
		if spec.Description != nil && strings.Contains(strings.ToLower(*spec.Description), "starter") {
			t.Errorf("%s: description %q still reads as a starter", name, *spec.Description)
		}
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return sorted(out)
}

func sorted(in []string) []string {
	out := append([]string{}, in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
