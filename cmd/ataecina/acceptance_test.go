//go:build acceptance

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/configfile"
	"codeberg.org/simplicer/ataecina/internal/platform/xdg"
)

// Acceptance tests drive whole flows through run(): configuration,
// collection, snapshot file and displays, with in-process fake providers
// and a clock the test moves. HOME and XDG live in a temporary directory.

type movableClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *movableClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *movableClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type scriptedSource struct {
	mu     sync.Mutex
	answer func(subscription domain.Subscription, now time.Time) (domain.Reading, error)
	clock  *movableClock
}

func (s *scriptedSource) Read(_ context.Context, subscription domain.Subscription) (domain.Reading, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.answer(subscription, s.clock.Now())
}

func (s *scriptedSource) set(answer func(domain.Subscription, time.Time) (domain.Reading, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.answer = answer
}

type world struct {
	t      *testing.T
	root   string
	vars   map[string]string
	clock  *movableClock
	source *scriptedSource
	env    environment
}

func newWorld(t *testing.T, config string) *world {
	t.Helper()
	root := t.TempDir()
	vars := map[string]string{
		"HOME":            filepath.Join(root, "home"),
		"XDG_CONFIG_HOME": filepath.Join(root, "config"),
		"XDG_RUNTIME_DIR": filepath.Join(root, "run"),
		"XDG_STATE_HOME":  filepath.Join(root, "state"),
	}
	for _, dir := range []string{vars["XDG_CONFIG_HOME"] + "/ataecina", vars["XDG_RUNTIME_DIR"]} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(vars["XDG_CONFIG_HOME"], "ataecina", "config.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	clock := &movableClock{now: time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)}
	source := &scriptedSource{clock: clock}
	registry := standardSources().with(
		catalog.Entry{Source: "scripted", Provider: "acme", Fidelity: domain.FidelityReported},
		func(configfile.SubscriptionSettings, xdg.Paths, environment) domain.Source { return source },
	)
	return &world{t: t, root: root, vars: vars, clock: clock, source: source, env: environment{
		getenv:  func(key string) string { return vars[key] },
		stdin:   strings.NewReader(""),
		clock:   clock,
		sources: registry,
	}}
}

func (w *world) run(args ...string) (int, string, string) {
	w.t.Helper()
	var stdout, stderr bytes.Buffer
	env := w.env
	env.stdout, env.stderr = &stdout, &stderr
	code := run(context.Background(), args, env)
	return code, stdout.String(), stderr.String()
}

type statusDocument struct {
	Data struct {
		Subscriptions []struct {
			ID         string  `json:"id"`
			Outcome    string  `json:"outcome"`
			Freshness  string  `json:"freshness"`
			ErrorClass *string `json:"error_class"`
			Meters     []struct {
				Window      string  `json:"window"`
				UsedPercent float64 `json:"used_percent"`
				ResetsAt    *string `json:"resets_at"`
				Estimated   bool    `json:"estimated"`
			} `json:"meters"`
		} `json:"subscriptions"`
	} `json:"data"`
	Meta struct {
		WorstUsedPercent *float64 `json:"worst_used_percent"`
	} `json:"meta"`
}

func (w *world) status() statusDocument {
	w.t.Helper()
	code, stdout, stderr := w.run("status", "--json")
	if code != exitOK {
		w.t.Fatalf("status --json = %d: %s", code, stderr)
	}
	var doc statusDocument
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		w.t.Fatal(err)
	}
	return doc
}

func (w *world) collect() {
	w.t.Helper()
	if code, _, stderr := w.run("collect"); code != exitOK {
		w.t.Fatalf("collect = %d: %s", code, stderr)
	}
}

const twoSubscriptions = `{"version": 1, "subscriptions": [
  {"id": "acme", "source": "scripted", "plan": "pro"},
  {"id": "zai", "source": "zai-api"}
]}`

func fiveHours(used float64, resetsIn time.Duration) func(domain.Subscription, time.Time) (domain.Reading, error) {
	return func(subscription domain.Subscription, now time.Time) (domain.Reading, error) {
		return domain.NewReading(domain.ReadingFacts{
			Subscription: subscription.ID(), Provider: subscription.Provider(), Source: subscription.Source(),
			Fidelity: subscription.Fidelity(), Outcome: domain.OutcomeOK, ObservedAt: now,
			Meters: []domain.Meter{domain.NewMeter("", domain.FiveHourWindow(), domain.UsedPercent(used), now.Add(resetsIn))},
		})
	}
}

func TestCollectThenStatus(t *testing.T) {
	w := newWorld(t, twoSubscriptions)
	w.source.set(fiveHours(23.5, 2*time.Hour))
	w.collect()

	doc := w.status()
	if len(doc.Data.Subscriptions) != 2 {
		t.Fatalf("%d subscriptions", len(doc.Data.Subscriptions))
	}
	acme, zai := doc.Data.Subscriptions[0], doc.Data.Subscriptions[1]
	if acme.Outcome != "OK" || acme.Freshness != "FRESH" || acme.Meters[0].UsedPercent != 23.5 {
		t.Fatalf("acme = %+v", acme)
	}
	if zai.Outcome != "UNSUPPORTED" {
		t.Fatalf("zai = %+v", zai)
	}
	_, table, _ := w.run()
	if !strings.Contains(table, "5h 23.5% ↻ 2h00m") {
		t.Fatalf("table:\n%s", table)
	}
	logged, _ := os.ReadFile(filepath.Join(w.root, "state", "ataecina", "ataecina.log"))
	if strings.Count(string(logged), `"msg":"subscription collected"`) != 2 {
		t.Fatalf("log:\n%s", logged)
	}
}

func TestABrokenProviderIsIsolated(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": [
	  {"id": "acme", "source": "scripted"},
	  {"id": "other", "source": "scripted"}
	]}`)
	w.source.set(func(subscription domain.Subscription, now time.Time) (domain.Reading, error) {
		if subscription.ID() == "acme" {
			return domain.Reading{}, domain.Failure(domain.ErrorContractChanged, "unexpected shape", nil)
		}
		return fiveHours(10, time.Hour)(subscription, now)
	})
	w.collect()
	doc := w.status()
	if doc.Data.Subscriptions[0].Outcome != "BROKEN" || doc.Data.Subscriptions[1].Outcome != "OK" {
		t.Fatalf("outcomes = %+v", doc.Data.Subscriptions)
	}
}

func TestReadingsGoStaleAndWindowsAreReborn(t *testing.T) {
	w := newWorld(t, twoSubscriptions)
	w.source.set(fiveHours(96, 2*time.Hour))
	w.collect()
	if worst := w.status().Meta.WorstUsedPercent; worst == nil || *worst != 96 {
		t.Fatalf("worst = %v", worst)
	}

	w.clock.advance(30 * time.Minute)
	if acme := w.status().Data.Subscriptions[0]; acme.Freshness != "STALE" || acme.Meters[0].Estimated {
		t.Fatalf("after 30 minutes: %+v", acme)
	}

	w.clock.advance(2 * time.Hour)
	acme := w.status().Data.Subscriptions[0]
	if !acme.Meters[0].Estimated || acme.Meters[0].UsedPercent != 0 || acme.Meters[0].ResetsAt != nil {
		t.Fatalf("after the reset the window is reborn: %+v", acme.Meters[0])
	}
}

func TestATransientFailureKeepsTheLastGoodReading(t *testing.T) {
	w := newWorld(t, twoSubscriptions)
	w.source.set(fiveHours(40, 3*time.Hour))
	w.collect()
	w.clock.advance(5 * time.Minute)
	w.source.set(func(domain.Subscription, time.Time) (domain.Reading, error) {
		return domain.Reading{}, domain.Failure(domain.ErrorUnavailable, "provider down", nil)
	})
	w.collect()
	acme := w.status().Data.Subscriptions[0]
	if acme.Outcome != "OK" || acme.ErrorClass == nil || *acme.ErrorClass != "UNAVAILABLE" || acme.Meters[0].UsedPercent != 40 {
		t.Fatalf("kept reading = %+v", acme)
	}
}

func TestLooseConfigurationStopsTheCollector(t *testing.T) {
	w := newWorld(t, twoSubscriptions)
	if err := os.Chmod(filepath.Join(w.root, "config", "ataecina", "config.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := w.run("collect")
	if code != exitFailure || !strings.Contains(stderr, "chmod 600") {
		t.Fatalf("collect = %d: %s", code, stderr)
	}
}

func TestStatusWithoutARuntimeDirectoryIsAProblem(t *testing.T) {
	w := newWorld(t, twoSubscriptions)
	getenv := w.env.getenv
	w.env.getenv = func(key string) string {
		if key == "XDG_RUNTIME_DIR" {
			return ""
		}
		return getenv(key)
	}
	code, _, stderr := w.run("status", "--json")
	var problem map[string]string
	if code != exitFailure || json.Unmarshal([]byte(stderr), &problem) != nil || problem["type"] != "urn:ataecina:problem:environment" {
		t.Fatalf("status = %d: %s", code, stderr)
	}
}

func TestCommandLine(t *testing.T) {
	w := newWorld(t, twoSubscriptions)
	if code, out, _ := w.run("version"); code != exitOK || !strings.HasPrefix(out, "ataecina ") {
		t.Fatalf("version = %d %q", code, out)
	}
	if code, out, _ := w.run("--version"); code != exitOK || !strings.HasPrefix(out, "ataecina ") {
		t.Fatalf("--version = %d %q", code, out)
	}
	for _, args := range [][]string{{"help"}, {"--help"}} {
		if code, out, _ := w.run(args...); code != exitOK || !strings.Contains(out, "Usage") {
			t.Fatalf("%v = %d", args, code)
		}
	}
	if code, _, stderr := w.run("dance"); code != exitUsage || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("dance = %d", code)
	}
	for _, args := range [][]string{{"status", "--bogus"}, {"status", "extra"}, {"collect", "extra"}} {
		if code, _, _ := w.run(args...); code != exitUsage {
			t.Fatalf("%v = %d", args, code)
		}
	}
}
