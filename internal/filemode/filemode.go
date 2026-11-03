// Package filemode sets or clears a file's execute bits, the one rule every writer of dashcrew files follows.
package filemode

import "os"

// SetExecutable adds the execute bits, as `chmod +x` does under the usual umask, or clears them, leaving the other
// permission bits as they are.
func SetExecutable(file string, executable bool) error {
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	perm := info.Mode().Perm() &^ 0o111
	if executable {
		perm = info.Mode().Perm() | 0o111
	}
	if perm == info.Mode().Perm() {
		return nil
	}
	return os.Chmod(file, perm)
}
