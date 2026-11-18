package initproject

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var requires = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([A-Za-z0-9.\-_/]+)\s+v[0-9]`)

// goModuleRules map a module path (and every path below it) to the pack of the built-in go platform it implies, in
// the order GoDefaults prefers them.
var goModuleRules = []struct{ module, family, option string }{
	{"github.com/go-chi/chi", "http", "chi"},
	{"github.com/gin-gonic/gin", "http", "gin"},
	{"github.com/labstack/echo", "http", "echo"},
	{"github.com/gofiber/fiber", "http", "fiber"},
	{"gorm.io/gorm", "db", "gorm"},
	{"github.com/jackc/pgx", "db", "pgx"},
}

// sqlcFiles are the configuration files that mean the project generates its queries with sqlc.
var sqlcFiles = []string{"sqlc.yaml", "sqlc.yml", "sqlc.json"}

// goEvidence is one pack that go.mod or a sqlc file in projectDir points at.
type goEvidence struct{ family, option, evidence string }

// goPacksIn reads go.mod (its required modules) and the sqlc files of projectDir: one entry per matching pack, with
// what matched, sqlc first, then in goModuleRules order.
func goPacksIn(projectDir string) []goEvidence {
	var found []goEvidence
	for _, file := range sqlcFiles {
		if isFile(filepath.Join(projectDir, file)) {
			found = append(found, goEvidence{"db", "sqlc", file})
			break
		}
	}
	data, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		return found
	}
	var modules []string
	for _, match := range requires.FindAllStringSubmatch(string(data), -1) {
		modules = append(modules, match[1])
	}
	sort.Strings(modules)
	for _, rule := range goModuleRules {
		for _, module := range modules {
			if module == rule.module || strings.HasPrefix(module, rule.module+"/") {
				found = append(found, goEvidence{rule.family, rule.option, module + " in go.mod"})
				break
			}
		}
	}
	return found
}

// GoDefaults are the http and db packs of the built-in go platform that projectDir points at, one per family:
// sqlc over gorm over pgx, and the first HTTP framework of goModuleRules; "nethttp" and "none" when nothing
// matches, and when there is no go.mod.
func GoDefaults(projectDir string) (http, db string) {
	http, db = "nethttp", "none"
	seen := map[string]bool{}
	for _, pack := range goPacksIn(projectDir) {
		if seen[pack.family] {
			continue
		}
		seen[pack.family] = true
		if pack.family == "http" {
			http = pack.option
		} else {
			db = pack.option
		}
	}
	return http, db
}
