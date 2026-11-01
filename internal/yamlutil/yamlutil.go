// Package yamlutil parses YAML into plain values the way the Dart `yaml` package does,
// and gives those values Dart's string form and equality.
//
// Values are nil, bool, int, float64, string, []any and *omap.Map[any] (keys in document order).
package yamlutil

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// Parse reads one YAML document; an empty document is nil.
// Plain scalars resolve with the YAML 1.2 core schema as Dart's loader does: no yes/no booleans,
// no underscores in numbers, no timestamps.
func Parse(text string) (any, error) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(text), &node); err != nil {
		return nil, err
	}
	if node.Kind == 0 || len(node.Content) == 0 {
		return nil, nil
	}
	return convert(node.Content[0])
}

func convert(node *yaml.Node) (any, error) {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil, nil
		}
		return convert(node.Content[0])
	case yaml.AliasNode:
		return convert(node.Alias)
	case yaml.SequenceNode:
		list := make([]any, 0, len(node.Content))
		for _, item := range node.Content {
			value, err := convert(item)
			if err != nil {
				return nil, err
			}
			list = append(list, value)
		}
		return list, nil
	case yaml.MappingNode:
		out := omap.New[any]()
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, err := convert(node.Content[i])
			if err != nil {
				return nil, err
			}
			name := DartString(key)
			if out.Has(name) {
				return nil, fmt.Errorf("line %d: duplicate mapping key %q", node.Content[i].Line, name)
			}
			value, err := convert(node.Content[i+1])
			if err != nil {
				return nil, err
			}
			out.Set(name, value)
		}
		return out, nil
	case yaml.ScalarNode:
		return scalar(node)
	}
	return nil, fmt.Errorf("line %d: unsupported YAML node", node.Line)
}

func scalar(node *yaml.Node) (any, error) {
	explicit := node.Tag != "" && node.Tag != "!" && node.Style&yaml.TaggedStyle != 0
	if explicit {
		switch node.ShortTag() {
		case "!!str":
			return node.Value, nil
		case "!!null":
			if isNull(node.Value) {
				return nil, nil
			}
			return nil, fmt.Errorf("line %d: invalid null scalar", node.Line)
		case "!!bool":
			if value, ok := boolValue(node.Value); ok {
				return value, nil
			}
			return nil, fmt.Errorf("line %d: invalid bool scalar", node.Line)
		case "!!int":
			if value, ok := number(node.Value, true, false); ok {
				return value, nil
			}
			return nil, fmt.Errorf("line %d: invalid int scalar", node.Line)
		case "!!float":
			if value, ok := number(node.Value, false, true); ok {
				return value, nil
			}
			return nil, fmt.Errorf("line %d: invalid float scalar", node.Line)
		default:
			return nil, fmt.Errorf("line %d: undefined tag: %s", node.Line, node.Tag)
		}
	}
	if node.Style&(yaml.SingleQuotedStyle|yaml.DoubleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 || node.Tag == "!" {
		return node.Value, nil
	}
	value := node.Value
	if value == "" {
		return nil, nil
	}
	switch first := value[0]; {
	case first == '.' || first == '+' || first == '-':
		if n, ok := number(value, true, true); ok {
			return n, nil
		}
	case first == 'n' || first == 'N' || first == '~':
		if isNull(value) {
			return nil, nil
		}
	case first == 't' || first == 'T' || first == 'f' || first == 'F':
		if b, ok := boolValue(value); ok {
			return b, nil
		}
	case first >= '0' && first <= '9':
		if n, ok := number(value, true, true); ok {
			return n, nil
		}
	}
	return value, nil
}

func isNull(value string) bool {
	switch value {
	case "", "null", "Null", "NULL", "~":
		return true
	}
	return false
}

func boolValue(value string) (bool, bool) {
	switch value {
	case "true", "True", "TRUE":
		return true, true
	case "false", "False", "FALSE":
		return false, true
	}
	return false, false
}

var (
	decimalInt = regexp.MustCompile(`^[+-]?[0-9]+$`)
	hexInt     = regexp.MustCompile(`^0x[0-9a-fA-F]+$`)
	dartDouble = regexp.MustCompile(`^[+-]?([0-9]+\.?[0-9]*|\.[0-9]+)([eE][+-]?[0-9]+)?$`)
)

// number mirrors the Dart loader's _parseNumberValue.
func number(contents string, allowInt, allowFloat bool) (any, bool) {
	if contents == "" {
		return nil, false
	}
	first := contents[0]
	if allowInt && len(contents) == 1 {
		if first >= '0' && first <= '9' {
			return int(first - '0'), true
		}
		return nil, false
	}
	if len(contents) < 2 {
		return nil, false
	}
	second := contents[1]
	if allowInt && first == '0' {
		if second == 'x' {
			if hexInt.MatchString(contents) {
				if n, err := strconv.ParseInt(contents[2:], 16, 64); err == nil {
					return int(n), true
				}
			}
			return nil, false
		}
		if second == 'o' {
			if n, err := strconv.ParseInt(contents[2:], 8, 64); err == nil && !strings.ContainsAny(contents[2:], "+-_") {
				return int(n), true
			}
			return nil, false
		}
	}
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	if isDigit(first) || ((first == '+' || first == '-') && isDigit(second)) {
		if allowInt && decimalInt.MatchString(contents) {
			if n, err := strconv.ParseInt(contents, 10, 64); err == nil {
				return int(n), true
			}
		}
		if allowFloat && dartDouble.MatchString(contents) {
			if f, err := strconv.ParseFloat(contents, 64); err == nil || math.IsInf(f, 0) {
				return f, true
			}
		}
		return nil, false
	}
	if !allowFloat {
		return nil, false
	}
	if (first == '.' && isDigit(second)) || ((first == '-' || first == '+') && second == '.') {
		switch contents {
		case "+.inf", "+.Inf", "+.INF":
			return math.Inf(1), true
		case "-.inf", "-.Inf", "-.INF":
			return math.Inf(-1), true
		}
		if dartDouble.MatchString(contents) {
			if f, err := strconv.ParseFloat(contents, 64); err == nil {
				return f, true
			}
		}
		return nil, false
	}
	if len(contents) == 4 && first == '.' {
		switch contents {
		case ".inf", ".Inf", ".INF":
			return math.Inf(1), true
		case ".nan", ".NaN", ".NAN":
			return math.NaN(), true
		}
	}
	return nil, false
}

// DartString is the value's Dart toString(): lists as `[a, b]`, maps as `{k: v}`, doubles with a fraction.
func DartString(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return DartDouble(v)
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = DartString(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []string:
		return "[" + strings.Join(v, ", ") + "]"
	case *omap.Map[any]:
		parts := make([]string, 0, v.Len())
		for _, key := range v.Keys() {
			parts = append(parts, key+": "+DartString(v.Value(key)))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sortStrings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, key+": "+DartString(v[key]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprint(value)
}

// DartDouble formats a double as Dart's double.toString does.
func DartDouble(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	}
	abs := math.Abs(v)
	if abs >= 1e21 || (abs != 0 && abs < 1e-6) {
		s := strconv.FormatFloat(v, 'e', -1, 64)
		mantissa, exponent, _ := strings.Cut(s, "e")
		sign := exponent[0]
		digits := strings.TrimLeft(exponent[1:], "0")
		return mantissa + "e" + string(sign) + digits
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Equal is Dart's == on parsed values: numbers compare by value across int and double,
// lists and maps only by identity, which parsed values never share.
func Equal(a, b any) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case string:
		y, ok := b.(string)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case int, int64, float64:
		fx, okx := toFloat(x)
		fy, oky := toFloat(b)
		return okx && oky && fx == fy
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// LookupPath follows a dotted path (`policy.agent_commit`) through nested maps; nil when absent.
func LookupPath(context any, dottedPath string) any {
	current := context
	for _, segment := range strings.Split(dottedPath, ".") {
		switch m := current.(type) {
		case *omap.Map[any]:
			current = m.Value(segment)
		case map[string]any:
			current = m[segment]
		default:
			return nil
		}
	}
	return current
}
