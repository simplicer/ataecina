// Package privatefs reads and writes the owner's private files: owned by
// the current user, closed to group and others, never through symlinks,
// replaced atomically. Like ssh, Ataecina refuses anything looser.
package privatefs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// ErrNotPrivate reports a file or directory that others could read or
// change, that belongs to another user, or that is a symlink.
var ErrNotPrivate = errors.New("not private")

// ErrTooLarge reports a file larger than its reader accepts.
var ErrTooLarge = errors.New("file too large")

const (
	dirMode  fs.FileMode = 0o700
	fileMode fs.FileMode = 0o600
)

// EnsureDir creates a directory (and its parents) with mode 0700 and
// checks that an existing one is private.
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, dirMode); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return RequireDir(path)
}

// RequireDir checks that a directory exists, is ours and is 0700 or tighter.
func RequireDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("check %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrNotPrivate, path)
	}
	return requireOwnedAndClosed(path, info, "chmod 700")
}

// ReadFile reads a private file of at most limit bytes without following
// symlinks. A missing file wraps fs.ErrNotExist.
func ReadFile(path string, limit int64) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if errors.Is(err, syscall.ELOOP) {
		return nil, fmt.Errorf("%w: %s is a symlink", ErrNotPrivate, path)
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("check %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrNotPrivate, path)
	}
	if err := requireOwnedAndClosed(path, info, "chmod 600"); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: %s is larger than %d bytes", ErrTooLarge, path, limit)
	}
	return data, nil
}

// WriteFile replaces a file atomically with mode 0600: a temporary file in
// the same directory is written, synced and renamed over the target.
func WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := fill(temporary, data); err != nil {
		_ = os.Remove(temporary.Name())
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		_ = os.Remove(temporary.Name())
		return fmt.Errorf("write %s: %w", path, err)
	}
	return syncDir(dir)
}

// fill makes a file private, writes it, syncs it and closes it.
func fill(file *os.File, data []byte) error {
	if err := file.Chmod(fileMode); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("sync %s: %w", dir, err)
	}
	defer handle.Close()
	if err := handle.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", dir, err)
	}
	return nil
}

func requireOwnedAndClosed(path string, info fs.FileInfo, fix string) error {
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is a symlink", ErrNotPrivate, path)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("%w: %s belongs to another user", ErrNotPrivate, path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%w: %s has mode %04o; run: %s %s", ErrNotPrivate, path, info.Mode().Perm(), fix, path)
	}
	return nil
}
