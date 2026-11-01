// Package guard is the pre-tool hook every agent calls (`dashcrew guard`) and what sync writes for it: the policy
// in `.ai/guard.json` and the `.ai/bin/guard` shim.
package guard

import (
	"errors"
	"sort"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Schema is the `schema` of `.ai/guard.json`.
const Schema = 1

// AlwaysBlockedGit are the git subcommands blocked even when `policy.agent_commit` is true.
var AlwaysBlockedGit = []string{"push", "stash", "reset", "rebase", "cherry-pick", "revert", "merge", "tag"}

// Policy is the compiled guard policy, stored as `.ai/guard.json` by sync.
type Policy struct {
	BlockedGit       map[string]bool
	BlockFormatters  bool
	PlansDir         string
	RoleCapability   map[string]string
	ForbiddenImports []string
	LocalPackages    []string

	// Set by LoadPolicy: entries of the wrong type, which fail only when Evaluate reaches them.
	brokenRoles     map[string]bool
	forbiddenBroken bool
	localBroken     bool
}

// CompilePolicy builds the policy for p; c must be resolved for the profile's platform.
// Push and history-rewriting git commands stay blocked even when `policy.agent_commit` is true.
func CompilePolicy(p *profile.Profile, c *catalog.Catalog) Policy {
	forbiddenImports := []string{}
	if analytics, ok := p.Stack.Get("analytics"); ok {
		var vendorImport any
		if options, ok := c.Packs.Get("analytics"); ok {
			if pack, ok := options.Get(analytics.Option); ok && pack.Params != nil {
				vendorImport = pack.Params.Value("vendor_import")
			}
		}
		if analytics.Params.Has("vendor_import") {
			vendorImport = analytics.Params.Value("vendor_import")
		}
		if text, ok := vendorImport.(string); ok && text != "" {
			forbiddenImports = append(forbiddenImports, text)
		}
	}

	blocked := map[string]bool{}
	for _, sub := range AlwaysBlockedGit {
		blocked[sub] = true
	}
	if p.Policy.Value("agent_commit") != true {
		blocked["commit"] = true
	}
	roles := map[string]string{}
	for _, role := range c.Roles {
		roles[role.Name] = role.Capability
	}
	localPackages := []string{}
	if list, ok := p.Project.Value("local_packages").([]any); ok {
		for _, item := range list {
			localPackages = append(localPackages, yamlutil.DartString(item))
		}
	}
	plans, _ := p.Paths.Value("plans").(string)
	return Policy{
		BlockedGit:       blocked,
		BlockFormatters:  p.Policy.Value("formatter") != true,
		PlansDir:         plans,
		RoleCapability:   roles,
		ForbiddenImports: forbiddenImports,
		LocalPackages:    localPackages,
	}
}

// ToJSON is the `.ai/guard.json` object: blocked git subcommands and role names sorted.
func (p Policy) ToJSON() *omap.Map[any] {
	blocked := make([]string, 0, len(p.BlockedGit))
	for sub := range p.BlockedGit {
		blocked = append(blocked, sub)
	}
	sort.Strings(blocked)
	names := make([]string, 0, len(p.RoleCapability))
	for name := range p.RoleCapability {
		names = append(names, name)
	}
	sort.Strings(names)
	roles := omap.New[any]()
	for _, name := range names {
		roles.Set(name, p.RoleCapability[name])
	}
	return omap.Of[any](
		"schema", Schema,
		"blocked_git", toAny(blocked),
		"block_formatters", p.BlockFormatters,
		"plans_dir", p.PlansDir,
		"role_capability", roles,
		"forbidden_imports", toAny(p.ForbiddenImports),
		"local_packages", toAny(p.LocalPackages),
	)
}

func toAny(items []string) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

// PolicyFromJSON reads `.ai/guard.json` with the checks of the Dart reader: every key but `schema` is required with
// its type, and `blocked_git` must hold strings only. Entries of the other lists and maps are not type checked
// there; here an entry that is not a string is left out.
func PolicyFromJSON(text string) (Policy, error) {
	value, err := dartjson.Decode(text)
	if err != nil {
		return Policy{}, err
	}
	object, ok := value.(*omap.Map[any])
	if !ok {
		return Policy{}, errors.New("guard policy is not a JSON object")
	}
	blockedList, ok := object.Value("blocked_git").([]any)
	if !ok {
		return Policy{}, errors.New("blocked_git is not a list")
	}
	blocked := map[string]bool{}
	for _, item := range blockedList {
		sub, ok := item.(string)
		if !ok {
			return Policy{}, errors.New("blocked_git holds a value that is not a string")
		}
		blocked[sub] = true
	}
	blockFormatters, ok := object.Value("block_formatters").(bool)
	if !ok {
		return Policy{}, errors.New("block_formatters is not a bool")
	}
	plansDir, ok := object.Value("plans_dir").(string)
	if !ok {
		return Policy{}, errors.New("plans_dir is not a string")
	}
	roleMap, ok := object.Value("role_capability").(*omap.Map[any])
	if !ok {
		return Policy{}, errors.New("role_capability is not a map")
	}
	roles := map[string]string{}
	for _, name := range roleMap.Keys() {
		if capability, ok := roleMap.Value(name).(string); ok {
			roles[name] = capability
		}
	}
	forbidden, err := stringsOf(object, "forbidden_imports")
	if err != nil {
		return Policy{}, err
	}
	local, err := stringsOf(object, "local_packages")
	if err != nil {
		return Policy{}, err
	}
	return Policy{BlockedGit: blocked, BlockFormatters: blockFormatters, PlansDir: plansDir, RoleCapability: roles, ForbiddenImports: forbidden, LocalPackages: local}, nil
}

func stringsOf(object *omap.Map[any], key string) ([]string, error) {
	list, ok := object.Value(key).([]any)
	if !ok {
		return nil, errors.New(key + " is not a list")
	}
	out := []string{}
	for _, item := range list {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out, nil
}
