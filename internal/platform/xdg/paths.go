// Package xdg resolves Ataecina's directories from the XDG Base Directory
// specification.
package xdg

import (
	"errors"
	"path/filepath"
)

const application = "ataecina"

// ErrNoHome reports that HOME is unset or relative.
var ErrNoHome = errors.New("HOME is not set to an absolute path")

// ErrNoRuntimeDir reports that XDG_RUNTIME_DIR is unset or relative.
var ErrNoRuntimeDir = errors.New("XDG_RUNTIME_DIR is not set to an absolute path; Ataecina keeps its snapshot there (log in through a session manager such as systemd-logind)")

// Paths are Ataecina's directories.
type Paths struct {
	home    string
	config  string
	state   string
	runtime string
}

// Resolve reads HOME and the XDG variables through getenv. Relative XDG
// values are ignored, as the specification requires.
func Resolve(getenv func(string) string) (Paths, error) {
	home := getenv("HOME")
	if !filepath.IsAbs(home) {
		return Paths{}, ErrNoHome
	}
	base := func(variable, fallback string) string {
		if value := getenv(variable); filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
		return filepath.Join(home, fallback)
	}
	paths := Paths{
		home:   filepath.Clean(home),
		config: filepath.Join(base("XDG_CONFIG_HOME", ".config"), application),
		state:  filepath.Join(base("XDG_STATE_HOME", filepath.Join(".local", "state")), application),
	}
	if runtime := getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(runtime) {
		paths.runtime = filepath.Join(filepath.Clean(runtime), application)
	}
	return paths, nil
}

// Home is the owner's home directory.
func (p Paths) Home() string { return p.home }

// ConfigDir holds config.json and keys.json.
func (p Paths) ConfigDir() string { return p.config }

// ConfigFile is config.json.
func (p Paths) ConfigFile() string { return filepath.Join(p.config, "config.json") }

// StateDir holds alert state, declared balances, log offsets and the log.
func (p Paths) StateDir() string { return p.state }

// LogFile is the Loki-compatible log.
func (p Paths) LogFile() string { return filepath.Join(p.state, "ataecina.log") }

// RuntimeDir holds the snapshot and the hook signals (tmpfs).
func (p Paths) RuntimeDir() (string, error) {
	if p.runtime == "" {
		return "", ErrNoRuntimeDir
	}
	return p.runtime, nil
}
