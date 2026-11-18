package initproject

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoDefaults(t *testing.T) {
	cases := []struct{ gomod, extra, http, db string }{
		{"", "", "nethttp", "none"},
		{"require github.com/go-chi/chi/v5 v5.2.1\nrequire github.com/jackc/pgx/v5 v5.7.4\n", "", "chi", "pgx"},
		{"require (\n\tgithub.com/gin-gonic/gin v1.10.0\n\tgorm.io/gorm v1.25.12\n)\n", "", "gin", "gorm"},
		{"require (\n\tgithub.com/labstack/echo/v4 v4.13.3\n\tgithub.com/jackc/pgx/v5 v5.7.4\n)\n", "sqlc.yaml", "echo", "sqlc"},
		{"require github.com/gofiber/fiber/v2 v2.52.6\n", "", "fiber", "none"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		if c.gomod != "" {
			os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.26\n\n"+c.gomod), 0o644)
		}
		if c.extra != "" {
			os.WriteFile(filepath.Join(dir, c.extra), []byte("version: \"2\"\n"), 0o644)
		}
		if http, db := GoDefaults(dir); http != c.http || db != c.db {
			t.Errorf("%q: %s %s, want %s %s", c.gomod, http, db, c.http, c.db)
		}
	}
}
