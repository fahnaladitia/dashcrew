// Package mustache is a port of the subset of Dart's mustache_template 2.0 that the catalog uses:
// variables, sections and inverted sections, in strict mode with no HTML escaping.
// Scanning, standalone-line handling and value resolution follow the Dart library line by line, so rendered
// whitespace is byte-identical. Partials, lambdas and delimiter changes are not supported and fail.
package mustache

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Error is a template error; Message is the Dart library's message, Offset the rune offset in the source.
type Error struct {
	Message string
	Offset  int
}

func (e *Error) Error() string { return e.Message }

// Render parses source and renders it against context.
// Context values are nil, bool, numbers, strings, []any, []string, map[string]any and *omap.Map[any].
func Render(source string, context any) (string, error) {
	nodes, err := parse(source)
	if err != nil {
		return "", err
	}
	r := &renderer{stack: []any{context}}
	if err := r.render(nodes); err != nil {
		return "", err
	}
	return r.out.String(), nil
}

// ---- scanner ----

type tokenType int

const (
	tText tokenType = iota
	tOpenDelimiter
	tCloseDelimiter
	tIdentifier
	tDot
	tSigil
	tWhitespace
	tLineEnd
)

func (t tokenType) String() string {
	return [...]string{"text", "openDelimiter", "closeDelimiter", "identifier", "dot", "sigil", "whitespace", "lineEnd"}[t]
}

type token struct {
	typ        tokenType
	value      string
	start, end int
}

const eof = -1

type scanner struct {
	runes  []rune
	offset int
	tokens []token
}

func (s *scanner) peek() rune {
	if s.offset < len(s.runes) {
		return s.runes[s.offset]
	}
	return eof
}

func (s *scanner) read() rune {
	c := s.peek()
	if c != eof {
		s.offset++
	}
	return c
}

func (s *scanner) readWhile(test func(rune) bool) string {
	var b strings.Builder
	for s.peek() != eof && test(s.peek()) {
		b.WriteRune(s.read())
	}
	return b.String()
}

func (s *scanner) expect(expected rune) error {
	c := s.read()
	if c == eof {
		return &Error{Message: "Unexpected end of input", Offset: s.offset - 1}
	}
	if c != expected {
		return &Error{Message: fmt.Sprintf("Unexpected character, expected: %c, was: %c", expected, c), Offset: s.offset - 1}
	}
	return nil
}

func (s *scanner) add(typ tokenType, value string, start, end int) {
	s.tokens = append(s.tokens, token{typ, value, start, end})
}

func isWhitespace(c rune) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func isSigil(c rune) bool {
	return c == '#' || c == '^' || c == '/' || c == '>' || c == '&' || c == '!'
}

func scan(source string) ([]token, error) {
	s := &scanner{runes: []rune(source)}
	for c := s.peek(); c != eof; c = s.peek() {
		if c != '{' {
			s.scanText()
			continue
		}
		start := s.offset
		s.read()
		if s.peek() != '{' {
			s.add(tText, "{", start, s.offset)
			continue
		}
		if err := s.expect('{'); err != nil {
			return nil, err
		}
		if s.peek() == '{' {
			s.read()
			s.add(tOpenDelimiter, "{{{", start, s.offset)
			s.scanTagContent()
			if s.peek() != eof {
				closeStart := s.offset
				for i := 0; i < 3; i++ {
					if err := s.expect('}'); err != nil {
						return nil, err
					}
				}
				s.add(tCloseDelimiter, "}}}", closeStart, s.offset)
			}
			continue
		}
		wsStart := s.offset
		ws := s.readWhile(isWhitespace)
		if s.peek() == '=' {
			return nil, &Error{Message: "Change delimiter tags are not supported.", Offset: start}
		}
		s.add(tOpenDelimiter, "{{", start, wsStart)
		if ws != "" {
			s.add(tWhitespace, ws, wsStart, s.offset)
		}
		s.scanTagContent()
		if s.peek() != eof {
			closeStart := s.offset
			if err := s.expect('}'); err != nil {
				return nil, err
			}
			if err := s.expect('}'); err != nil {
				return nil, err
			}
			s.add(tCloseDelimiter, "}}", closeStart, s.offset)
		}
	}
	return s.tokens, nil
}

func (s *scanner) scanText() {
	for c := s.peek(); c != eof && c != '{'; c = s.peek() {
		start := s.offset
		var typ tokenType
		var value string
		switch c {
		case ' ', '\t':
			value = s.readWhile(func(c rune) bool { return c == ' ' || c == '\t' })
			typ = tWhitespace
		case '\n':
			s.read()
			typ, value = tLineEnd, "\n"
		case '\r':
			s.read()
			if s.peek() == '\n' {
				s.read()
				typ, value = tLineEnd, "\r\n"
			} else {
				typ, value = tText, "\r"
			}
		default:
			value = s.readWhile(func(c rune) bool { return c != '{' && c != '\n' })
			typ = tText
		}
		s.add(typ, value, start, s.offset)
	}
}

func (s *scanner) scanTagContent() {
	for c := s.peek(); c != eof && c != '}'; c = s.peek() {
		start := s.offset
		var typ tokenType
		var value string
		switch {
		case isSigil(c):
			s.read()
			typ, value = tSigil, string(c)
		case isWhitespace(c):
			typ, value = tWhitespace, s.readWhile(isWhitespace)
		case c == '.':
			s.read()
			typ, value = tDot, "."
		default:
			typ = tIdentifier
			value = s.readWhile(func(c rune) bool {
				return !isSigil(c) && !isWhitespace(c) && c != '.' && c != '}'
			})
		}
		s.add(typ, value, start, s.offset)
	}
}

// ---- parser ----

type tagType int

const (
	tagOpenSection tagType = iota
	tagOpenInverseSection
	tagCloseSection
	tagVariable
	tagTripleMustache
	tagUnescapedVariable
	tagPartial
	tagComment
)

type tag struct {
	typ        tagType
	name       string
	start, end int
}

type node interface{}

type textNode struct{ text string }

type variableNode struct {
	name  string
	start int
}

type sectionNode struct {
	name     string
	start    int
	inverse  bool
	children []node
}

type parser struct {
	tokens []token
	offset int
	stack  []*sectionNode
	length int
}

var validIdentifier = regexp.MustCompile(`^[0-9a-zA-Z_\-.]+$`)

func parse(source string) ([]node, error) {
	tokens, err := scan(source)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens, stack: []*sectionNode{{name: "root"}}, length: len([]rune(source))}

	if lineEnd := p.readIf(tLineEnd); lineEnd != nil {
		p.appendText(*lineEnd)
	}
	if err := p.parseLine(); err != nil {
		return nil, err
	}

	for t := p.peek(); t != nil; t = p.peek() {
		switch t.typ {
		case tText, tWhitespace:
			p.read()
			p.appendText(*t)
		case tOpenDelimiter:
			tg, err := p.readTag()
			if err != nil {
				return nil, err
			}
			if tg != nil {
				if err := p.appendTag(tg, p.nodeFromTag(tg)); err != nil {
					return nil, err
				}
			}
		case tLineEnd:
			p.appendText(*p.read())
			if err := p.parseLine(); err != nil {
				return nil, err
			}
		default:
			return nil, &Error{Message: "Unreachable code.", Offset: t.start}
		}
	}

	if len(p.stack) != 1 {
		last := p.stack[len(p.stack)-1]
		return nil, &Error{Message: fmt.Sprintf("Unclosed tag: '%s'.", last.name), Offset: last.start}
	}
	return p.stack[0].children, nil
}

func (p *parser) peek() *token {
	if p.offset < len(p.tokens) {
		return &p.tokens[p.offset]
	}
	return nil
}

func (p *parser) read() *token {
	t := p.peek()
	if t != nil {
		p.offset++
	}
	return t
}

func (p *parser) readIf(typ tokenType) *token {
	if t := p.peek(); t != nil && t.typ == typ {
		return p.read()
	}
	return nil
}

func (p *parser) errorEOF() error {
	return &Error{Message: "Unexpected end of input.", Offset: p.length - 1}
}

func (p *parser) appendText(t token) {
	top := p.stack[len(p.stack)-1]
	if n := len(top.children); n > 0 {
		if last, ok := top.children[n-1].(*textNode); ok {
			last.text += t.value
			return
		}
	}
	top.children = append(top.children, &textNode{text: t.value})
}

func (p *parser) appendTag(tg *tag, n node) error {
	top := p.stack[len(p.stack)-1]
	switch tg.typ {
	case tagOpenSection, tagOpenInverseSection:
		section := n.(*sectionNode)
		top.children = append(top.children, section)
		p.stack = append(p.stack, section)
	case tagCloseSection:
		if tg.name != top.name {
			return &Error{Message: fmt.Sprintf("Mismatched tag, expected: '%s', was: '%s'", top.name, tg.name), Offset: tg.start}
		}
		p.stack = p.stack[:len(p.stack)-1]
	case tagVariable, tagUnescapedVariable, tagTripleMustache:
		top.children = append(top.children, n)
	case tagPartial:
		return &Error{Message: fmt.Sprintf("Partial not found: %s.", tg.name), Offset: tg.start}
	case tagComment:
	}
	return nil
}

// parseLine drops the whitespace and line end around a tag that stands alone on its line.
func (p *parser) parseLine() error {
	if t := p.peek(); t != nil && t.typ == tLineEnd {
		p.appendText(*t)
	}
	for p.peek() != nil {
		p.readIf(tLineEnd)
		preceding := p.readIf(tWhitespace)
		tg, err := p.readTag()
		if err != nil {
			return err
		}
		n := p.nodeFromTag(tg)
		following := p.readIf(tWhitespace)

		standalone := tg != nil && (p.peek() == nil || p.peek().typ == tLineEnd) &&
			(tg.typ == tagOpenSection || tg.typ == tagCloseSection || tg.typ == tagOpenInverseSection ||
				tg.typ == tagPartial || tg.typ == tagComment)
		if standalone {
			if err := p.appendTag(tg, n); err != nil {
				return err
			}
			continue
		}
		if preceding != nil {
			p.appendText(*preceding)
		}
		if tg != nil {
			if err := p.appendTag(tg, n); err != nil {
				return err
			}
		}
		if following != nil {
			p.appendText(*following)
		}
		break
	}
	return nil
}

func (p *parser) readTag() (*tag, error) {
	t := p.peek()
	if t == nil || t.typ != tOpenDelimiter {
		return nil, nil
	}
	open := p.read()
	p.readIf(tWhitespace)

	typ := tagVariable
	if open.value == "{{{" {
		typ = tagTripleMustache
	} else if sigil := p.readIf(tSigil); sigil != nil {
		typ = map[string]tagType{
			"#": tagOpenSection, "^": tagOpenInverseSection, "/": tagCloseSection,
			"&": tagUnescapedVariable, ">": tagPartial, "!": tagComment,
		}[sigil.value]
	}
	p.readIf(tWhitespace)

	var b strings.Builder
	for t := p.peek(); t != nil && t.typ != tCloseDelimiter; t = p.peek() {
		p.read()
		b.WriteString(t.value)
	}
	name := strings.TrimSpace(b.String())
	if p.peek() == nil {
		return nil, p.errorEOF()
	}
	if typ != tagComment {
		if name == "" {
			return nil, &Error{Message: "Empty tag name.", Offset: open.start}
		}
		if strings.ContainsAny(name, "\t\n\r") {
			return nil, &Error{Message: "Tags may not contain newlines or tabs.", Offset: open.start}
		}
		if !validIdentifier.MatchString(name) {
			return nil, &Error{
				Message: "Unless in lenient mode, tags may only contain the characters a-z, A-Z, minus, underscore and period.",
				Offset:  open.start,
			}
		}
	}
	closeToken := p.read()
	return &tag{typ: typ, name: name, start: open.start, end: closeToken.end}, nil
}

func (p *parser) nodeFromTag(tg *tag) node {
	if tg == nil {
		return nil
	}
	switch tg.typ {
	case tagOpenSection, tagOpenInverseSection:
		return &sectionNode{name: tg.name, start: tg.start, inverse: tg.typ == tagOpenInverseSection}
	case tagVariable, tagUnescapedVariable, tagTripleMustache:
		return &variableNode{name: tg.name, start: tg.start}
	}
	return nil
}

// ---- renderer ----

type missing struct{}

var noSuchProperty = missing{}

type renderer struct {
	stack []any
	out   strings.Builder
}

func (r *renderer) render(nodes []node) error {
	for _, n := range nodes {
		switch n := n.(type) {
		case *textNode:
			r.out.WriteString(n.text)
		case *variableNode:
			value := r.resolve(n.name)
			if value == noSuchProperty {
				return &Error{Message: fmt.Sprintf("Value was missing for variable tag: %s.", n.name), Offset: n.start}
			}
			if value != nil {
				r.out.WriteString(yamlutil.DartString(value))
			}
		case *sectionNode:
			var err error
			if n.inverse {
				err = r.inverseSection(n)
			} else {
				err = r.section(n)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *renderer) with(n *sectionNode, value any) error {
	r.stack = append(r.stack, value)
	err := r.render(n.children)
	r.stack = r.stack[:len(r.stack)-1]
	return err
}

func (r *renderer) section(n *sectionNode) error {
	value := r.resolve(n.name)
	switch v := value.(type) {
	case nil:
		return nil
	case missing:
		return &Error{Message: fmt.Sprintf("Value was missing for section tag: %s.", n.name), Offset: n.start}
	case bool:
		if v {
			return r.with(n, v)
		}
		return nil
	}
	if items, ok := asList(value); ok {
		for _, item := range items {
			if err := r.with(n, item); err != nil {
				return err
			}
		}
		return nil
	}
	return r.with(n, value)
}

func (r *renderer) inverseSection(n *sectionNode) error {
	value := r.resolve(n.name)
	switch v := value.(type) {
	case nil:
		return r.with(n, nil)
	case missing:
		return &Error{Message: fmt.Sprintf("Value was missing for inverse section: %s.", n.name), Offset: n.start}
	case bool:
		if !v {
			return r.with(n, n.name)
		}
		return nil
	case map[string]any, *omap.Map[any]:
		return nil
	}
	if items, ok := asList(value); ok {
		if len(items) == 0 {
			return r.with(n, n.name)
		}
		return nil
	}
	return &Error{
		Message: fmt.Sprintf("Invalid value type for inverse section, section: %s, type: %s.", n.name, dartTypeName(value)),
		Offset:  n.start,
	}
}

func asList(value any) ([]any, bool) {
	switch v := value.(type) {
	case []any:
		return v, true
	case []string:
		items := make([]any, len(v))
		for i, item := range v {
			items[i] = item
		}
		return items, true
	}
	return nil, false
}

func dartTypeName(value any) string {
	switch value.(type) {
	case string:
		return "String"
	case int, int64:
		return "int"
	case float64:
		return "double"
	}
	return fmt.Sprintf("%T", value)
}

// resolve walks the stack for the first part of a dotted name, then follows the rest from there only.
func (r *renderer) resolve(name string) any {
	if name == "." {
		return r.stack[len(r.stack)-1]
	}
	parts := strings.Split(name, ".")
	object := any(noSuchProperty)
	for i := len(r.stack) - 1; i >= 0; i-- {
		object = property(r.stack[i], parts[0])
		if object != noSuchProperty {
			break
		}
	}
	for _, part := range parts[1:] {
		if object == noSuchProperty {
			return noSuchProperty
		}
		object = property(object, part)
	}
	return object
}

var integerTag = regexp.MustCompile(`^[0-9]+$`)

func property(object any, name string) any {
	switch m := object.(type) {
	case map[string]any:
		if value, ok := m[name]; ok {
			return value
		}
	case *omap.Map[any]:
		if value, ok := m.Get(name); ok {
			return value
		}
	}
	if items, ok := asList(object); ok && integerTag.MatchString(name) {
		var index int
		if _, err := fmt.Sscanf(name, "%d", &index); err == nil && index < len(items) {
			return items[index]
		}
	}
	return noSuchProperty
}
