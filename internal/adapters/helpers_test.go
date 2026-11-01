package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/render"
)

var (
	shippedOnce    sync.Once
	shippedCatalog *catalog.Catalog
)

func shipped(t *testing.T) *catalog.Catalog {
	t.Helper()
	shippedOnce.Do(func() {
		var err error
		if shippedCatalog, err = catalog.LoadEmbedded(); err != nil {
			t.Fatal(err)
		}
	})
	return shippedCatalog
}

// files renders yaml with the shipped catalog and returns what adapter writes, with no lock.
func files(t *testing.T, c *catalog.Catalog, adapter Adapter, yaml string) map[string]string {
	t.Helper()
	p, err := profile.Parse(yaml, c.KnownOptions())
	if err != nil {
		t.Fatal(err)
	}
	r := render.New(c)
	targets, err := r.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	out, err := adapter.Files(p, targets, r.ContextFor(p), map[string]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return plain(out)
}

// plain is files as a Go map, for tests that do not look at the order.
func plain(files *omap.Map[string]) map[string]string {
	out := map[string]string{}
	for _, path := range files.Keys() {
		out[path] = files.Value(path)
	}
	return out
}

func writeFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(root, relative string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
	return err == nil
}

func decodeMap(t *testing.T, text string) *omap.Map[any] {
	t.Helper()
	value, err := dartjson.Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	return value.(*omap.Map[any])
}

func compact(t *testing.T, value any) string {
	t.Helper()
	text, err := dartjson.Encode(value)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func keysWithPrefix(files map[string]string, prefix string) []string {
	var out []string
	for path := range files {
		if strings.HasPrefix(path, prefix) {
			out = append(out, path)
		}
	}
	return out
}

func renderFor(t *testing.T, c *catalog.Catalog, yaml string) (*profile.Profile, []render.RenderedTarget, map[string]any) {
	t.Helper()
	p, err := profile.Parse(yaml, c.KnownOptions())
	if err != nil {
		t.Fatal(err)
	}
	r := render.New(c)
	targets, err := r.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, targets, r.ContextFor(p)
}
