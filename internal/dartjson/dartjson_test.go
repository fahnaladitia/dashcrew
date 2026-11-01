package dartjson

import (
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

func TestEncodeMatchesDartJsonEncode(t *testing.T) {
	value := omap.Of[any](
		"z", "a\"b\\c\n\t\r\b\f\x01 </> & é",
		"a", []any{1, 2.5, 3.0, true, nil},
		"empty", omap.New[any](),
		"list", []any{},
	)
	compact, err := Encode(value)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"z":"a\"b\\c\n\t\r\b\f\u0001 </> & é","a":[1,2.5,3.0,true,null],"empty":{},"list":[]}`
	if compact != want {
		t.Errorf("compact:\n got %s\nwant %s", compact, want)
	}

	indented, err := EncodeIndent(value, "  ")
	if err != nil {
		t.Fatal(err)
	}
	wantIndented := `{
  "z": "a\"b\\c\n\t\r\b\f\u0001 </> & é",
  "a": [
    1,
    2.5,
    3.0,
    true,
    null
  ],
  "empty": {},
  "list": []
}`
	if indented != wantIndented {
		t.Errorf("indented:\n got %s\nwant %s", indented, wantIndented)
	}
}

func TestDecodeKeepsOrderAndIntegers(t *testing.T) {
	value, err := Decode(`{"b": 1, "a": {"y": 2.0, "x": [1, "s"]}}`)
	if err != nil {
		t.Fatal(err)
	}
	m := value.(*omap.Map[any])
	if keys := m.Keys(); keys[0] != "b" || keys[1] != "a" {
		t.Errorf("keys %v", keys)
	}
	if m.Value("b") != 1 {
		t.Errorf("b %#v", m.Value("b"))
	}
	inner := m.Value("a").(*omap.Map[any])
	if inner.Keys()[0] != "y" || inner.Value("y") != 2.0 {
		t.Errorf("inner %#v", inner)
	}
	again, _ := Encode(value)
	if again != `{"b":1,"a":{"y":2.0,"x":[1,"s"]}}` {
		t.Errorf("round trip %s", again)
	}
}

func TestDecodeRejectsTrailingData(t *testing.T) {
	if _, err := Decode(`{} {}`); err == nil {
		t.Error("expected an error")
	}
}
