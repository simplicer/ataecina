// Package systemdcreds holds the master key as a systemd credential,
// encrypted with `systemd-creds encrypt --user` (bound to the TPM2 when
// present) and handed to the collector unit with LoadCredentialEncrypted=
// (SPECS §6). For headless collectors.
package systemdcreds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"codeberg.org/simplicer/ataecina/internal/keys/domain"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
	"codeberg.org/simplicer/ataecina/internal/platform/secret"
)

// CredentialName names the credential inside the unit.
const CredentialName = "ataecina.master-key"

const (
	masterKeyBytes = 32
	toolTimeout    = 30 * time.Second
	maxToolOutput  = 4096
)

type record struct {
	Credential string `json:"credential"`
	File       string `json:"file"`
	DropIn     string `json:"drop_in"`
}

// Paths are where the unlocker keeps its files.
type Paths struct {
	// CredentialFile is the encrypted master key, e.g.
	// ~/.config/ataecina/master-key.cred.
	CredentialFile string
	// DropIn hands it to the collector unit, e.g.
	// ~/.config/systemd/user/ataecina-collect.service.d/master-key.conf.
	DropIn string
	// CredentialsDirectory is $CREDENTIALS_DIRECTORY inside the unit.
	CredentialsDirectory string
}

// Unlocker implements domain.Unlocker with systemd credentials.
type Unlocker struct {
	findTool func() (string, error)
	paths    Paths
}

// New runs the systemd-creds that findTool finds.
func New(findTool func() (string, error), paths Paths) Unlocker {
	return Unlocker{findTool: findTool, paths: paths}
}

// Wrap encrypts the master key for this user and writes the drop-in that
// gives it to ataecina-collect.service (run `systemctl --user
// daemon-reload` afterwards).
func (u Unlocker) Wrap(ctx context.Context, master secret.Key) (domain.WrappedMasterKey, error) {
	if err := privatefs.EnsureDir(filepath.Dir(u.paths.CredentialFile)); err != nil {
		return domain.WrappedMasterKey{}, err
	}
	if _, err := u.run(ctx, master.Reveal(), "encrypt", "--user", "--name="+CredentialName, "-", u.paths.CredentialFile); err != nil {
		return domain.WrappedMasterKey{}, err
	}
	if err := os.Chmod(u.paths.CredentialFile, 0o600); err != nil {
		return domain.WrappedMasterKey{}, err
	}
	dropIn := fmt.Sprintf("# Written by `ataecina unlockers add systemd-creds`.\n[Service]\nLoadCredentialEncrypted=%s:%s\n", CredentialName, u.paths.CredentialFile)
	if err := os.MkdirAll(filepath.Dir(u.paths.DropIn), 0o755); err != nil {
		return domain.WrappedMasterKey{}, err
	}
	if err := privatefs.WriteFile(u.paths.DropIn, []byte(dropIn)); err != nil {
		return domain.WrappedMasterKey{}, err
	}
	data, err := json.Marshal(record{Credential: CredentialName, File: u.paths.CredentialFile, DropIn: u.paths.DropIn})
	if err != nil {
		return domain.WrappedMasterKey{}, err
	}
	return domain.NewWrappedMasterKey(domain.SystemdCreds, data), nil
}

// Unwrap reads the credential systemd decrypted for the unit, or decrypts
// the file with systemd-creds outside the unit.
func (u Unlocker) Unwrap(ctx context.Context, wrapped domain.WrappedMasterKey) (secret.Key, error) {
	if dir := u.paths.CredentialsDirectory; dir != "" {
		raw, err := os.ReadFile(filepath.Join(dir, CredentialName))
		if err == nil {
			defer clear(raw)
			return masterKey(raw)
		}
	}
	file := u.paths.CredentialFile
	var stored record
	if json.Unmarshal(wrapped.Data(), &stored) == nil && stored.File != "" {
		file = stored.File
	}
	raw, err := u.run(ctx, nil, "decrypt", "--user", "--name="+CredentialName, file, "-")
	if err != nil {
		return secret.Key{}, err
	}
	defer clear(raw)
	return masterKey(raw)
}

// Forget deletes the encrypted file and the drop-in.
func (u Unlocker) Forget(context.Context, domain.WrappedMasterKey) error {
	for _, path := range []string{u.paths.CredentialFile, u.paths.DropIn} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func masterKey(raw []byte) (secret.Key, error) {
	if len(raw) != masterKeyBytes {
		return secret.Key{}, fmt.Errorf("%w: the systemd credential is not a master key", domain.ErrBrokenSeal)
	}
	return secret.New(raw)
}

// run starts systemd-creds with a fixed argv and no shell.
func (u Unlocker) run(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	tool, err := u.findTool()
	if err != nil {
		return nil, fmt.Errorf("%w: systemd-creds is not installed", domain.ErrUnlockerUnavailable)
	}
	ctx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &limited{buffer: &stderr}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("systemd-creds %s failed: %s", args[0], bytes.TrimSpace(stderr.Bytes()))
	}
	return stdout.Bytes(), nil
}

// limited keeps the first bytes of an error output.
type limited struct{ buffer *bytes.Buffer }

func (l *limited) Write(p []byte) (int, error) {
	if room := maxToolOutput - l.buffer.Len(); room > 0 {
		l.buffer.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}
