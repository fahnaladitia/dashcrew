// Package guard is the pre-tool hook every agent calls (`dashcrew guard`) and what sync writes for it: the policy
// in `.ai/guard.json` and the `.ai/bin/guard` shim.
package guard

import (
	"errors"
	"sort"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/knowledge"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/version"
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
	// KnowledgeClone is the knowledge clone as `~/.dashcrew/knowledge/<name>`; empty without `knowledge`. Writes
	// whose real path is inside it are allowed only under `inbox/`.
	KnowledgeClone string
	// BlockCommands, BlockSQL and BlockMigrations come from the platform's `guard:` key (P10); empty without one.
	BlockCommands   [][]string
	BlockSQL        SQLBlock
	BlockMigrations MigrationBlock

	// knowledge is the hook's folder and KnowledgeClone resolved on this machine; Run sets it (see at). Without it
	// the write rules read paths as text only, as the Dart guard did.
	knowledge *knowledgeRoots

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
	clone := ""
	if p.Knowledge != nil {
		clone = knowledge.PolicyClonePath(p.Knowledge.Name)
	}
	policy := Policy{
		KnowledgeClone:   clone,
		BlockedGit:       blocked,
		BlockFormatters:  p.Policy.Value("formatter") != true,
		PlansDir:         plans,
		RoleCapability:   roles,
		ForbiddenImports: forbiddenImports,
		LocalPackages:    localPackages,
	}
	if g := c.PlatformFor(p).Guard; !g.Empty() {
		policy.BlockCommands = g.Commands
		policy.BlockSQL = SQLBlock{Clients: g.SQLClients, Statements: g.SQLStatements}
		policy.BlockMigrations = MigrationBlock{Tools: g.MigrationTools, LocalHosts: g.LocalHosts}
	}
	return policy
}

// SQLBlock blocks the statements, as whole words in any case, in the SQL given to a client.
type SQLBlock struct{ Clients, Statements []string }

// MigrationBlock blocks a migration tool run whose connection string names a host outside LocalHosts.
type MigrationBlock struct{ Tools, LocalHosts []string }

// Minimum is the oldest dashcrew whose guard enforces p: version.BackendGuardMinimum with backend rules, else
// version.GuardMinimum. Sync writes it into the shim and the lock.
func (p Policy) Minimum() string {
	if p.HasBackendRules() {
		return version.BackendGuardMinimum
	}
	return version.GuardMinimum
}

// HasBackendRules is true when the policy holds a rule a dashcrew older than version.BackendGuardMinimum ignores.
func (p Policy) HasBackendRules() bool {
	return len(p.BlockCommands) > 0 || len(p.BlockSQL.Clients) > 0 || len(p.BlockMigrations.Tools) > 0
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
	object := omap.Of[any](
		"schema", Schema,
		"blocked_git", toAny(blocked),
		"block_formatters", p.BlockFormatters,
		"plans_dir", p.PlansDir,
		"role_capability", roles,
		"forbidden_imports", toAny(p.ForbiddenImports),
		"local_packages", toAny(p.LocalPackages),
	)
	if p.KnowledgeClone != "" {
		object.Set("knowledge_clone", p.KnowledgeClone)
	}
	if len(p.BlockCommands) > 0 {
		commands := make([]any, len(p.BlockCommands))
		for i, words := range p.BlockCommands {
			commands[i] = toAny(words)
		}
		object.Set("block_commands", commands)
	}
	if len(p.BlockSQL.Clients) > 0 {
		object.Set("block_sql", omap.Of[any]("clients", toAny(p.BlockSQL.Clients), "statements", toAny(p.BlockSQL.Statements)))
	}
	if len(p.BlockMigrations.Tools) > 0 {
		object.Set("block_migrations", omap.Of[any]("tools", toAny(p.BlockMigrations.Tools), "local_hosts", toAny(p.BlockMigrations.LocalHosts)))
	}
	return object
}

// backendRulesOf reads the optional `block_commands`, `block_sql` and `block_migrations`; a value of the wrong
// shape is an error, which lets the call through like every other broken policy.
func backendRulesOf(object *omap.Map[any], p *Policy) error {
	if raw := object.Value("block_commands"); raw != nil {
		list, ok := raw.([]any)
		if !ok {
			return errors.New("block_commands is not a list")
		}
		for _, item := range list {
			words, ok := stringItems(item)
			if !ok || len(words) == 0 {
				return errors.New("block_commands holds an entry that is not a list of strings")
			}
			p.BlockCommands = append(p.BlockCommands, words)
		}
	}
	section := func(key, first, second string) ([]string, []string, error) {
		raw := object.Value(key)
		if raw == nil {
			return nil, nil, nil
		}
		m, ok := raw.(*omap.Map[any])
		if !ok {
			return nil, nil, errors.New(key + " is not an object")
		}
		a, okA := stringItems(m.Value(first))
		b, okB := stringItems(m.Value(second))
		if !okA || !okB {
			return nil, nil, errors.New(key + " holds a value that is not a list of strings")
		}
		return a, b, nil
	}
	var err error
	if p.BlockSQL.Clients, p.BlockSQL.Statements, err = section("block_sql", "clients", "statements"); err != nil {
		return err
	}
	p.BlockMigrations.Tools, p.BlockMigrations.LocalHosts, err = section("block_migrations", "tools", "local_hosts")
	return err
}

func stringItems(value any) ([]string, bool) {
	list, ok := value.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
}

// knowledgeCloneOf is the optional `knowledge_clone` of a policy object; a value that is not a string is an error.
func knowledgeCloneOf(object *omap.Map[any]) (string, error) {
	switch value := object.Value("knowledge_clone").(type) {
	case nil:
		return "", nil
	case string:
		return value, nil
	}
	return "", errors.New("knowledge_clone is not a string")
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
	clone, err := knowledgeCloneOf(object)
	if err != nil {
		return Policy{}, err
	}
	policy := Policy{BlockedGit: blocked, BlockFormatters: blockFormatters, PlansDir: plansDir, RoleCapability: roles, ForbiddenImports: forbidden, LocalPackages: local, KnowledgeClone: clone}
	if err := backendRulesOf(object, &policy); err != nil {
		return Policy{}, err
	}
	return policy, nil
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
