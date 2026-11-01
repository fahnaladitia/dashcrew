// Package shell splits a shell script into the simple commands the guard checks, as the Dart guard's `shell.dart`.
//
// The scanner walks bytes. Every character it acts on is ASCII, and UTF-8 never puts an ASCII byte inside a
// multi-byte character, so the words it returns are those of the Dart scanner, which walks UTF-16 code units.
package shell

import "strings"

// Commands are the simple commands in script, as argv words; quoted text and heredoc bodies are data.
// Commands inside `$(...)` and backticks are returned too, also inside double quotes, after the top-level ones.
// A heredoc body is parsed as commands when its line runs a shell (`bash <<EOF`, `cat <<EOF | sh`).
func Commands(script string) [][]string {
	s := &scanner{source: script}
	return s.scan()
}

var shellReceivers = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "ssh": true, "eval": true, "source": true, ".": true}

type heredoc struct {
	delimiter string
	stripTabs bool
}

type scanner struct {
	source        string
	pos           int
	commands      [][]string
	substitutions [][]string
	current       []string
	word          strings.Builder
	inWord        bool
	pending       []heredoc
	lineStart     int
}

func lastSegment(word string) string {
	return word[strings.LastIndexByte(word, '/')+1:]
}

func (s *scanner) scan() [][]string {
	src := s.source
	for s.pos < len(src) {
		c := src[s.pos]
		switch {
		case c == '\n':
			s.endCommand()
			feedsShell := false
			for _, argv := range s.commands[s.lineStart:] {
				for _, w := range argv {
					if shellReceivers[lastSegment(w)] {
						feedsShell = true
					}
				}
			}
			s.pos++
			s.skipHeredocBodies(feedsShell)
			s.lineStart = len(s.commands)
		case c == ' ' || c == '\t':
			s.endWord()
			s.pos++
		case c == '#' && !s.inWord:
			for s.pos < len(src) && src[s.pos] != '\n' {
				s.pos++
			}
		case c == ';' || c == '(' || c == ')':
			s.endCommand()
			s.pos++
		case c == '&' || c == '|':
			s.endCommand()
			s.pos++
			if s.pos < len(src) && (src[s.pos] == c || src[s.pos] == '&') {
				s.pos++
			}
		case c == '<' || c == '>':
			s.redirect()
		case c == '\\':
			s.inWord = true
			if s.pos+1 < len(src) && src[s.pos+1] != '\n' {
				s.word.WriteByte(src[s.pos+1])
			}
			s.pos += 2
		case c == '\'':
			s.inWord = true
			stop := indexFrom(src, '\'', s.pos+1)
			s.word.WriteString(src[s.pos+1 : stop])
			s.pos = stop + 1
		case c == '"':
			s.inWord = true
			s.doubleQuoted()
		case c == '$' && s.peek(1) == '(':
			s.inWord = true
			s.substitution(s.pos+2, ')')
		case c == '`':
			s.inWord = true
			s.substitution(s.pos+1, '`')
		default:
			s.inWord = true
			s.word.WriteByte(c)
			s.pos++
		}
	}
	s.endCommand()
	return append(s.commands, s.substitutions...)
}

// indexFrom is the index of c in src at or after from, else len(src).
func indexFrom(src string, c byte, from int) int {
	if from > len(src) {
		return len(src)
	}
	if i := strings.IndexByte(src[from:], c); i >= 0 {
		return from + i
	}
	return len(src)
}

// peek is the byte offset after pos, or 0 past the end.
func (s *scanner) peek(offset int) byte {
	if s.pos+offset < len(s.source) {
		return s.source[s.pos+offset]
	}
	return 0
}

func (s *scanner) endWord() {
	if s.inWord {
		s.current = append(s.current, s.word.String())
	}
	s.word.Reset()
	s.inWord = false
}

func (s *scanner) endCommand() {
	s.endWord()
	if len(s.current) > 0 {
		s.commands = append(s.commands, s.current)
	}
	s.current = nil
}

func (s *scanner) doubleQuoted() {
	src := s.source
	s.pos++
	for s.pos < len(src) && src[s.pos] != '"' {
		c := src[s.pos]
		switch {
		case c == '\\' && s.pos+1 < len(src):
			s.word.WriteByte(src[s.pos+1])
			s.pos += 2
		case c == '$' && s.peek(1) == '(':
			s.substitution(s.pos+2, ')')
		case c == '`':
			s.substitution(s.pos+1, '`')
		default:
			s.word.WriteByte(c)
			s.pos++
		}
	}
	s.pos++
}

// substitution scans a `$(...)` or backtick body starting at start, recursing into it.
func (s *scanner) substitution(start int, closer byte) {
	src := s.source
	depth := 0
	i := start
	for i < len(src) {
		c := src[i]
		if c == '\\' {
			i += 2
			continue
		}
		if c == '\'' && closer == ')' {
			i = indexFrom(src, '\'', i+1) + 1
			if i > len(src) {
				i = len(src)
			}
			continue
		}
		if closer == ')' && c == '(' {
			depth++
		}
		if c == closer {
			if depth == 0 {
				break
			}
			depth--
		}
		i++
	}
	end := min(max(i, start), len(src))
	body := ""
	if start <= len(src) {
		body = src[start:end]
	}
	s.substitutions = append(s.substitutions, Commands(body)...)
	s.word.WriteString("<subst>")
	s.pos = i + 1
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func allDigits(text string) bool {
	if text == "" {
		return false
	}
	for i := 0; i < len(text); i++ {
		if !isDigit(text[i]) {
			return false
		}
	}
	return true
}

func (s *scanner) redirect() {
	src := s.source
	if s.inWord && allDigits(s.word.String()) {
		s.word.Reset()
		s.inWord = false
	} else {
		s.endWord()
	}

	if strings.HasPrefix(src[s.pos:], "<<<") {
		s.pos += 3
		s.readTargetWord()
		return
	}
	if strings.HasPrefix(src[s.pos:], "<<") {
		s.pos += 2
		stripTabs := false
		if s.peek(0) == '-' {
			stripTabs = true
			s.pos++
		}
		if delimiter := s.readTargetWord(); delimiter != "" {
			s.pending = append(s.pending, heredoc{delimiter, stripTabs})
		}
		return
	}

	s.pos++
	for s.pos < len(src) && strings.IndexByte("<>&|", src[s.pos]) >= 0 {
		s.pos++
	}
	if s.pos < len(src) && (isDigit(src[s.pos]) || src[s.pos] == '-') && (src[s.pos-1] == '&' || src[s.pos-1] == '>') {
		for s.pos < len(src) && (isDigit(src[s.pos]) || src[s.pos] == '-') {
			s.pos++
		}
		return
	}
	s.readTargetWord()
}

func (s *scanner) readTargetWord() string {
	src := s.source
	for s.pos < len(src) && (src[s.pos] == ' ' || src[s.pos] == '\t') {
		s.pos++
	}
	var b strings.Builder
	for s.pos < len(src) && strings.IndexByte(" \t\n;&|()<>", src[s.pos]) < 0 {
		c := src[s.pos]
		switch {
		case c == '\'' || c == '"':
			stop := indexFrom(src, c, s.pos+1)
			b.WriteString(src[s.pos+1 : stop])
			s.pos = stop + 1
		case c == '\\' && s.pos+1 < len(src):
			b.WriteByte(src[s.pos+1])
			s.pos += 2
		default:
			b.WriteByte(c)
			s.pos++
		}
	}
	return b.String()
}

// skipHeredocBodies moves past the pending heredoc bodies; with parse their lines are scanned as commands.
func (s *scanner) skipHeredocBodies(parse bool) {
	src := s.source
	for _, h := range s.pending {
		var body []string
		for s.pos < len(src) {
			stop := indexFrom(src, '\n', s.pos)
			line := src[s.pos:stop]
			if h.stripTabs {
				line = strings.TrimLeft(line, "\t")
			}
			s.pos = stop + 1
			if line == h.delimiter {
				break
			}
			body = append(body, line)
		}
		if parse {
			s.substitutions = append(s.substitutions, Commands(strings.Join(body, "\n"))...)
		}
	}
	s.pending = nil
}
