// Package pathlookup finds an official CLI (codex, opencode) in a PATH
// value, as a shell would, without running anything. Relative PATH entries
// are ignored so the current directory can never supply the binary.
package pathlookup

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Find returns a finder of the executable name in the directories of path.
func Find(name, path string) func() (string, error) {
	return func() (string, error) {
		for _, dir := range filepath.SplitList(path) {
			if !filepath.IsAbs(dir) {
				continue
			}
			candidate := filepath.Join(dir, name)
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
				return candidate, nil
			}
		}
		return "", exec.ErrNotFound
	}
}
