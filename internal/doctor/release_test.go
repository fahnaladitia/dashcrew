package doctor

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/sync"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

const lineProfile = `toolkit: 1
agents: [claude-code]
project:
  release_lines: { main: develop, release: 'release/(?P<version>[0-9.]+)' }
  branch_prefix:
    feat: feat/
    hotfix: hotfix/
    fix: fix/
`

func (f *fixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = f.project
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil && args[0] != "merge" {
		f.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// lineFixture is a synced project committed on develop, with release/2.4.0 cut from it and checked out.
func lineFixture(t *testing.T) *fixture {
	f := setup(t, lineProfile)
	f.installGuard()
	f.sync()
	f.git("init", "-q", "-b", "develop")
	f.git("add", "-A")
	f.git("commit", "-q", "-m", "toolkit")
	f.git("checkout", "-q", "-b", "release/2.4.0")
	return f
}

// lockVersion rewrites the lock's dashcrew_version; with tamper, AGENTS.md also reads as an older catalog wrote it.
func (f *fixture) lockVersion(value string, tamper bool) {
	f.t.Helper()
	parsed, err := dartjson.Decode(f.read(sync.LockPath))
	if err != nil {
		f.t.Fatal(err)
	}
	lock := parsed.(*omap.Map[any])
	lock.Set("dashcrew_version", value)
	if tamper {
		agents := strings.Replace(f.read("AGENTS.md"), "\n", "\nOlder catalog text.\n", 1)
		f.write("AGENTS.md", agents)
		lock.Value("files").(*omap.Map[any]).Set("AGENTS.md", sync.FileHash("AGENTS.md", agents))
	}
	text, err := dartjson.EncodeIndent(lock, "  ")
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(sync.LockPath, text+"\n")
}

func TestReleaseLineReportsItsToolkitAgainstTheMainLine(t *testing.T) {
	t.Run("same version", func(t *testing.T) {
		f := lineFixture(t)
		expect(t, f.doctor(), "release-line", OK, "release/2.4.0 is the release line 2.4.0", "toolkit "+version.Current()+", same as the main line develop")
	})

	t.Run("older than the main line, with held files", func(t *testing.T) {
		f := lineFixture(t)
		f.lockVersion("0.0.0", true)
		checks := f.doctor()
		expect(t, checks, "release-line", Info, "toolkit 0.0.0, the main line develop has "+version.Current(), "dashcrew sync --upgrade")
		expect(t, checks, "version", Info, "this release line keeps it until dashcrew sync --upgrade")
		expect(t, checks, "stale", Info, "held at the lock's toolkit on this release line: AGENTS.md;", "dashcrew sync --upgrade")
		expect(t, checks, "drift", OK)
	})

	t.Run("newer than the main line", func(t *testing.T) {
		f := lineFixture(t)
		f.git("checkout", "-q", "develop")
		f.lockVersion("0.0.0", false)
		f.git("commit", "-q", "-am", "older lock")
		f.git("checkout", "-q", "release/2.4.0")
		expect(t, f.doctor(), "release-line", Warn, "toolkit "+version.Current()+" is newer than the main line develop's 0.0.0")
	})

	t.Run("the main line, a missing main lock and a profile without lines report nothing", func(t *testing.T) {
		f := lineFixture(t)
		f.git("checkout", "-q", "develop")
		if _, ok := f.doctor()["release-line"]; ok {
			t.Error("reported on the main line")
		}
		f.git("checkout", "-q", "release/2.4.0")
		f.git("branch", "-m", "develop", "old-develop")
		if _, ok := f.doctor()["release-line"]; ok {
			t.Error("reported without a main line")
		}
		plain := setup(t, blocProfile)
		plain.sync()
		if _, ok := plain.doctor()["release-line"]; ok {
			t.Error("reported without release_lines")
		}
	})
}

// merged is a project on release/2.5.0 that merged release/2.4.0; each line changed one branch prefix.
func merged(t *testing.T) *fixture {
	f := lineFixture(t)
	f.git("branch", "release/2.5.0")
	edit := func(from, to string) {
		f.write(".ai/project.yaml", strings.Replace(f.read(".ai/project.yaml"), from, to, 1))
		f.sync()
		f.git("add", "-A")
		f.git("commit", "-q", "-m", to)
	}
	edit("feat: feat/", "feat: feature/")
	f.git("checkout", "-q", "release/2.5.0")
	edit("fix: fix/", "fix: bugfix/")
	f.git("merge", "--no-edit", "release/2.4.0")
	return f
}

func TestConflictMarkersInGeneratedFiles(t *testing.T) {
	t.Run("after a merge", func(t *testing.T) {
		f := merged(t)
		checks := f.doctor()
		expect(t, checks, "lock", Fail, sync.LockPath+" has conflict markers", "dashcrew sync --after-merge")
		expect(t, checks, "drift", Fail, "conflict markers: ", "AGENTS.md", "dashcrew sync --after-merge renders them fresh")

		if _, err := (sync.Sync{Catalog: shippedCatalog, ProjectDir: f.project, Home: f.home}).RunWith(sync.Options{AfterMerge: true}); err != nil {
			t.Fatal(err)
		}
		checks = f.doctor()
		expect(t, checks, "lock", OK)
		expect(t, checks, "drift", OK)
		expect(t, checks, "stale", OK)
	})

	t.Run("in a generated file outside a merge", func(t *testing.T) {
		f := setup(t, blocProfile)
		f.sync()
		f.write(".claude/agents/reviewer.md", "<<<<<<< HEAD\na\n=======\nb\n>>>>>>> other\n")
		expect(t, f.doctor(), "drift", Fail, "conflict markers: .claude/agents/reviewer.md", "dashcrew sync --after-merge")
	})
}
