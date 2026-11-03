package me

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// RepoFile is a repo's personal file: the member's role and level in that repo only. Never committed.
const RepoFile = ".ai/me.yaml"

// RepoFiles are every personal path inside a repo, relative to its root, for the git exclude file.
var RepoFiles = []string{RepoFile, ".claude/rules/" + ruleFile, ".agents/rules/" + ruleFile}

// RepoProfile is `.ai/me.yaml`; an empty field falls back to the home profile.
type RepoProfile struct {
	Role  string
	Level string
	// Skills are the optional project skills of this repo the member turns on.
	Skills []string
}

// ParseRepo reads `.ai/me.yaml` text: role, level and skills.
func ParseRepo(text string) (*RepoProfile, error) {
	value, err := yamlutil.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("%s is not valid YAML: %w", RepoFile, err)
	}
	r := &RepoProfile{}
	if value == nil {
		return r, nil
	}
	root, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fmt.Errorf("%s: expected a map", RepoFile)
	}
	for _, key := range root.Keys() {
		switch key {
		case "role", "level", "skills":
		case "stacks", "chat_language", "agents":
			return nil, fmt.Errorf("%s %s: belongs in ~/.dashcrew/me.yaml, the same for every repo", RepoFile, key)
		default:
			return nil, fmt.Errorf("%s %s: unknown key; a repo holds only role, level and skills", RepoFile, key)
		}
	}
	if root.Has("role") {
		if r.Role, ok = root.Value("role").(string); !ok || !contains(Roles, r.Role) {
			return nil, fmt.Errorf("%s role: expected one of %s", RepoFile, strings.Join(Roles, ", "))
		}
	}
	if root.Has("level") {
		if r.Level, ok = root.Value("level").(string); !ok || !contains(Levels, r.Level) {
			return nil, fmt.Errorf("%s level: expected one of %s", RepoFile, strings.Join(Levels, ", "))
		}
	}
	if r.Skills, err = namesIn(RepoFile, root.Value("skills"), "skills", validSkillName); err != nil {
		return nil, err
	}
	return r, nil
}

// LoadRepo reads `.ai/me.yaml` under dir; nil when there is none.
func LoadRepo(dir string) (*RepoProfile, error) {
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(RepoFile)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ParseRepo(string(data))
}

// Text is the repo profile as `.ai/me.yaml`.
func (r *RepoProfile) Text() string {
	out := "# Your role, level and optional skills in this repo only. Never committed (listed in .git/info/exclude).\n" +
		"# stacks, chat_language and agents live in ~/.dashcrew/me.yaml. Run dashcrew me sync after editing.\n"
	if r.Role != "" {
		out += "role: " + r.Role + "   # " + strings.Join(Roles, " | ") + "\n"
	}
	if r.Level != "" {
		out += "level: " + r.Level + "   # " + strings.Join(Levels, " | ") + "\n"
	}
	if len(r.Skills) > 0 {
		out += "skills: [" + strings.Join(r.Skills, ", ") + "]   # optional skills of this repo (.ai/skills) you turn on\n"
	}
	return out
}

// Save writes `.ai/me.yaml` under dir.
func (r *RepoProfile) Save(dir string) error {
	path := filepath.Join(dir, filepath.FromSlash(RepoFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(r.Text()), 0o644)
}

// Source says where an effective value came from.
type Source string

const (
	FromHome Source = "home"
	FromRepo Source = "repo"
)

// Effective is the profile in force in one repo: the home profile with the repo's role and level over it.
type Effective struct {
	Profile
	RoleFrom, LevelFrom Source
}

// EffectiveIn merges the home profile with a repo profile; repo may be nil.
func EffectiveIn(home *Profile, repo *RepoProfile) Effective {
	e := Effective{Profile: *home, RoleFrom: FromHome, LevelFrom: FromHome}
	if repo == nil {
		return e
	}
	if repo.Role != "" {
		e.Role, e.RoleFrom = repo.Role, FromRepo
	}
	if repo.Level != "" {
		e.Level, e.LevelFrom = repo.Level, FromRepo
	}
	return e
}

// Describe is one line: the effective role and level with their sources.
func (e Effective) Describe() string {
	return "role " + e.Role + " (" + string(e.RoleFrom) + "), level " + e.Level + " (" + string(e.LevelFrom) + ")"
}
