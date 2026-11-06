//go:build integration

package logging_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log/slog"

	"codeberg.org/simplicer/ataecina/internal/platform/logging"
)

func TestLinesAreJSONOnStderrAndInTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ataecina.log")
	var stderr bytes.Buffer
	logger, closer, err := logging.Open(&stderr, path, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("subscription collected", "subscription", "claude", "latency_ms", 3)
	logger.Debug("hidden")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !bytes.Equal(data, stderr.Bytes()) {
		t.Fatalf("file and stderr differ:\n%s\n%s", data, stderr.Bytes())
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("%d lines", len(lines))
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"time", "level", "msg", "subscription", "latency_ms"} {
		if _, ok := line[key]; !ok {
			t.Fatalf("missing %q in %s", key, lines[0])
		}
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %04o", info.Mode().Perm())
	}
}

func TestTheFileIsRotatedWhenFull(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ataecina.log")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), logging.MaxFileBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	logger, closer, err := logging.Open(&bytes.Buffer{}, path, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("fresh")
	_ = closer.Close()
	if info, err := os.Stat(path + ".1"); err != nil || info.Size() != logging.MaxFileBytes {
		t.Fatalf("rotated file = %v", err)
	}
	if info, _ := os.Stat(path); info.Size() >= logging.MaxFileBytes {
		t.Fatal("the new file must start empty")
	}
}

func TestOpenFailsWhenTheDirectoryIsMissing(t *testing.T) {
	if _, _, err := logging.Open(&bytes.Buffer{}, filepath.Join(t.TempDir(), "missing", "x.log"), slog.LevelInfo); err == nil {
		t.Fatal("Open must fail")
	}
	logging.Discard().Info("nothing")
}
