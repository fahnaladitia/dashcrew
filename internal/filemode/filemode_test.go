package filemode

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSetExecutableAddsAndClearsOnlyTheExecuteBits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no execute bits on Windows")
	}
	file := filepath.Join(t.TempDir(), "run.sh")
	if err := os.WriteFile(file, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		executable bool
		want       os.FileMode
	}{{true, 0o751}, {true, 0o751}, {false, 0o640}} {
		if err := SetExecutable(file, c.executable); err != nil {
			t.Fatal(err)
		}
		if info, _ := os.Stat(file); info.Mode().Perm() != c.want {
			t.Errorf("executable %v: mode %v, want %v", c.executable, info.Mode().Perm(), c.want)
		}
	}
}
