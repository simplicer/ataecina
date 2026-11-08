//go:build integration

package officialcli_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/platform/officialcli"
)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

type figures struct {
	Spent string `json:"spent"`
}

func recalled(r *officialcli.Reuse) (figures, bool) {
	var got figures
	ok := r.Recall(&got)
	return got, ok
}

func TestKeptFiguresHoldWhileNothingChanged(t *testing.T) {
	dir := t.TempDir()
	watched := filepath.Join(dir, "database")
	if err := os.WriteFile(watched, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	at := &clock{now: time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)}
	reuse := &officialcli.Reuse{File: filepath.Join(dir, "state", "kept.json"), Watch: []string{watched}, MaxAge: time.Hour, Now: at.Now}

	if _, ok := recalled(reuse); ok {
		t.Fatal("nothing was kept yet")
	}
	reuse.Keep(figures{Spent: "1.25"})
	at.now = at.now.Add(59 * time.Minute)
	if got, ok := recalled(reuse); !ok || got.Spent != "1.25" {
		t.Fatalf("Recall = %+v, %v; want the kept figures", got, ok)
	}
	info, err := os.Stat(reuse.File)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("kept file: %v, %v; want 0600", info, err)
	}
	dirInfo, err := os.Stat(filepath.Dir(reuse.File))
	if err != nil || dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("kept directory: %v, %v; want 0700", dirInfo, err)
	}

	at.now = at.now.Add(time.Minute)
	if _, ok := recalled(reuse); ok {
		t.Fatal("figures as old as MaxAge still held")
	}
}

func TestAChangedWatchedFileCallsForNewFigures(t *testing.T) {
	dir := t.TempDir()
	database, journal := filepath.Join(dir, "database"), filepath.Join(dir, "database-wal")
	if err := os.WriteFile(database, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	reuse := &officialcli.Reuse{File: filepath.Join(dir, "state", "kept.json"), Watch: []string{database, journal}, MaxAge: time.Hour}

	reuse.Keep(figures{Spent: "1"})
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(database, later, later); err != nil {
		t.Fatal(err)
	}
	if _, ok := recalled(reuse); ok {
		t.Fatal("a changed watched file did not call for new figures")
	}

	reuse.Keep(figures{Spent: "2"})
	if err := os.WriteFile(journal, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := recalled(reuse); ok {
		t.Fatal("a watched file that appeared did not call for new figures")
	}
}

func TestBrokenOrForeignKeptFilesAreIgnored(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		content string
		mode    os.FileMode
	}{
		"not json":        {"{not json", 0o600},
		"wrong figures":   {`{"taken_at":"` + time.Now().UTC().Format(time.RFC3339) + `","watched":{},"value":[1]}`, 0o600},
		"in the future":   {`{"taken_at":"` + time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `","watched":{},"value":{}}`, 0o600},
		"readable by all": {`{"taken_at":"` + time.Now().UTC().Format(time.RFC3339) + `","watched":{},"value":{}}`, 0o644},
	}
	for name, c := range cases {
		file := filepath.Join(state, "kept.json")
		if err := os.WriteFile(file, []byte(c.content), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(file, c.mode); err != nil {
			t.Fatal(err)
		}
		if _, ok := recalled(&officialcli.Reuse{File: file, MaxAge: time.Hour}); ok {
			t.Fatalf("%s: a kept file was trusted", name)
		}
	}
}

func TestANilReuseKeepsNothing(t *testing.T) {
	var reuse *officialcli.Reuse
	reuse.Keep(figures{Spent: "1"})
	if _, ok := recalled(reuse); ok {
		t.Fatal("a nil Reuse recalled figures")
	}
}

func TestOneOfficialCLIRunsAtATime(t *testing.T) {
	busy := filepath.Join(t.TempDir(), "busy")
	c := command(script(t, `mkdir '`+busy+`' || exit 3; sleep 0.3; rmdir '`+busy+`'`))
	var wait sync.WaitGroup
	failures := make(chan error, 3)
	for range 3 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := c.Output(context.Background()); err != nil {
				failures <- err
			}
		}()
	}
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Fatalf("two official CLIs ran at once: %v", err)
	}
}

func TestWaitingForTheTurnHonoursTheCallersDeadline(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "running")
	slow := command(script(t, `touch '`+marker+`'; sleep 1`))
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = slow.Output(context.Background())
	}()
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the slow command never started")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	begun := time.Now()
	_, err := command(script(t, "echo quick")).Output(ctx)
	if !errors.Is(err, officialcli.ErrFailed) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Output = %v, want a deadline while waiting", err)
	}
	if time.Since(begun) > 600*time.Millisecond {
		t.Fatal("the wait outlived the caller's deadline")
	}
	<-done
}
