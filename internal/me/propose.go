package me

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// RepoPlatform is the `platform` of `<dir>/.ai/project.yaml`: flutter when the key is absent, empty when the
// file is missing or unreadable.
func RepoPlatform(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, ".ai", "project.yaml"))
	if err != nil {
		return ""
	}
	value, err := yamlutil.Parse(string(data))
	if err != nil {
		return ""
	}
	root, ok := value.(*omap.Map[any])
	if !ok {
		return ""
	}
	if platform, ok := root.Value("platform").(string); ok && stackName.MatchString(platform) {
		return platform
	}
	if root.Has("platform") {
		return ""
	}
	return "flutter"
}

// Propose is what `me init` offers: the existing profile with the repo's platform added to its stacks, or a new
// one with a role from that platform and the agents whose config folder exists on this machine.
func Propose(existing *Profile, platform string, places Places) *Profile {
	if existing != nil {
		p := *existing
		p.Stacks = append([]string(nil), existing.Stacks...)
		if platform != "" && !contains(p.Stacks, platform) {
			p.Stacks = append(p.Stacks, platform)
		}
		return &p
	}
	p := &Profile{Role: "fullstack", Level: "regular"}
	switch platform {
	case "flutter":
		p.Role = "mobile"
	case "go":
		p.Role = "be"
	}
	if platform != "" {
		p.Stacks = []string{platform}
	}
	for i, dir := range []string{places.ClaudeDir, places.CodexDir, places.GeminiDir} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			p.Agents = append(p.Agents, Agents[i])
		}
	}
	if len(p.Agents) == 0 {
		p.Agents = []string{"claude-code"}
	}
	return p
}

// LoadOrNil is Load with no profile as nil.
func LoadOrNil(home string) (*Profile, error) {
	p, err := Load(home)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return p, err
}
