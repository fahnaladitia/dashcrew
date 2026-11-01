// Package dartjson reads JSON keeping key order and writes it exactly as Dart's `jsonEncode` and
// `JsonEncoder.withIndent` do, so merged config files stay byte-identical with the Dart engine.
//
// Values are nil, bool, int, float64, string, []any and *omap.Map[any].
package dartjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Decode parses one JSON value; objects keep their key order, integers become int.
func Decode(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	value, err := decodeValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after the JSON value")
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	tok, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			out := omap.New[any]()
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				value, err := decodeValue(decoder)
				if err != nil {
					return nil, err
				}
				out.Set(keyToken.(string), value)
			}
			_, err := decoder.Token()
			return out, err
		case '[':
			list := []any{}
			for decoder.More() {
				value, err := decodeValue(decoder)
				if err != nil {
					return nil, err
				}
				list = append(list, value)
			}
			_, err := decoder.Token()
			return list, err
		}
	case json.Number:
		text := t.String()
		if !strings.ContainsAny(text, ".eE") {
			if n, err := strconv.ParseInt(text, 10, 64); err == nil {
				return int(n), nil
			}
		}
		return t.Float64()
	case string, bool, nil:
		return t, nil
	}
	return nil, fmt.Errorf("unexpected JSON token %v", tok)
}

// Encode is Dart's `jsonEncode`: no whitespace at all.
func Encode(value any) (string, error) {
	var b bytes.Buffer
	if err := write(&b, value, "", 0); err != nil {
		return "", err
	}
	return b.String(), nil
}

// EncodeIndent is Dart's `JsonEncoder.withIndent(indent)`.
func EncodeIndent(value any, indent string) (string, error) {
	var b bytes.Buffer
	if err := write(&b, value, indent, 0); err != nil {
		return "", err
	}
	return b.String(), nil
}

func write(b *bytes.Buffer, value any, indent string, depth int) error {
	switch v := value.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case int:
		b.WriteString(strconv.Itoa(v))
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("converting object to an encodable object failed: %s", yamlutil.DartDouble(v))
		}
		b.WriteString(yamlutil.DartDouble(v))
	case string:
		writeString(b, v)
	case []string:
		items := make([]any, len(v))
		for i, item := range v {
			items[i] = item
		}
		return write(b, items, indent, depth)
	case []any:
		if len(v) == 0 {
			b.WriteString("[]")
			return nil
		}
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			newline(b, indent, depth+1)
			if err := write(b, item, indent, depth+1); err != nil {
				return err
			}
		}
		newline(b, indent, depth)
		b.WriteByte(']')
	case *omap.Map[any]:
		if v.Len() == 0 {
			b.WriteString("{}")
			return nil
		}
		b.WriteByte('{')
		for i, key := range v.Keys() {
			if i > 0 {
				b.WriteByte(',')
			}
			newline(b, indent, depth+1)
			writeString(b, key)
			b.WriteByte(':')
			if indent != "" {
				b.WriteByte(' ')
			}
			if err := write(b, v.Value(key), indent, depth+1); err != nil {
				return err
			}
		}
		newline(b, indent, depth)
		b.WriteByte('}')
	default:
		return fmt.Errorf("converting object to an encodable object failed: %T", value)
	}
	return nil
}

func newline(b *bytes.Buffer, indent string, depth int) {
	if indent == "" {
		return
	}
	b.WriteByte('\n')
	for i := 0; i < depth; i++ {
		b.WriteString(indent)
	}
}

// writeString escapes as Dart does: quote, backslash and control characters only, \b \t \n \f \r by name,
// other controls and unpaired surrogates as lowercase \u escapes.
func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(b, `\u%04x`, s[i])
			i++
			continue
		}
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20:
			fmt.Fprintf(b, `\u%04x`, r)
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	b.WriteByte('"')
}
