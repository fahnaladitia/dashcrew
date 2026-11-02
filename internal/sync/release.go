package sync

import (
	"regexp"
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/lines"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

// ProfilePath is the team's profile.
const ProfilePath = ".ai/project.yaml"

var (
	conflictStart = regexp.MustCompile(`(?m)^<{7}(?: |$)`)
	conflictEnd   = regexp.MustCompile(`(?m)^>{7}(?: |$)`)
)

// HasConflictMarkers is true when text holds a git conflict: a line starting `<<<<<<<` and one starting `>>>>>>>`.
func HasConflictMarkers(text string) bool {
	return conflictStart.MatchString(text) && conflictEnd.MatchString(text)
}

// LineHold is what a sync on a release line keeps at the lock's toolkit.
//
// The rule (R1). It applies only on a protected branch (a release or hotfix line, or a branch that records a
// release; see lines.Resolution.Protected), without --upgrade, and only when the catalog is newer than the lock's:
// the lock has no dashcrew_version, or an older one than this dashcrew, or its `inputs.catalog` digest differs
// from this catalog's. Then every generated file whose rendered hash differs from the lock (added, changed or
// removed) is judged on its own:
//   - profile-driven, and written, when rendering the profile the lock was made from (`inputs.profile`, the
//     current .ai/project.yaml or HEAD's when its hash matches) with this catalog reproduces the locked file:
//     this catalog does not change it, so the difference comes from the profile;
//   - catalog-driven, and held, otherwise, including when no such profile is available.
//
// A held file keeps its content and lock hash, and the lock keeps its dashcrew_version, guard_min_version and
// catalog digest, so the line's toolkit version never moves without --upgrade.
type LineHold struct {
	Line lines.Resolution
	// LockVersion is the lock's dashcrew_version; empty when it has none.
	LockVersion string
	// SameVersion is true when only the catalog digest differs.
	SameVersion bool
	// Files are the held generated files, sorted.
	Files []string
	// MainLine is the profile's main line branch.
	MainLine string
}

// Message explains the held files and the two ways forward.
func (h *LineHold) Message() string {
	reason := "dashcrew " + version.Current() + " is newer than the lock's " + h.LockVersion
	switch {
	case h.LockVersion == "":
		reason = "the lock records no dashcrew version"
	case h.SameVersion:
		reason = "this dashcrew's catalog differs from the one that wrote the lock (both " + h.LockVersion + ")"
	}
	var b strings.Builder
	b.WriteString("Held: " + h.Line.Describe() + ", and " + reason + ".\n")
	b.WriteString("These generated files would change with the newer catalog and were left as they are:\n")
	for _, path := range h.Files {
		b.WriteString("  " + path + "\n")
	}
	b.WriteString("Changes that come from .ai/project.yaml on this line are still written. To take the newer toolkit, either:\n")
	b.WriteString("  dashcrew sync --upgrade (the upgrade lands on this line), or\n")
	b.WriteString("  upgrade on the main line " + h.MainLine + " (dashcrew sync there, commit) and merge it down to this line.")
	return b.String()
}

// profileHash is the hash `inputs.profile` records for a profile text.
func profileHash(text string) string { return FileHash(ProfilePath, text) }

// lockInputs is `inputs` of a lock: the catalog digest and profile hash it was rendered from; empty when absent.
func lockInputs(lock *omap.Map[any]) (catalogDigest, profile string) {
	inputs, _ := lock.Value("inputs").(*omap.Map[any])
	if inputs == nil {
		return "", ""
	}
	catalogDigest, _ = inputs.Value("catalog").(string)
	profile, _ = inputs.Value("profile").(string)
	return catalogDigest, profile
}

// hold applies the R1 rule (see LineHold); nil when nothing is held back and the lock may take this toolkit.
func (s Sync) hold(rendered *Rendered, lock *omap.Map[any], profileText string, upgrade bool) *LineHold {
	rl := rendered.Profile.ReleaseLines
	if rl == nil || upgrade || !lock.Has("files") {
		return nil
	}
	all, err := lines.List(s.ProjectDir, rl)
	if err != nil {
		return nil
	}
	resolution := lines.Resolve(s.ProjectDir, rl, all, "")
	if !resolution.Protected() {
		return nil
	}
	lockVersion, _ := lock.Value("dashcrew_version").(string)
	lockCatalog, lockProfile := lockInputs(lock)
	olderVersion := lockVersion == "" || version.Compare(lockVersion, version.Current()) < 0
	if !olderVersion && (lockCatalog == "" || lockCatalog == s.Catalog.Digest) {
		return nil
	}
	h := &LineHold{Line: resolution, LockVersion: lockVersion, SameVersion: !olderVersion, Files: []string{}, MainLine: rl.Main}
	files, locked := rendered.Files, lockFiles(lock)
	previous := s.previousRender(lockProfile, profileHash(profileText), files)
	for _, path := range union(files.Keys(), locked.Keys()) {
		content, isRendered := files.Get(path)
		hash, isLocked := locked.Get(path)
		if isRendered == isLocked && (!isRendered || FileHash(path, content) == hash) {
			continue
		}
		if previous != nil && reproduces(previous, locked, path) {
			continue
		}
		h.Files = append(h.Files, path)
	}
	sort.Strings(h.Files)
	return h
}

// reproduces is true when previous renders path exactly as the lock records it, absent included.
func reproduces(previous, locked *omap.Map[string], path string) bool {
	content, isRendered := previous.Get(path)
	hash, isLocked := locked.Get(path)
	return isRendered == isLocked && (!isRendered || FileHash(path, content) == hash)
}

// previousRender is the output of the profile the lock was rendered from, with this catalog: current when the
// profile is unchanged, else HEAD's .ai/project.yaml when its hash is the locked one; nil when neither is.
func (s Sync) previousRender(lockProfile, currentProfile string, current *omap.Map[string]) *omap.Map[string] {
	if lockProfile == "" {
		return nil
	}
	if lockProfile == currentProfile {
		return current
	}
	text, ok := lines.Show(s.ProjectDir, "HEAD:./"+ProfilePath)
	if !ok || profileHash(text) != lockProfile {
		return nil
	}
	previous := s
	previous.profileText = &text
	rendered, err := previous.Render(false, nil)
	if err != nil {
		return nil
	}
	return rendered.Files
}

// Hold is what a sync would keep at the lock's toolkit on this branch; nil when nothing would be held.
func (s Sync) Hold() (*LineHold, error) {
	rendered, err := s.Render(false, nil)
	if err != nil {
		return nil, err
	}
	lock, err := s.readLock()
	if err != nil {
		return nil, err
	}
	text, err := s.project().Read(ProfilePath)
	if err != nil {
		return nil, err
	}
	h := s.hold(rendered, lock, text, false)
	if h == nil || len(h.Files) == 0 {
		return nil, nil
	}
	return h, nil
}

// MergeLock is the lock `sync --after-merge` works from. When `.ai/lock.json` parses it is that lock and false.
// When it carries conflict markers it is our side (index stage 2, else HEAD) with the files and MCP servers only
// their side (stage 3) has added, and true.
func (s Sync) MergeLock() (*omap.Map[any], bool, error) {
	project := s.project()
	if !project.Exists(LockPath) {
		return omap.New[any](), false, nil
	}
	text, err := project.Read(LockPath)
	if err != nil {
		return nil, false, err
	}
	if !HasConflictMarkers(text) {
		lock, err := s.readLock()
		return lock, false, err
	}
	decode := func(spec string) *omap.Map[any] {
		text, ok := lines.Show(s.ProjectDir, spec)
		if !ok {
			return nil
		}
		value, err := dartjson.Decode(text)
		if err != nil {
			return nil
		}
		lock, _ := value.(*omap.Map[any])
		return lock
	}
	ours := decode(":2:./" + LockPath)
	if ours == nil {
		ours = decode("HEAD:./" + LockPath)
	}
	if ours == nil {
		return nil, true, &Error{Message: LockPath + " has conflict markers and git has no clean copy of it; take one side with " +
			"git checkout --ours " + LockPath + ", then rerun dashcrew sync --after-merge."}
	}
	if theirs := decode(":3:./" + LockPath); theirs != nil {
		files, _ := ours.Value("files").(*omap.Map[any])
		theirFiles, _ := theirs.Value("files").(*omap.Map[any])
		if files != nil && theirFiles != nil {
			for _, path := range theirFiles.Keys() {
				if !files.Has(path) {
					files.Set(path, theirFiles.Value(path))
				}
			}
		}
		ourServers, _ := ours.Value("mcp_servers").(*omap.Map[any])
		theirServers, _ := theirs.Value("mcp_servers").(*omap.Map[any])
		if ourServers != nil && theirServers != nil {
			for _, agent := range theirServers.Keys() {
				names, _ := ourServers.Value(agent).([]any)
				theirNames, _ := theirServers.Value(agent).([]any)
				for _, name := range theirNames {
					if !containsAny(names, name) {
						names = append(names, name)
					}
				}
				ourServers.Set(agent, names)
			}
		}
	}
	return ours, true, nil
}

func containsAny(list []any, value any) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// lineInputs is `inputs` for the lock of a profile with release_lines; nil without them. A held toolkit keeps the
// lock's catalog digest, and drops it when the lock had none.
func lineInputs(p *profile.Profile, catalogDigest, profileText string) *omap.Map[any] {
	if p.ReleaseLines == nil {
		return nil
	}
	inputs := omap.New[any]()
	if catalogDigest != "" {
		inputs.Set("catalog", catalogDigest)
	}
	inputs.Set("profile", profileHash(profileText))
	return inputs
}
