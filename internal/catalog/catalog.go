// Package catalog loads the neutral sources: `core/` fragments and roles, the shared `packs/`, `platforms/`,
// and a project's `.ai/packs/` and `.ai/platforms/`. Source paths are slash-separated and relative to the
// catalog root or the project root, as in the Dart engine.
package catalog

import (
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Error is a problem in a catalog file, with that file's path.
type Error struct {
	Source  string
	Message string
}

func (e *Error) Error() string { return e.Source + ": " + e.Message }

func fail(source, message string) *Error { return &Error{Source: source, Message: message} }

// Fragment is one Markdown file with front matter, rendered into its target.
type Fragment struct {
	Source string
	Target string
	Order  int
	Rules  []string
	When   *omap.Map[any]
	Body   string
}

// DocMeta describes a skill or workflow, from its `meta.yaml` (or a pack's `skills/<name>.yaml`).
type DocMeta struct {
	Description  string
	WhenToUse    *string
	ArgumentHint *string
	// Review marks a skill whose checklist the reviewer applies when the skill is active.
	Review bool
}

// RoleMeta is `core/roles/<name>/role.yaml`.
type RoleMeta struct {
	Name        string
	Description string
	Capability  string
	Model       *string
	Effort      *string
	// When uses fragment matching; the role is generated only when it matches.
	When *omap.Map[any]
}

// Capabilities a role may declare, in the order error messages list them.
var Capabilities = []string{"read", "write-plans", "write-code"}

// McpServer is an MCP server a platform's agents use; each adapter writes it in its own format.
type McpServer struct {
	Command string
	Args    []string
}

// Pack is one option of a stack family, e.g. `state/bloc`.
type Pack struct {
	Family    string
	Option    string
	Params    *omap.Map[any]
	Fragments []Fragment
	// Skills this pack adds, from `skills/<name>.yaml`.
	Skills map[string]DocMeta
	// FromProject marks a pack from `.ai/packs/`; it replaces a built-in pack of the same family and option.
	FromProject bool
}

// Packs maps family to option to pack, in load order.
type Packs = *omap.Map[*omap.Map[*Pack]]

// PlatformSpec is `platforms/<name>/`, `.ai/platforms/<name>/`, or `other` from the profile.
type PlatformSpec struct {
	Name string
	// Root is the source folder (`platforms/flutter`); empty for `other`.
	Root        string
	Description *string
	Detect      []string
	// Commands holds analyze, test, test_scoped, deps and format (nil when not declared).
	Commands   *omap.Map[any]
	Link       *string
	DocComment *string
	// McpServers in declaration order.
	McpServers *omap.Map[McpServer]
	// Fragments outside `packs/`; one at the same path as a core fragment replaces it.
	Fragments   []Fragment
	Skills      map[string]DocMeta
	Workflows   map[string]DocMeta
	Packs       Packs
	FromProject bool
}

var platformKeys = []string{"description", "detect", "commands", "link", "doc_comment", "mcp_servers"}

func stringOrNil(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

// ToContext is `platform.*` in templates.
func (p *PlatformSpec) ToContext() map[string]any {
	detect := make([]any, len(p.Detect))
	for i, file := range p.Detect {
		detect[i] = file
	}
	commands := p.Commands
	if commands == nil {
		commands = omap.New[any]()
	}
	return map[string]any{
		"name":        p.Name,
		"description": stringOrNil(p.Description),
		"detect":      detect,
		"commands":    commands,
		"link":        stringOrNil(p.Link),
		"doc_comment": stringOrNil(p.DocComment),
	}
}

// ReplacedCoreSources are `core/<path>` for each platform fragment at `<root>/<path>`.
func (p *PlatformSpec) ReplacedCoreSources() map[string]bool {
	out := map[string]bool{}
	for _, fragment := range p.Fragments {
		out["core/"+strings.TrimPrefix(fragment.Source, p.Root+"/")] = true
	}
	return out
}

// Catalog holds the neutral sources, plus a project's packs and platforms after WithProjectPacks.
type Catalog struct {
	Core      []Fragment
	Packs     Packs
	Roles     map[string]RoleMeta
	Skills    map[string]DocMeta
	Workflows map[string]DocMeta
	Platforms map[string]*PlatformSpec
}

// LoadEmbedded loads the catalog compiled into the binary.
func LoadEmbedded() (*Catalog, error) {
	return Load(dashcrew.CatalogFS)
}

// Load reads `core/`, `packs/` and `platforms/` from the root of fsys.
func Load(fsys fs.FS) (*Catalog, error) {
	c := &Catalog{Roles: map[string]RoleMeta{}}
	var err error
	if isDir(fsys, "core") {
		if c.Core, err = fragmentsIn(fsys, "core"); err != nil {
			return nil, err
		}
	}
	if isDir(fsys, "core/roles") {
		for _, dir := range sortedDirs(fsys, "core/roles") {
			manifest := dir + "/role.yaml"
			if exists(fsys, manifest) {
				role, err := loadRole(fsys, manifest)
				if err != nil {
					return nil, err
				}
				c.Roles[path.Base(dir)] = role
			}
		}
	}
	if c.Skills, err = metasIn(fsys, "core/skills"); err != nil {
		return nil, err
	}
	if c.Workflows, err = metasIn(fsys, "core/workflows"); err != nil {
		return nil, err
	}
	if c.Packs, err = packsIn(fsys, "packs", false); err != nil {
		return nil, err
	}
	if c.Platforms, err = platformsIn(fsys, "platforms", false); err != nil {
		return nil, err
	}
	return c, nil
}

func copyPacks(packs Packs) Packs {
	out := omap.New[*omap.Map[*Pack]]()
	for _, family := range packs.Keys() {
		out.Set(family, packs.Value(family).Copy())
	}
	return out
}

// WithProjectPacks adds `<projectDir>/.ai/packs/`, which replace built-in packs of the same name, and
// `<projectDir>/.ai/platforms/`, which replace a built-in platform of the same name.
func (c *Catalog) WithProjectPacks(projectDir string) (*Catalog, error) {
	project := os.DirFS(projectDir)
	packs := copyPacks(c.Packs)
	projectPacks, err := packsIn(project, ".ai/packs", true)
	if err != nil {
		return nil, err
	}
	for _, family := range projectPacks.Keys() {
		target, ok := packs.Get(family)
		if !ok {
			target = omap.New[*Pack]()
			packs.Set(family, target)
		}
		options := projectPacks.Value(family)
		for _, option := range options.Keys() {
			target.Set(option, options.Value(option))
		}
	}
	projectPlatforms, err := platformsIn(project, ".ai/platforms", true)
	if err != nil {
		return nil, err
	}
	platforms := map[string]*PlatformSpec{}
	for name, platform := range c.Platforms {
		platforms[name] = platform
	}
	for name, platform := range projectPlatforms {
		platforms[name] = platform
	}
	return &Catalog{Core: c.Core, Packs: packs, Roles: c.Roles, Skills: c.Skills, Workflows: c.Workflows, Platforms: platforms}, nil
}

// PacksFor is the packs a profile on platform may select: the platform's families over the shared ones;
// a project pack wins over both.
func (c *Catalog) PacksFor(platform string) Packs {
	merged := copyPacks(c.Packs)
	spec := c.Platforms[platform]
	if spec == nil {
		return merged
	}
	for _, family := range spec.Packs.Keys() {
		target, ok := merged.Get(family)
		if !ok {
			target = omap.New[*Pack]()
			merged.Set(family, target)
		}
		options := spec.Packs.Value(family)
		for _, option := range options.Keys() {
			if existing, ok := target.Get(option); !ok || !existing.FromProject {
				target.Set(option, options.Value(option))
			}
		}
	}
	return merged
}

// ResolvedFor is this catalog with Packs resolved by PacksFor, for code that reads Packs directly.
func (c *Catalog) ResolvedFor(platform string) *Catalog {
	return &Catalog{Core: c.Core, Packs: c.PacksFor(platform), Roles: c.Roles, Skills: c.Skills, Workflows: c.Workflows, Platforms: c.Platforms}
}

// FragmentsFor is core minus the fragments the platform replaces, then the platform's own.
func (c *Catalog) FragmentsFor(platform string) []Fragment {
	spec := c.Platforms[platform]
	if spec == nil {
		return c.Core
	}
	replaced := spec.ReplacedCoreSources()
	out := []Fragment{}
	for _, fragment := range c.Core {
		if !replaced[fragment.Source] {
			out = append(out, fragment)
		}
	}
	return append(out, spec.Fragments...)
}

// SkillsFor is the core skill descriptions with the platform's over them.
func (c *Catalog) SkillsFor(platform string) map[string]DocMeta {
	out := map[string]DocMeta{}
	for name, meta := range c.Skills {
		out[name] = meta
	}
	if spec := c.Platforms[platform]; spec != nil {
		for name, meta := range spec.Skills {
			out[name] = meta
		}
	}
	return out
}

// WorkflowsFor is the core workflow descriptions with the platform's over them.
func (c *Catalog) WorkflowsFor(platform string) map[string]DocMeta {
	out := map[string]DocMeta{}
	for name, meta := range c.Workflows {
		out[name] = meta
	}
	if spec := c.Platforms[platform]; spec != nil {
		for name, meta := range spec.Workflows {
			out[name] = meta
		}
	}
	return out
}

// PlatformFor is the profile's platform: `other` comes from platform_config, an unknown name is an empty platform.
func (c *Catalog) PlatformFor(p *profile.Profile) *PlatformSpec {
	if p.Platform == profile.OtherOption && p.PlatformConfig != nil {
		return &PlatformSpec{Name: profile.OtherOption, Detect: p.PlatformConfig.Detect, Commands: p.PlatformConfig.Commands,
			McpServers: omap.New[McpServer](), Packs: omap.New[*omap.Map[*Pack]]()}
	}
	if spec := c.Platforms[p.Platform]; spec != nil {
		return spec
	}
	return &PlatformSpec{Name: p.Platform, Detect: []string{}, Commands: omap.New[any](), McpServers: omap.New[McpServer](),
		Packs: omap.New[*omap.Map[*Pack]]()}
}

// KnownOptions maps flutter, every platform and other to the families and options a profile there may select.
func (c *Catalog) KnownOptions() profile.KnownOptions {
	known := profile.KnownOptions{}
	names := []string{profile.DefaultPlatform}
	for name := range c.Platforms {
		names = append(names, name)
	}
	names = append(names, profile.OtherOption)
	for _, platform := range names {
		families := omap.New[[]string]()
		packs := c.PacksFor(platform)
		for _, family := range packs.Keys() {
			families.Set(family, packs.Value(family).Keys())
		}
		known[platform] = families
	}
	return known
}

// ---- loading ----

func exists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return err == nil
}

func isDir(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.IsDir()
}

func isFile(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}

// sortedDirs lists the folders in dir (following links), sorted by path.
func sortedDirs(fsys fs.FS, dir string) []string {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		full := dir + "/" + entry.Name()
		if isDir(fsys, full) {
			out = append(out, full)
		}
	}
	sort.Strings(out)
	return out
}

func markdownFiles(fsys fs.FS, dir string, out *[]string) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		full := dir + "/" + entry.Name()
		info, err := fs.Stat(fsys, full)
		if err != nil {
			continue
		}
		if info.IsDir() {
			markdownFiles(fsys, full, out)
		} else if info.Mode().IsRegular() && strings.HasSuffix(full, ".md") {
			*out = append(*out, full)
		}
	}
}

func fragmentsIn(fsys fs.FS, dir string) ([]Fragment, error) {
	var files []string
	markdownFiles(fsys, dir, &files)
	sort.Strings(files)
	fragments := make([]Fragment, 0, len(files))
	for _, file := range files {
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, err
		}
		fragment, err := parseFragment(file, string(data))
		if err != nil {
			return nil, err
		}
		fragments = append(fragments, fragment)
	}
	return fragments, nil
}

func parseFragment(source, text string) (Fragment, error) {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return Fragment{}, fail(source, "missing front matter")
	}
	end := strings.Index(normalized[4:], "\n---\n")
	if end < 0 {
		return Fragment{}, fail(source, "unterminated front matter")
	}
	end += 4
	parsed, err := yamlutil.Parse(normalized[4:end])
	if err != nil {
		return Fragment{}, fail(source, err.Error())
	}
	meta, ok := parsed.(*omap.Map[any])
	if !ok {
		return Fragment{}, fail(source, "front matter must be a map")
	}
	target, ok := meta.Value("target").(string)
	if !ok || target == "" {
		return Fragment{}, fail(source, "front matter needs a target")
	}
	order := 100
	if value := meta.Value("order"); value != nil {
		n, ok := value.(int)
		if !ok {
			return Fragment{}, fail(source, "order must be an integer")
		}
		order = n
	}
	rules := []string{}
	if value := meta.Value("rules"); value != nil {
		list, ok := value.([]any)
		if !ok {
			return Fragment{}, fail(source, "rules must be a list")
		}
		for _, rule := range list {
			rules = append(rules, yamlutil.DartString(rule))
		}
	}
	when := omap.New[any]()
	if value := meta.Value("when"); value != nil {
		if when, ok = value.(*omap.Map[any]); !ok {
			return Fragment{}, fail(source, "when must be a map")
		}
	}
	return Fragment{Source: source, Target: target, Order: order, Rules: rules, When: when, Body: normalized[end+5:]}, nil
}

func readYaml(fsys fs.FS, file string) (any, error) {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, err
	}
	parsed, err := yamlutil.Parse(string(data))
	if err != nil {
		return nil, fail(file, err.Error())
	}
	return parsed, nil
}

func metasIn(fsys fs.FS, dir string) (map[string]DocMeta, error) {
	out := map[string]DocMeta{}
	if !isDir(fsys, dir) {
		return out, nil
	}
	for _, sub := range sortedDirs(fsys, dir) {
		file := sub + "/meta.yaml"
		if !exists(fsys, file) {
			continue
		}
		meta, err := loadDocMeta(fsys, file)
		if err != nil {
			return nil, err
		}
		out[path.Base(sub)] = meta
	}
	return out, nil
}

func optionalString(value any) *string {
	if value == nil {
		return nil
	}
	text := yamlutil.DartString(value)
	return &text
}

func loadDocMeta(fsys fs.FS, file string) (DocMeta, error) {
	parsed, err := readYaml(fsys, file)
	if err != nil {
		return DocMeta{}, err
	}
	yaml, ok := parsed.(*omap.Map[any])
	if !ok {
		return DocMeta{}, fail(file, "needs a description")
	}
	description, ok := yaml.Value("description").(string)
	if !ok {
		return DocMeta{}, fail(file, "needs a description")
	}
	review := false
	if value := yaml.Value("review"); value != nil {
		if review, ok = value.(bool); !ok {
			return DocMeta{}, fail(file, "review: expected true or false")
		}
	}
	return DocMeta{
		Description:  description,
		WhenToUse:    optionalString(yaml.Value("when_to_use")),
		ArgumentHint: optionalString(yaml.Value("argument_hint")),
		Review:       review,
	}, nil
}

func loadRole(fsys fs.FS, manifest string) (RoleMeta, error) {
	parsed, err := readYaml(fsys, manifest)
	if err != nil {
		return RoleMeta{}, err
	}
	yaml, ok := parsed.(*omap.Map[any])
	if !ok {
		return RoleMeta{}, fail(manifest, "expected a YAML map")
	}
	capability, ok := yaml.Value("capability").(string)
	if !ok || !contains(Capabilities, capability) {
		return RoleMeta{}, fail(manifest, "capability must be one of "+strings.Join(Capabilities, ", "))
	}
	description := ""
	if value := yaml.Value("description"); value != nil {
		description = yamlutil.DartString(value)
	}
	when := omap.New[any]()
	if value := yaml.Value("when"); value != nil {
		if when, ok = value.(*omap.Map[any]); !ok {
			return RoleMeta{}, fail(manifest, "when must be a map")
		}
	}
	return RoleMeta{
		Name:        path.Base(path.Dir(manifest)),
		Description: description,
		Capability:  capability,
		Model:       optionalString(yaml.Value("model")),
		Effort:      optionalString(yaml.Value("effort")),
		When:        when,
	}, nil
}

func packsIn(fsys fs.FS, dir string, fromProject bool) (Packs, error) {
	packs := omap.New[*omap.Map[*Pack]]()
	if !isDir(fsys, dir) {
		return packs, nil
	}
	for _, familyDir := range sortedDirs(fsys, dir) {
		family := path.Base(familyDir)
		for _, optionDir := range sortedDirs(fsys, familyDir) {
			pack, err := loadPack(fsys, family, path.Base(optionDir), optionDir, fromProject)
			if err != nil {
				return nil, err
			}
			options, ok := packs.Get(family)
			if !ok {
				options = omap.New[*Pack]()
				packs.Set(family, options)
			}
			options.Set(pack.Option, pack)
		}
	}
	return packs, nil
}

func loadPack(fsys fs.FS, family, option, dir string, fromProject bool) (*Pack, error) {
	manifest := dir + "/pack.yaml"
	if option == profile.OtherOption {
		return nil, fail(manifest, `option name "`+option+`" is reserved`)
	}
	if !exists(fsys, manifest) {
		return nil, fail(manifest, "missing pack.yaml")
	}
	parsed, err := readYaml(fsys, manifest)
	if err != nil {
		return nil, err
	}
	yaml, ok := parsed.(*omap.Map[any])
	if !ok {
		return nil, fail(manifest, "expected a YAML map")
	}
	params := omap.New[any]()
	if value := yaml.Value("params"); value != nil {
		if params, ok = value.(*omap.Map[any]); !ok {
			return nil, fail(manifest, "params must be a map")
		}
	}
	skills := map[string]DocMeta{}
	skillsDir := dir + "/skills"
	if isDir(fsys, skillsDir) {
		entries, _ := fs.ReadDir(fsys, skillsDir)
		for _, entry := range entries {
			file := skillsDir + "/" + entry.Name()
			if !strings.HasSuffix(file, ".yaml") || !isFile(fsys, file) {
				continue
			}
			meta, err := loadDocMeta(fsys, file)
			if err != nil {
				return nil, err
			}
			skills[strings.TrimSuffix(entry.Name(), ".yaml")] = meta
		}
	}
	fragments, err := fragmentsIn(fsys, dir)
	if err != nil {
		return nil, err
	}
	return &Pack{Family: family, Option: option, Params: params, Fragments: fragments, Skills: skills, FromProject: fromProject}, nil
}

func platformsIn(fsys fs.FS, dir string, fromProject bool) (map[string]*PlatformSpec, error) {
	out := map[string]*PlatformSpec{}
	if !isDir(fsys, dir) {
		return out, nil
	}
	for _, platformDir := range sortedDirs(fsys, dir) {
		spec, err := loadPlatform(fsys, platformDir, fromProject)
		if err != nil {
			return nil, err
		}
		out[spec.Name] = spec
	}
	return out, nil
}

func loadPlatform(fsys fs.FS, dir string, fromProject bool) (*PlatformSpec, error) {
	name := path.Base(dir)
	manifest := dir + "/platform.yaml"
	if name == profile.OtherOption {
		return nil, fail(manifest, `platform name "`+name+`" is reserved`)
	}
	if !exists(fsys, manifest) {
		return nil, fail(manifest, "missing platform.yaml")
	}
	parsed, err := readYaml(fsys, manifest)
	if err != nil {
		return nil, err
	}
	yaml, ok := parsed.(*omap.Map[any])
	if !ok {
		return nil, fail(manifest, "expected a YAML map")
	}
	for _, key := range yaml.Keys() {
		if !contains(platformKeys, key) {
			return nil, fail(manifest, "unknown key "+key)
		}
	}
	detect, ok := profile.StringList(yaml.Value("detect"))
	if !ok || len(detect) == 0 {
		return nil, fail(manifest, "detect: required, a non-empty list of files that identify the platform")
	}
	commands, err := profile.PlatformCommandsFrom(yaml.Value("commands"), func(key, message string) error {
		return fail(manifest, key+": "+message)
	})
	if err != nil {
		return nil, err
	}
	texts := map[string]*string{}
	for _, key := range []string{"description", "link", "doc_comment"} {
		value := yaml.Value(key)
		if value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return nil, fail(manifest, key+": expected a string")
		}
		texts[key] = &text
	}

	servers := omap.New[McpServer]()
	var rawServers any = yaml.Value("mcp_servers")
	if rawServers == nil {
		rawServers = omap.New[any]()
	}
	serverMap, ok := rawServers.(*omap.Map[any])
	if !ok {
		return nil, fail(manifest, "mcp_servers: expected a map of name to command and args")
	}
	for _, serverName := range serverMap.Keys() {
		server, _ := serverMap.Value(serverName).(*omap.Map[any])
		var command, args any
		if server != nil {
			command = server.Value("command")
			args = server.Value("args")
			if args == nil {
				args = []any{}
			}
		}
		commandText, commandOK := command.(string)
		argList, argsOK := args.([]any)
		argStrings := []string{}
		for _, arg := range argList {
			text, ok := arg.(string)
			if !ok {
				argsOK = false
				break
			}
			argStrings = append(argStrings, text)
		}
		if !commandOK || commandText == "" || !argsOK {
			return nil, fail(manifest, "mcp_servers."+serverName+": expected { command: <string>, args: [<string>, ...] }")
		}
		servers.Set(serverName, McpServer{Command: commandText, Args: argStrings})
	}

	all, err := fragmentsIn(fsys, dir)
	if err != nil {
		return nil, err
	}
	packsDir := dir + "/packs"
	fragments := []Fragment{}
	for _, fragment := range all {
		if !strings.HasPrefix(fragment.Source, packsDir+"/") {
			fragments = append(fragments, fragment)
		}
	}
	skills, err := metasIn(fsys, dir+"/skills")
	if err != nil {
		return nil, err
	}
	workflows, err := metasIn(fsys, dir+"/workflows")
	if err != nil {
		return nil, err
	}
	packs, err := packsIn(fsys, packsDir, fromProject)
	if err != nil {
		return nil, err
	}
	return &PlatformSpec{
		Name:        name,
		Root:        dir,
		Description: texts["description"],
		Detect:      detect,
		Commands:    commands,
		Link:        texts["link"],
		DocComment:  texts["doc_comment"],
		McpServers:  servers,
		Fragments:   fragments,
		Skills:      skills,
		Workflows:   workflows,
		Packs:       packs,
		FromProject: fromProject,
	}, nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
