package main

// A port of the parts of Dart's `package:args` 2.7.0 (ArgParser, CommandRunner, HelpCommand) that bin/dashcrew.dart
// uses, so parsing, help text, usage errors and exit codes match the Dart engine byte for byte. Usage is never
// wrapped, as with `usageLineLength: null`.

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf16"
)

const executableName = "dashcrew"

// option is one flag or option of a command.
type option struct {
	name, abbr, help string
	isFlag           bool
	negatable        bool
	multiple         bool
	defaultFlag      bool
	defaultValue     *string
	defaultList      []string
	allowed          []string
}

// command is the runner (root), a branch command with subcommands, or a leaf command with run.
type command struct {
	name        string
	description string
	// invocation overrides the generated `dashcrew <path> [arguments]`.
	invocation string
	hidden     bool
	options    []*option
	commands   []*command
	parent     *command
	run        func(r *results) (int, error)
}

func newCommand(name, description string) *command {
	c := &command{name: name, description: description}
	c.flag("help", "h", "Print this usage information.", false, false)
	return c
}

func (c *command) flag(name, abbr, help string, defaultsTo, negatable bool) *command {
	c.options = append(c.options, &option{name: name, abbr: abbr, help: help, isFlag: true, negatable: negatable,
		defaultFlag: defaultsTo})
	return c
}

func (c *command) option(name, abbr, help string, defaultsTo *string) *command {
	c.options = append(c.options, &option{name: name, abbr: abbr, help: help, defaultValue: defaultsTo})
	return c
}

func (c *command) multiOption(name, help string, allowed, defaultsTo []string) *command {
	c.options = append(c.options, &option{name: name, help: help, multiple: true, allowed: allowed, defaultList: defaultsTo})
	return c
}

func (c *command) add(sub *command) *command {
	sub.parent = c
	c.commands = append(c.commands, sub)
	return c
}

func (c *command) sub(name string) *command {
	for _, s := range c.commands {
		if s.name == name {
			return s
		}
	}
	return nil
}

func (c *command) byAbbr(abbr string) *option {
	for _, o := range c.options {
		if o.abbr != "" && o.abbr == abbr {
			return o
		}
	}
	return nil
}

func (c *command) byName(name string) *option {
	for _, o := range c.options {
		if o.name == name {
			return o
		}
	}
	return nil
}

func (c *command) isRoot() bool { return c.parent == nil }

func (c *command) summary() string { return strings.Split(c.description, "\n")[0] }

func (c *command) invocationText() string {
	if c.isRoot() {
		return executableName + " <command> [arguments]"
	}
	if c.invocation != "" {
		return c.invocation
	}
	names := []string{}
	for cmd := c; cmd != nil; cmd = cmd.parent {
		if cmd.isRoot() {
			names = append([]string{executableName}, names...)
		} else {
			names = append([]string{cmd.name}, names...)
		}
	}
	if len(c.commands) > 0 {
		return strings.Join(names, " ") + " <subcommand> [arguments]"
	}
	return strings.Join(names, " ") + " [arguments]"
}

// usage is the text `--help` prints, without the final newline.
func (c *command) usage() string { return c.description + "\n\n" + c.usageWithoutDescription() }

// usageWithoutDescription follows the message of a usage error.
func (c *command) usageWithoutDescription() string {
	if c.isRoot() {
		return "Usage: " + c.invocationText() + "\n\nGlobal options:\n" + optionsUsage(c.options) + "\n\n" +
			commandsUsage(c.commands, false) + "\n\n" +
			`Run "` + executableName + ` help <command>" for more information about a command.`
	}
	var b strings.Builder
	b.WriteString("Usage: " + c.invocationText() + "\n" + optionsUsage(c.options) + "\n")
	if len(c.commands) > 0 {
		b.WriteString("\n" + commandsUsage(c.commands, true) + "\n")
	}
	b.WriteString("\n" + `Run "` + executableName + ` help" to see global options.`)
	return b.String()
}

func padRight(text string, width int) string {
	if n := len(text); n < width {
		return text + strings.Repeat(" ", width-n)
	}
	return text
}

func (o *option) abbrText() string {
	if o.abbr == "" {
		return ""
	}
	return "-" + o.abbr + ", "
}

func (o *option) longText() string {
	if o.isFlag && o.negatable {
		return "--[no-]" + o.name
	}
	return "--" + o.name
}

// extraText is the second help line: the allowed values or the default.
func (o *option) extraText() string {
	switch {
	case o.allowed != nil:
		parts := []string{}
		for _, value := range o.allowed {
			isDefault := false
			if o.multiple {
				for _, d := range o.defaultList {
					isDefault = isDefault || d == value
				}
			} else {
				isDefault = o.defaultValue != nil && *o.defaultValue == value
			}
			if isDefault {
				value += " (default)"
			}
			parts = append(parts, value)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case o.isFlag:
		if o.defaultFlag {
			return "(defaults to on)"
		}
	case o.multiple:
		if len(o.defaultList) > 0 {
			quoted := []string{}
			for _, value := range o.defaultList {
				quoted = append(quoted, `"`+value+`"`)
			}
			return "(defaults to " + strings.Join(quoted, ", ") + ")"
		}
	case o.defaultValue != nil:
		return `(defaults to "` + *o.defaultValue + `")`
	}
	return ""
}

// optionsUsage is args' three-column option table; every option here has a one-line help text.
func optionsUsage(options []*option) string {
	abbrWidth, titleWidth := 0, 0
	for _, o := range options {
		abbrWidth = max(abbrWidth, len(o.abbrText()))
		titleWidth = max(titleWidth, len(o.longText()))
	}
	titleWidth += 4
	rows := []string{}
	for _, o := range options {
		row := padRight(o.abbrText(), abbrWidth) + padRight(o.longText(), titleWidth) + o.help
		if extra := o.extraText(); extra != "" {
			row += "\n" + strings.Repeat(" ", abbrWidth+titleWidth) + extra
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

func commandsUsage(commands []*command, isSubcommand bool) string {
	visible := []*command{}
	for _, c := range commands {
		if !c.hidden {
			visible = append(visible, c)
		}
	}
	if len(visible) == 0 {
		visible = commands
	}
	sort.SliceStable(visible, func(i, j int) bool { return visible[i].name < visible[j].name })
	length := 0
	for _, c := range visible {
		length = max(length, len(c.name))
	}
	header := "Available commands:"
	if isSubcommand {
		header = "Available subcommands:"
	}
	var b strings.Builder
	b.WriteString(header)
	for _, c := range visible {
		b.WriteString("\n  " + padRight(c.name, length) + "   " + c.summary())
	}
	return b.String()
}

// results are the parsed arguments of one command level.
type results struct {
	name    string
	values  map[string]any
	grammar *command
	command *results
	rest    []string
}

func (r *results) flag(name string) bool {
	if value, ok := r.values[name].(bool); ok {
		return value
	}
	return r.grammar.byName(name).defaultFlag
}

func (r *results) option(name string) *string {
	if value, ok := r.values[name].(string); ok {
		return &value
	}
	return r.grammar.byName(name).defaultValue
}

func (r *results) multiOption(name string) []string {
	if value, ok := r.values[name].([]string); ok {
		return value
	}
	return r.grammar.byName(name).defaultList
}

// argError is an ArgParserException: commands is the path to the command whose usage follows the message.
type argError struct {
	message  string
	commands []string
}

func (e *argError) Error() string { return e.message }

type parser struct {
	grammar *command
	args    *[]string
	parent  *parser
	rest    []string
	values  map[string]any
}

func (p *parser) current() string { return (*p.args)[0] }

func (p *parser) pop() string {
	value := (*p.args)[0]
	*p.args = (*p.args)[1:]
	return value
}

func invalid(message string) *argError { return &argError{message: message} }

func (p *parser) parse(name string) (*results, *argError) {
	var commandResults *results
	for len(*p.args) > 0 {
		if p.current() == "--" {
			p.pop()
			break
		}
		if sub := p.grammar.sub(p.current()); sub != nil {
			if len(p.rest) > 0 {
				return nil, invalid("Cannot specify arguments before a command.")
			}
			p.pop()
			child := &parser{grammar: sub, args: p.args, parent: p, values: map[string]any{}}
			parsed, err := child.parse(sub.name)
			if err != nil {
				err.commands = append([]string{sub.name}, err.commands...)
				return nil, err
			}
			commandResults = parsed
			p.rest = nil
			break
		}
		if ok, err := p.parseSoloOption(); err != nil || ok {
			if err != nil {
				return nil, err
			}
			continue
		}
		if ok, err := p.parseAbbreviation(p); err != nil || ok {
			if err != nil {
				return nil, err
			}
			continue
		}
		if ok, err := p.parseLongOption(); err != nil || ok {
			if err != nil {
				return nil, err
			}
			continue
		}
		p.rest = append(p.rest, p.pop())
	}
	p.rest = append(p.rest, *p.args...)
	*p.args = nil
	return &results{name: name, values: p.values, grammar: p.grammar, command: commandResults, rest: p.rest}, nil
}

func isLetterOrDigit(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

func (p *parser) readNextArgAsValue(o *option, arg string) *argError {
	if len(*p.args) == 0 {
		return invalid(`Missing argument for "` + arg + `".`)
	}
	if err := p.setOption(o, p.current(), arg); err != nil {
		return err
	}
	p.pop()
	return nil
}

func (p *parser) parseSoloOption() (bool, *argError) {
	current := p.current()
	if len(current) != 2 || current[0] != '-' || !isLetterOrDigit(current[1]) {
		return false, nil
	}
	return p.handleSoloOption(current[1:])
}

func (p *parser) handleSoloOption(opt string) (bool, *argError) {
	o := p.grammar.byAbbr(opt)
	if o == nil {
		if p.parent == nil {
			return false, invalid(`Could not find an option or flag "-` + opt + `".`)
		}
		return p.parent.handleSoloOption(opt)
	}
	p.pop()
	if o.isFlag {
		p.values[o.name] = true
		return true, nil
	}
	return true, p.readNextArgAsValue(o, "-"+opt)
}

func (p *parser) parseAbbreviation(innermost *parser) (bool, *argError) {
	current := p.current()
	if len(current) < 2 || current[0] != '-' {
		return false, nil
	}
	index := 1
	for index < len(current) && isLetterOrDigit(current[index]) {
		index++
	}
	if index == 1 {
		return false, nil
	}
	letters, rest := current[1:index], current[index:]
	if strings.ContainsAny(rest, "\n\r") {
		return false, nil
	}
	return p.handleAbbreviation(letters, rest, innermost)
}

func (p *parser) handleAbbreviation(letters, rest string, innermost *parser) (bool, *argError) {
	c := letters[:1]
	first := p.grammar.byAbbr(c)
	switch {
	case first == nil:
		if p.parent == nil {
			return false, invalid(`Could not find an option with short name "-` + c + `".`)
		}
		return p.parent.handleAbbreviation(letters, rest, innermost)
	case !first.isFlag:
		if err := p.setOption(first, letters[1:]+rest, "-"+c); err != nil {
			return false, err
		}
	default:
		if rest != "" {
			return false, invalid(`Option "-` + c + `" is a flag and cannot handle value "` + letters[1:] + rest + `".`)
		}
		for i := 0; i < len(letters); i++ {
			if err := innermost.parseShortFlag(letters[i : i+1]); err != nil {
				return false, err
			}
		}
	}
	p.pop()
	return true, nil
}

func (p *parser) parseShortFlag(c string) *argError {
	o := p.grammar.byAbbr(c)
	if o == nil {
		if p.parent == nil {
			return invalid(`Could not find an option with short name "-` + c + `".`)
		}
		return p.parent.parseShortFlag(c)
	}
	if !o.isFlag {
		return invalid(`Option "-` + c + `" must be a flag to be in a collapsed "-".`)
	}
	p.values[o.name] = true
	return nil
}

func (p *parser) parseLongOption() (bool, *argError) {
	current := p.current()
	if !strings.HasPrefix(current, "--") {
		return false, nil
	}
	name, value, hasValue := strings.Cut(current[2:], "=")
	for i := 0; i < len(name); i++ {
		if !isLetterOrDigit(name[i]) && name[i] != '-' && name[i] != '_' {
			return false, nil
		}
	}
	if hasValue && strings.ContainsAny(value, "\n\r") {
		return false, nil
	}
	var v *string
	if hasValue {
		v = &value
	}
	return p.handleLongOption(name, v)
}

func (p *parser) handleLongOption(name string, value *string) (bool, *argError) {
	if o := p.grammar.byName(name); o != nil {
		p.pop()
		switch {
		case o.isFlag:
			if value != nil {
				return false, invalid(`Flag option "--` + name + `" should not be given a value.`)
			}
			p.values[o.name] = true
		case value != nil:
			if err := p.setOption(o, *value, "--"+name); err != nil {
				return false, err
			}
		default:
			if err := p.readNextArgAsValue(o, "--"+name); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	if positive, ok := strings.CutPrefix(name, "no-"); ok {
		o := p.grammar.byName(positive)
		if o == nil {
			if p.parent == nil {
				return false, invalid(`Could not find an option named "--` + name + `".`)
			}
			return p.parent.handleLongOption(name, value)
		}
		p.pop()
		if !o.isFlag {
			return false, invalid(`Cannot negate non-flag option "--` + name + `".`)
		}
		if !o.negatable {
			return false, invalid(`Cannot negate option "--` + name + `".`)
		}
		p.values[o.name] = false
		return true, nil
	}
	if p.parent == nil {
		return false, invalid(`Could not find an option named "--` + name + `".`)
	}
	return p.parent.handleLongOption(name, value)
}

func (p *parser) setOption(o *option, value, arg string) *argError {
	allowed := func(v string) *argError {
		if o.allowed == nil {
			return nil
		}
		for _, a := range o.allowed {
			if a == v {
				return nil
			}
		}
		return invalid(`"` + v + `" is not an allowed value for option "` + arg + `".`)
	}
	if !o.multiple {
		if err := allowed(value); err != nil {
			return err
		}
		p.values[o.name] = value
		return nil
	}
	list, _ := p.values[o.name].([]string)
	for _, element := range strings.Split(value, ",") {
		if err := allowed(element); err != nil {
			return err
		}
		list = append(list, element)
	}
	p.values[o.name] = list
	return nil
}

// usageError is a UsageException: the message and the usage of command go to stderr, and the exit code is 64.
type usageError struct {
	command *command
	message string
}

func (e *usageError) Error() string { return e.message }

// editDistance is the Damerau-Levenshtein distance args uses for suggestions, over UTF-16 code units.
func editDistance(from, to string) int {
	a := append([]uint16{' '}, utf16.Encode([]rune(from))...)
	b := append([]uint16{' '}, utf16.Encode([]rune(to))...)
	d := make([][]int, len(a))
	for i := range d {
		d[i] = make([]int, len(b))
		for j := range d[i] {
			switch {
			case i == 0:
				d[i][j] = j
			case j == 0:
				d[i][j] = i
			}
		}
	}
	for i := 1; i < len(a); i++ {
		for j := 1; j < len(b); j++ {
			cost := 1
			if a[i] == b[j] {
				cost = 0
			}
			best := min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && a[i] == b[j-1] && a[i-1] == b[j] {
				best = min(best, d[i-2][j-2]+1)
			}
			d[i][j] = best
		}
	}
	return d[len(a)-1][len(b)-1]
}

// similarCommands keeps the first command per distance, as args' SplayTreeSet ordered by distance does.
func similarCommands(requested string, commands []*command) string {
	byDistance := map[int]*command{}
	for _, c := range commands {
		if c.hidden {
			continue
		}
		if distance := editDistance(requested, c.name); distance <= 2 {
			if _, ok := byDistance[distance]; !ok {
				byDistance[distance] = c
			}
		}
	}
	if len(byDistance) == 0 {
		return ""
	}
	distances := []int{}
	for distance := range byDistance {
		distances = append(distances, distance)
	}
	sort.Ints(distances)
	var b strings.Builder
	b.WriteString("\n\nDid you mean one of these?\n")
	for _, distance := range distances {
		b.WriteString("  " + byDistance[distance].name + "\n")
	}
	return b.String()
}

// newHelpCommand is args' hidden `help [command]`.
func newHelpCommand(root *command, stdout io.Writer) *command {
	help := newCommand("help", "Display help information for "+executableName+".")
	help.invocation = executableName + " help [command]"
	help.hidden = true
	help.run = func(r *results) (int, error) {
		if len(r.rest) == 0 {
			fmt.Fprintln(stdout, root.usage())
			return 0, nil
		}
		commands := root.commands
		var cmd *command
		commandString := executableName
		for _, name := range r.rest {
			if len(commands) == 0 {
				return 0, &usageError{cmd, `Command "` + commandString + `" does not expect a subcommand.`}
			}
			next := (&command{commands: commands}).sub(name)
			if next == nil {
				if cmd == nil {
					return 0, &usageError{root, `Could not find a command named "` + name + `".`}
				}
				return 0, &usageError{cmd, `Could not find a subcommand named "` + name + `" for "` + commandString + `".`}
			}
			cmd = next
			commands = cmd.commands
			commandString += " " + name
		}
		fmt.Fprintln(stdout, cmd.usage())
		return 0, nil
	}
	return help
}

// runCommand is CommandRunner.run: parse, walk to the chosen command, print help or run it.
func runCommand(root *command, args []string, stdout, stderr io.Writer) (int, error) {
	queue := append([]string{}, args...)
	top, parseErr := (&parser{grammar: root, args: &queue, values: map[string]any{}}).parse("")
	if parseErr != nil {
		cmd := root
		for _, name := range parseErr.commands {
			cmd = cmd.sub(name)
		}
		return 0, &usageError{cmd, parseErr.message}
	}
	current := top
	var cmd *command
	commands := root.commands
	commandString := executableName
	for len(commands) > 0 {
		if current.command == nil {
			if len(current.rest) == 0 {
				if cmd == nil {
					fmt.Fprintln(stdout, root.usage())
					return 0, nil
				}
				return 0, &usageError{cmd, `Missing subcommand for "` + commandString + `".`}
			}
			requested := current.rest[0]
			similar := similarCommands(requested, commands)
			if cmd == nil {
				return 0, &usageError{root, `Could not find a command named "` + requested + `".` + similar}
			}
			return 0, &usageError{cmd, `Could not find a subcommand named "` + requested + `" for "` + commandString + `".` + similar}
		}
		current = current.command
		cmd = (&command{commands: commands}).sub(current.name)
		commands = cmd.commands
		commandString += " " + current.name
		if current.flag("help") {
			fmt.Fprintln(stdout, cmd.usage())
			return 0, nil
		}
	}
	if top.flag("help") {
		fmt.Fprintln(stdout, cmd.usage())
		return 0, nil
	}
	return cmd.run(current)
}
