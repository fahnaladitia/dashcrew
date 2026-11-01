package guard

import (
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/guard/shell"
)

// The rules below are Go only (G4b). They run after the Dart rules and can only add blocks: a command is first
// checked exactly as the Dart guard checks it, then again after unwrapping shells, keywords and wrappers.

// maxNesting is how many shells inside shells the guard follows; a deeper script is allowed, like any input the
// guard cannot read.
const maxNesting = 8

// keywords start a command without being one: `{ git push; }`, `if git push; then`, `! git push`, `time git push`.
var keywords = map[string]bool{"{": true, "!": true, "if": true, "elif": true, "then": true, "else": true, "do": true, "while": true, "until": true}

// shells run a script given with `-c`, alone or in an option cluster such as `-lc` or `-ec`.
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true}

// options describes a wrapper's options: short letters and long names that take a value in the next word when
// none is attached (`-u root`, `-uroot`, `--user root`, `--user=root`).
type options struct {
	short string
	long  map[string]bool
}

// parse splits args into the wrapper's leading options and the words after them; `--` ends the options. seen
// maps every option given to its value, empty for a flag.
func (o options) parse(args []string) (rest []string, seen map[string]string) {
	seen = map[string]string{}
	for len(args) > 0 {
		word := args[0]
		if word == "--" {
			return args[1:], seen
		}
		if len(word) < 2 || word[0] != '-' {
			return args, seen
		}
		args = args[1:]
		if strings.HasPrefix(word, "--") {
			name, value, attached := strings.Cut(word[2:], "=")
			if !attached && o.long[name] && len(args) > 0 {
				value, args = args[0], args[1:]
			}
			seen[name] = value
			continue
		}
		for i := 1; i < len(word); i++ {
			letter := word[i : i+1]
			if !strings.Contains(o.short, letter) {
				seen[letter] = ""
				continue
			}
			value := word[i+1:]
			if value == "" && len(args) > 0 {
				value, args = args[0], args[1:]
			}
			seen[letter] = value
			break
		}
	}
	return args, seen
}

func longs(names ...string) map[string]bool {
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	return set
}

var wrapperOptions = map[string]options{
	"sudo":    {"ughpCDRTUrta", longs("user", "group", "host", "prompt", "close-from", "chdir", "chroot", "command-timeout", "other-user", "role", "type", "auth-type", "login-class")},
	"env":     {"uCPSa", longs("unset", "chdir", "split-string", "argv0")},
	"nice":    {"n", longs("adjustment")},
	"ionice":  {"cnpPu", longs("class", "classdata", "pid", "pgid", "uid")},
	"timeout": {"sk", longs("signal", "kill-after")},
	"exec":    {"a", nil},
	"command": {"", nil},
	"builtin": {"", nil},
	"nohup":   {"", nil},
	"time":    {"of", longs("output", "format")},
	"xargs":   {"adEILnPs", longs("arg-file", "delimiter", "max-args", "max-procs", "max-chars", "process-slot-var")},
	"rtk":     {"", nil},
	"fvm":     {"", nil},
}

// unwrapped is what one simple command runs: argv after keywords, assignments and wrappers, the scripts it hands
// to a shell, and the commands `find -exec` runs.
type unwrapped struct {
	argv     []string
	scripts  []string
	commands [][]string
}

func baseName(word string) string {
	return word[strings.LastIndexByte(word, '/')+1:]
}

// unwrap peels argv until a program that is neither a keyword nor a wrapper is first.
func unwrap(argv []string) unwrapped {
	for len(argv) > 0 {
		if keywords[argv[0]] || assignment.MatchString(argv[0]) {
			argv = argv[1:]
			continue
		}
		name := baseName(argv[0])
		args := argv[1:]
		switch {
		case shells[name]:
			if script, ok := shellScript(args); ok {
				return unwrapped{scripts: []string{script}}
			}
			return unwrapped{argv: argv}
		case name == "eval":
			return unwrapped{scripts: []string{strings.Join(args, " ")}}
		case name == "find":
			return unwrapped{argv: argv, commands: findExec(args)}
		case name == "rtk" && len(args) > 0 && args[0] == "proxy":
			argv = args[1:]
			continue
		}
		spec, ok := wrapperOptions[name]
		if !ok {
			return unwrapped{argv: argv}
		}
		rest, seen := spec.parse(args)
		switch name {
		case "env":
			if split, ok := firstOf(seen, "S", "split-string"); ok {
				return unwrapped{scripts: []string{strings.TrimSpace(split + " " + strings.Join(rest, " "))}}
			}
			if len(rest) > 0 && rest[0] == "-" {
				rest = rest[1:]
			}
		case "command":
			if _, ok := firstOf(seen, "v", "V"); ok {
				return unwrapped{}
			}
		case "timeout":
			if len(rest) > 0 {
				rest = rest[1:]
			}
		}
		argv = rest
	}
	return unwrapped{}
}

func firstOf(seen map[string]string, names ...string) (string, bool) {
	for _, name := range names {
		if value, ok := seen[name]; ok {
			return value, true
		}
	}
	return "", false
}

// shellScript is the script of `sh -c <script>`: the first word after the options when a `-` option cluster
// holds `c`. `-o` and `-O` take the next word, as do `--rcfile` and `--init-file`.
func shellScript(args []string) (string, bool) {
	withC := false
	for i := 0; i < len(args); i++ {
		word := args[i]
		if word == "--" || word == "-" {
			if withC && i+1 < len(args) {
				return args[i+1], true
			}
			return "", false
		}
		if len(word) > 1 && (word[0] == '-' || word[0] == '+') {
			if strings.HasPrefix(word, "--") {
				if word == "--rcfile" || word == "--init-file" {
					i++
				}
				continue
			}
			if word[0] == '-' && strings.Contains(word[1:], "c") {
				withC = true
			}
			if strings.ContainsAny(word[1:], "oO") {
				i++
			}
			continue
		}
		if withC {
			return word, true
		}
		return "", false
	}
	return "", false
}

// findExec is every command a `find` expression runs with `-exec`, `-execdir`, `-ok` or `-okdir`, each ending
// at `;` or `+`.
func findExec(args []string) [][]string {
	var commands [][]string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-exec", "-execdir", "-ok", "-okdir":
			start := i + 1
			for i = start; i < len(args) && args[i] != ";" && args[i] != "+"; i++ {
			}
			if i > start {
				commands = append(commands, args[start:i])
			}
		}
	}
	return commands
}

// evaluateNested checks script's commands after unwrapping, following shells it runs up to maxNesting deep.
func evaluateNested(script string, p Policy, depth int) Decision {
	if depth > maxNesting {
		return Allow()
	}
	for _, argv := range shell.Expanded(script) {
		if decision := evaluateUnwrapped(argv, p, depth); decision.Blocked {
			return decision
		}
	}
	return Allow()
}

// evaluateUnwrapped checks one simple command after unwrap, then the scripts and commands it runs.
func evaluateUnwrapped(argv []string, p Policy, depth int) Decision {
	if depth > maxNesting {
		return Allow()
	}
	inner := unwrap(argv)
	if decision := checkCommand(inner.argv, p); decision.Blocked {
		return decision
	}
	for _, script := range inner.scripts {
		if decision := evaluateNested(script, p, depth+1); decision.Blocked {
			return decision
		}
	}
	for _, command := range inner.commands {
		if decision := evaluateUnwrapped(command, p, depth+1); decision.Blocked {
			return decision
		}
	}
	return Allow()
}
