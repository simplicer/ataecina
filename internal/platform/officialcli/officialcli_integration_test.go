//go:build integration

package officialcli_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/platform/officialcli"
)

// script writes an executable fake CLI and returns a finder for it.
func script(t *testing.T, body string) func() (string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-cli")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	return func() (string, error) { return path, nil }
}

func command(find func() (string, error)) officialcli.Command {
	return officialcli.Command{Find: find, Args: []string{"usage", "--output", "json"}, Env: []string{"FAKE_SWITCH=1"}, Timeout: 2 * time.Second, MaxOutput: 64}
}

func TestTheArgvAndEnvironmentAreFixed(t *testing.T) {
	out, err := command(script(t, `echo "$@ $FAKE_SWITCH"; echo noise >&2`)).Output(context.Background())
	if err != nil || strings.TrimSpace(string(out)) != "usage --output json 1" {
		t.Fatalf("Output = %q, %v", out, err)
	}
}

func TestEveryFailureIsClassified(t *testing.T) {
	cases := map[string]struct {
		find func() (string, error)
		want error
	}{
		"missing":   {func() (string, error) { return "", exec.ErrNotFound }, officialcli.ErrNotFound},
		"exit 1":    {script(t, "exit 1"), officialcli.ErrFailed},
		"too large": {script(t, "head -c 100 /dev/zero"), officialcli.ErrTooLarge},
		"too slow":  {script(t, "sleep 10"), officialcli.ErrFailed},
	}
	for name, c := range cases {
		started := time.Now()
		if _, err := command(c.find).Output(context.Background()); !errors.Is(err, c.want) {
			t.Fatalf("%s: %v, want %v", name, err, c.want)
		}
		if time.Since(started) > 6*time.Second {
			t.Fatalf("%s: the timeout was not honoured", name)
		}
	}
}
