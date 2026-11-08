//go:build acceptance

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSubscriptionsAreAddedListedAndRemoved(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": [{"id": "minimax", "source": "minimax-api"}]}`)
	if code, out, stderr := w.run("subscriptions", "add", "minimax-work", "--source", "minimax-api"); code != exitOK || !strings.Contains(out, "ataecina keys set minimax-work") {
		t.Fatalf("add = %d %q %q", code, out, stderr)
	}
	if code, _, stderr := w.run("subscriptions", "add", "xai", "--source", "xai-api", "--set", "team=team-1", "--key", "grok"); code != exitOK {
		t.Fatalf("add with an option = %d %q", code, stderr)
	}
	if code, _, stderr := w.run("subscriptions", "add", "minimax", "--source", "minimax-api"); code != exitFailure || !strings.Contains(stderr, "appears twice") {
		t.Fatalf("a duplicate = %d %q", code, stderr)
	}
	if code, _, _ := w.run("subscriptions", "add", "--source", "minimax-api"); code != exitUsage {
		t.Fatalf("no id = %d", code)
	}
	code, out, _ := w.run("subscriptions", "--json")
	var listed struct {
		Data []struct {
			ID      string         `json:"id"`
			Label   string         `json:"label"`
			Key     *string        `json:"key"`
			Options map[string]any `json:"options"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &listed); err != nil || code != exitOK || len(listed.Data) != 3 {
		t.Fatalf("list = %d %s %v", code, out, err)
	}
	if xai := listed.Data[2]; xai.Label != "XAI_API" || *xai.Key != "grok" || xai.Options["team"] != "team-1" {
		t.Fatalf("xai = %+v", xai)
	}
	if code, out, _ := w.run("subscriptions", "remove", "minimax-work"); code != exitOK || !strings.Contains(out, "removed minimax-work") {
		t.Fatalf("remove = %d %q", code, out)
	}
	if code, _, _ := w.run("subscriptions", "remove", "minimax-work"); code != exitFailure {
		t.Fatalf("removing twice = %d", code)
	}
}

func TestSourcesSayWhatEachNeeds(t *testing.T) {
	w := newWorld(t, `{"version": 1, "subscriptions": []}`)
	code, out, _ := w.run("sources", "--json")
	var sources struct {
		Data []struct {
			ID       string   `json:"id"`
			Label    string   `json:"label"`
			NeedsKey bool     `json:"needs_key"`
			Required []string `json:"required"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &sources); err != nil || code != exitOK {
		t.Fatalf("sources = %d %v", code, err)
	}
	found := map[string][]string{}
	for _, source := range sources.Data {
		found[source.ID] = source.Required
		if source.ID == "cursor-admin-api" && (!source.NeedsKey || source.Label != "CURSOR_ADMIN_API") {
			t.Fatalf("cursor = %+v", source)
		}
	}
	if strings.Join(found["heimdall"], ",") != "provider" || strings.Join(found["copilot-gh"], ",") != "login,monthly_allowance" {
		t.Fatalf("required = %v", found)
	}
}
