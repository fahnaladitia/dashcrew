package mustache

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// testdata/dart_cases.json holds templates, contexts and the output or error message that Dart's
// mustache_template 2.0.4 gives for each (strict mode, no HTML escaping), as the Dart engine renders.
type dartCase struct {
	Template string          `json:"template"`
	Context  json.RawMessage `json:"context"`
	Out      *string         `json:"out"`
	Error    *string         `json:"error"`
}

func decodeContext(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return plain(value)
}

func plain(value any) any {
	switch v := value.(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
		f, _ := v.Float64()
		return f
	case map[string]any:
		for key, item := range v {
			v[key] = plain(item)
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = plain(item)
		}
		return v
	}
	return value
}

func TestMatchesDartMustacheTemplate(t *testing.T) {
	data, err := os.ReadFile("testdata/dart_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []dartCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		got, err := Render(c.Template, decodeContext(t, c.Context))
		switch {
		case c.Error != nil:
			var templateErr *Error
			if !errors.As(err, &templateErr) || templateErr.Message != *c.Error {
				t.Errorf("%q: error %v, want %q", c.Template, err, *c.Error)
			}
		case err != nil:
			t.Errorf("%q: unexpected error %v", c.Template, err)
		case got != *c.Out:
			t.Errorf("%q:\n got %q\nwant %q", c.Template, got, *c.Out)
		}
	}
}

func TestUnsupportedFeaturesFail(t *testing.T) {
	for _, source := range []string{"{{=<% %>=}}", "{{> partial}}"} {
		if _, err := Render(source, map[string]any{}); err == nil {
			t.Errorf("%q: expected an error", source)
		}
	}
}
