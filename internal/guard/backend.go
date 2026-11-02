package guard

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/guard/shell"
)

// Backend rules (P10), from the platform's `guard:` key. `block_commands` is checked in checkCommand on every
// unwrapped command; SQL and migrations are checked on the whole script by evaluateBackend, because the SQL may
// sit in a heredoc or a pipe and a connection string in a `VAR=` assignment, neither of which reaches argv.

// blockedCommand is the reason a command matches a `block_commands` entry: the program, then the entry's words in
// order among the words that are not options. Empty when none matches.
func blockedCommand(argv []string, p Policy) string {
	if len(p.BlockCommands) == 0 || len(argv) == 0 {
		return ""
	}
	program := baseName(argv[0])
	var words []string
	for _, word := range argv[1:] {
		if !strings.HasPrefix(word, "-") {
			words = append(words, word)
		}
	}
	for _, entry := range p.BlockCommands {
		if entry[0] != program || !inOrder(entry[1:], words) {
			continue
		}
		return strings.Join(entry, " ") + " is reserved for the user: it changes shared infrastructure. Ask them to run it."
	}
	return ""
}

func inOrder(want, words []string) bool {
	i := 0
	for _, word := range words {
		if i < len(want) && word == want[i] {
			i++
		}
	}
	return i == len(want)
}

// evaluateBackend blocks a script that runs a SQL client while its text holds a blocked statement, and a migration
// tool run whose connection string names a host that is not local. argv is the command sent as a list, if any.
func evaluateBackend(script string, argv []string, p Policy) Decision {
	if len(p.BlockSQL.Clients) == 0 && len(p.BlockMigrations.Tools) == 0 {
		return Allow()
	}
	var commands [][]string
	backendCommands(script, 0, &commands)
	text := script
	if argv != nil {
		commands = append(commands, argv)
		text += "\n" + strings.Join(argv, " ")
	}
	for _, raw := range commands {
		program := ""
		if inner := unwrap(raw).argv; len(inner) > 0 {
			program = baseName(inner[0])
		}
		if contains(p.BlockSQL.Clients, program) {
			if statement := sqlStatement(text, p.BlockSQL.Statements); statement != "" {
				return Block(statement + " through " + program + " is reserved for the user: it destroys data. Ask them to run it.")
			}
		}
		if contains(p.BlockMigrations.Tools, program) {
			if host := remoteHost(raw, p.BlockMigrations.LocalHosts); host != "" {
				return Block(program + " against " + host + " is reserved for the user: only local databases are migrated by agents. Ask them to run it.")
			}
		}
	}
	return Allow()
}

// backendCommands gathers every simple command of script, raw (assignments and wrappers kept), with the commands of
// the scripts and commands they run.
func backendCommands(script string, depth int, out *[][]string) {
	if depth > maxNesting {
		return
	}
	for _, argv := range shell.Expanded(script) {
		backendArgv(argv, depth, out)
	}
}

func backendArgv(argv []string, depth int, out *[][]string) {
	if depth > maxNesting {
		return
	}
	*out = append(*out, argv)
	inner := unwrap(argv)
	for _, script := range inner.scripts {
		backendCommands(script, depth+1, out)
	}
	for _, command := range inner.commands {
		backendArgv(command, depth+1, out)
	}
}

func sqlStatement(text string, statements []string) string {
	for _, statement := range statements {
		if regexp.MustCompile(`(?i)(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(statement) + `($|[^A-Za-z0-9_])`).MatchString(text) {
			return statement
		}
	}
	return ""
}

var (
	hostPair = regexp.MustCompile(`(?:^|\s)host=([^\s]+)`)
	tcpHost  = regexp.MustCompile(`@tcp\(([^)]+)\)`)
)

// remoteHost is the first host named by a connection string among argv's words, assignments included, that is not
// in local; empty when every host is local or none is visible.
func remoteHost(argv []string, local []string) string {
	for _, word := range argv {
		candidates := []string{word}
		if name, value, ok := strings.Cut(word, "="); ok && assignment.MatchString(name+"=") {
			candidates = append(candidates, value)
		}
		for _, candidate := range candidates {
			for _, host := range hostsIn(candidate) {
				if !isLocal(host, local) {
					return host
				}
			}
		}
	}
	return ""
}

func hostsIn(word string) []string {
	var hosts []string
	if strings.Contains(word, "://") {
		if parsed, err := url.Parse(word); err == nil && parsed.Hostname() != "" {
			hosts = append(hosts, parsed.Hostname())
		} else if err == nil {
			if host := parsed.Query().Get("host"); host != "" {
				hosts = append(hosts, host)
			}
		}
	}
	for _, match := range hostPair.FindAllStringSubmatch(word, -1) {
		hosts = append(hosts, strings.Trim(match[1], `'"`))
	}
	for _, match := range tcpHost.FindAllStringSubmatch(word, -1) {
		host := match[1]
		if i := strings.LastIndexByte(host, ':'); i > 0 && !strings.HasSuffix(host, "]") {
			host = host[:i]
		}
		hosts = append(hosts, strings.Trim(host, "[]"))
	}
	return hosts
}

// isLocal is true for a host in local, and for a Unix socket folder.
func isLocal(host string, local []string) bool {
	if strings.HasPrefix(host, "/") {
		return true
	}
	for _, name := range local {
		if strings.EqualFold(host, name) {
			return true
		}
	}
	return false
}

func contains(list []string, item string) bool {
	for _, entry := range list {
		if entry == item {
			return true
		}
	}
	return false
}
