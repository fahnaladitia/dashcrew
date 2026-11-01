// Package render joins catalog fragments per target for one profile: core, the profile's platform and the
// selected packs, filtered by `when`, rendered with strict Mustache.
package render

import (
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/mustache"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Error is a template error, with the fragment source that failed.
type Error struct {
	Source  string
	Message string
}

func (e *Error) Error() string { return e.Source + ": " + e.Message }

// RenderedTarget is the content of one target plus the rule ids its fragments state, in first-seen order.
type RenderedTarget struct {
	Target  string
	Content string
	Rules   []string
}

// Renderer renders one catalog.
type Renderer struct {
	Catalog *catalog.Catalog
}

// New returns a renderer for c.
func New(c *catalog.Catalog) *Renderer { return &Renderer{Catalog: c} }

// Render returns every target sorted by name; an unknown template variable is an error.
func (r *Renderer) Render(p *profile.Profile) ([]RenderedTarget, error) {
	context := r.ContextFor(p)
	fragments := append([]catalog.Fragment{}, r.Catalog.FragmentsFor(p.Platform)...)
	for _, pack := range r.SelectedPacks(p) {
		fragments = append(fragments, pack.Fragments...)
	}

	byTarget := map[string][]catalog.Fragment{}
	for _, fragment := range fragments {
		if WhenMatches(fragment.When, context) {
			byTarget[fragment.Target] = append(byTarget[fragment.Target], fragment)
		}
	}
	targets := make([]string, 0, len(byTarget))
	for target := range byTarget {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	context["skill_index"] = r.skillIndex(p, targets)

	out := make([]RenderedTarget, 0, len(targets))
	for _, target := range targets {
		rendered, err := renderTarget(target, byTarget[target], context)
		if err != nil {
			return nil, err
		}
		out = append(out, rendered)
	}
	return out, nil
}

func renderTarget(target string, fragments []catalog.Fragment, context map[string]any) (RenderedTarget, error) {
	sort.SliceStable(fragments, func(i, j int) bool {
		if fragments[i].Order != fragments[j].Order {
			return fragments[i].Order < fragments[j].Order
		}
		return fragments[i].Source < fragments[j].Source
	})
	parts := make([]string, 0, len(fragments))
	var rules []string
	seen := map[string]bool{}
	for _, fragment := range fragments {
		text, err := mustache.Render(fragment.Body, context)
		if err != nil {
			return RenderedTarget{}, &Error{Source: fragment.Source, Message: err.Error()}
		}
		parts = append(parts, TrimRight(text))
		for _, rule := range fragment.Rules {
			if !seen[rule] {
				seen[rule] = true
				rules = append(rules, rule)
			}
		}
	}
	return RenderedTarget{Target: target, Content: strings.Join(parts, "\n\n") + "\n", Rules: rules}, nil
}

// TrimRight is Dart's String.trimRight: it drops trailing whitespace, Unicode spaces included.
func TrimRight(s string) string {
	return strings.TrimRightFunc(s, isDartWhitespace)
}

func isDartWhitespace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0D, r == 0x20, r == 0x85, r == 0xA0, r == 0x1680,
		r >= 0x2000 && r <= 0x200A, r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000, r == 0xFEFF:
		return true
	}
	return false
}

// skillIndex is one entry per rendered skill target that has a description, sorted by name;
// a skill without when_to_use is indexed by its description.
func (r *Renderer) skillIndex(p *profile.Profile, targets []string) []any {
	metas := map[string]catalog.DocMeta{}
	for _, pack := range r.SelectedPacks(p) {
		for name, meta := range pack.Skills {
			metas[name] = meta
		}
	}
	for name, meta := range r.Catalog.SkillsFor(p.Platform) {
		metas[name] = meta
	}
	var names []string
	for _, target := range targets {
		if strings.HasPrefix(target, "skill/") {
			names = append(names, strings.TrimPrefix(target, "skill/"))
		}
	}
	sort.Strings(names)
	index := []any{}
	for _, name := range names {
		meta, ok := metas[name]
		if !ok {
			continue
		}
		whenToUse := meta.Description
		if meta.WhenToUse != nil {
			whenToUse = *meta.WhenToUse
		}
		index = append(index, map[string]any{"name": name, "when_to_use": whenToUse, "review": meta.Review})
	}
	return index
}

// SelectedPacks are the packs `profile.stack` chooses that exist for the profile's platform, in stack order.
func (r *Renderer) SelectedPacks(p *profile.Profile) []*catalog.Pack {
	packs := r.Catalog.PacksFor(p.Platform)
	var out []*catalog.Pack
	for _, family := range p.Stack.Keys() {
		options, ok := packs.Get(family)
		if !ok {
			continue
		}
		if pack, ok := options.Get(p.Stack.Value(family).Option); ok {
			out = append(out, pack)
		}
	}
	return out
}

// ContextFor is the template context: the profile, `platform.*`, and `pack.<family>` params over pack defaults.
func (r *Renderer) ContextFor(p *profile.Profile) map[string]any {
	context := p.ToContext()
	context["platform"] = r.Catalog.PlatformFor(p).ToContext()
	packs := map[string]any{}
	for _, pack := range r.SelectedPacks(p) {
		params := pack.Params.Copy()
		choice := p.Stack.Value(pack.Family).Params
		for _, key := range choice.Keys() {
			params.Set(key, choice.Value(key))
		}
		packs[pack.Family] = params
	}
	context["pack"] = packs
	return context
}

// WhenMatches is true when every `when` key matches; a list value means any of.
func WhenMatches(when *omap.Map[any], context map[string]any) bool {
	if when == nil {
		return true
	}
	for _, path := range when.Keys() {
		actual := yamlutil.LookupPath(context, path)
		expected := when.Value(path)
		ok := false
		if list, isList := expected.([]any); isList {
			for _, item := range list {
				ok = ok || yamlutil.Equal(item, actual)
			}
		} else {
			ok = yamlutil.Equal(expected, actual)
		}
		if !ok {
			return false
		}
	}
	return true
}
