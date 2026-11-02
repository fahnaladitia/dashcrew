package profile

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// PlatformGuard is a platform's `guard:` key (P10): commands the guard blocks for every agent in a project on that
// platform. Built-in and project platforms declare it in platform.yaml, a level 1 platform in `platform_config`.
type PlatformGuard struct {
	// Commands are a program and the leading words that follow it, options skipped: `[terraform, apply]`.
	Commands [][]string
	// SQLClients run SQL; a statement in SQLStatements in the SQL they are given is blocked.
	SQLClients, SQLStatements []string
	// MigrationTools are blocked when their connection string names a host outside LocalHosts.
	MigrationTools, LocalHosts []string
}

// Empty is true when the key declares nothing.
func (g *PlatformGuard) Empty() bool {
	return g == nil || (len(g.Commands) == 0 && len(g.SQLClients) == 0 && len(g.MigrationTools) == 0)
}

var (
	guardKeys     = []string{"commands", "sql", "migrations"}
	sqlKeys       = []string{"clients", "statements"}
	migrationKeys = []string{"tools", "local_hosts"}
	guardWord     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
	guardHost     = regexp.MustCompile(`^[A-Za-z0-9.:_-]+$`)
)

// ParsePlatformGuard reads a `guard:` value; nil without one. fail names the offending key, relative to `guard`.
func ParsePlatformGuard(value any, fail func(key, message string) error) (*PlatformGuard, error) {
	if value == nil {
		return nil, nil
	}
	config, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fail("guard", "expected a map with commands, sql and migrations")
	}
	for _, key := range config.Keys() {
		if !contains(guardKeys, key) {
			return nil, fail("guard."+key, "unknown key")
		}
	}
	g := &PlatformGuard{}
	if raw := config.Value("commands"); raw != nil {
		list, ok := raw.([]any)
		if !ok {
			return nil, fail("guard.commands", "expected a list of [program, subcommand...]")
		}
		for i, item := range list {
			words, ok := StringList(item)
			if !ok || len(words) == 0 || !allMatch(words, guardWord) {
				return nil, fail("guard.commands["+strconv.Itoa(i)+"]", "expected [program, subcommand...], plain words")
			}
			g.Commands = append(g.Commands, words)
		}
	}
	if raw := config.Value("sql"); raw != nil {
		sql, err := guardSection(raw, "guard.sql", sqlKeys, fail)
		if err != nil {
			return nil, err
		}
		if g.SQLClients, err = guardWords(sql, "guard.sql", "clients", guardWord, true, fail); err != nil {
			return nil, err
		}
		if g.SQLStatements, err = guardWords(sql, "guard.sql", "statements", guardWord, true, fail); err != nil {
			return nil, err
		}
		for i, statement := range g.SQLStatements {
			g.SQLStatements[i] = strings.ToUpper(statement)
		}
	}
	if raw := config.Value("migrations"); raw != nil {
		migrations, err := guardSection(raw, "guard.migrations", migrationKeys, fail)
		if err != nil {
			return nil, err
		}
		if g.MigrationTools, err = guardWords(migrations, "guard.migrations", "tools", guardWord, true, fail); err != nil {
			return nil, err
		}
		if g.LocalHosts, err = guardWords(migrations, "guard.migrations", "local_hosts", guardHost, false, fail); err != nil {
			return nil, err
		}
		if g.LocalHosts == nil {
			g.LocalHosts = []string{"localhost", "127.0.0.1", "::1"}
		}
	}
	return g, nil
}

func guardSection(raw any, path string, keys []string, fail func(key, message string) error) (*omap.Map[any], error) {
	section, ok := raw.(*omap.Map[any])
	if !ok {
		return nil, fail(path, "expected a map with "+strings.Join(keys, " and "))
	}
	for _, key := range section.Keys() {
		if !contains(keys, key) {
			return nil, fail(path+"."+key, "unknown key")
		}
	}
	return section, nil
}

func guardWords(section *omap.Map[any], path, key string, shape *regexp.Regexp, required bool,
	fail func(key, message string) error) ([]string, error) {
	raw := section.Value(key)
	if raw == nil {
		if required {
			return nil, fail(path+"."+key, "required, a non-empty list")
		}
		return nil, nil
	}
	words, ok := StringList(raw)
	if !ok || len(words) == 0 || !allMatch(words, shape) {
		return nil, fail(path+"."+key, "expected a non-empty list of plain words")
	}
	return words, nil
}

func allMatch(words []string, shape *regexp.Regexp) bool {
	for _, word := range words {
		if !shape.MatchString(word) {
			return false
		}
	}
	return true
}
