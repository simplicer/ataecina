//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

// fixtureNow is the instant the synthetic fixtures of the adapters are
// written for (providerhttptest.Now). The end-to-end tests run on the real
// clock, so fixture moves every instant by the time elapsed since then:
// a reset "two hours after the reading" stays two hours ahead, whatever
// the day the suite runs.
var fixtureNow = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

var (
	epochMillis  = regexp.MustCompile(`\b1[789]\d{11}\b`)
	epochSeconds = regexp.MustCompile(`\b1[789]\d{8}\b`)
	rfc3339      = regexp.MustCompile(`\b20\d\d-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z`)
)

// fixture returns an adapter's synthetic fixture with its instants moved
// to the present.
func fixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "allowances", "infrastructure", path))
	if err != nil {
		t.Fatal(err)
	}
	return shiftInstants(string(data), time.Since(fixtureNow).Truncate(time.Second))
}

func shiftInstants(body string, delta time.Duration) string {
	body = epochMillis.ReplaceAllStringFunc(body, func(value string) string {
		millis, _ := strconv.ParseInt(value, 10, 64)
		return strconv.FormatInt(millis+delta.Milliseconds(), 10)
	})
	body = epochSeconds.ReplaceAllStringFunc(body, func(value string) string {
		seconds, _ := strconv.ParseInt(value, 10, 64)
		return strconv.FormatInt(seconds+int64(delta/time.Second), 10)
	})
	return rfc3339.ReplaceAllStringFunc(body, func(value string) string {
		at, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return value
		}
		return at.Add(delta).UTC().Format(time.RFC3339Nano)
	})
}

func TestFixtureInstantsMoveTogether(t *testing.T) {
	got := shiftInstants(`{"ms": 1794002400000, "s": 1798675200, "at": "2026-11-07T00:00:00Z", "amount": 8.42}`, 36*time.Hour)
	want := `{"ms": 1794132000000, "s": 1798804800, "at": "2026-11-08T12:00:00Z", "amount": 8.42}`
	if got != want {
		t.Fatalf("shiftInstants = %s", got)
	}
}
