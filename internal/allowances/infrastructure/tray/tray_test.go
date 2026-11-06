package tray

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

var taken = time.Date(2026, 11, 6, 19, 40, 0, 0, time.UTC)

func report(t *testing.T, used domain.UsedPercent) domain.StatusReport {
	t.Helper()
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "claude", Provider: "anthropic", Source: "claude-statusline", Plan: "PRO",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomeOK, ObservedAt: taken,
		Meters: []domain.Meter{
			domain.NewMeter("", domain.FiveHourWindow(), used, taken.Add(130*time.Minute)),
			domain.NewMeter("spend_limit", domain.BillingCycleWindow(), 12, time.Time{}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := domain.NewSnapshot(taken, 5*time.Minute, []domain.Reading{reading})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot.StatusAt(taken)
}

func labels(entries []entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.separator {
			out = append(out, "---")
			continue
		}
		out = append(out, e.label)
	}
	return out
}

func TestTheMenuListsEverySubscriptionThenTheActions(t *testing.T) {
	entries, lines := New(Options{}).describe(report(t, 23), nil, taken.Add(time.Minute))
	got := strings.Join(labels(entries), "\n")
	want := strings.Join([]string{
		"claude · PRO · 5h 23% ↻ 2h09m · spend__limit cycle 12% · fresh 1m ago",
		"---",
		"Ataecina · updated 1m ago",
		"Refresh now",
		"Quit",
	}, "\n")
	if got != want {
		t.Fatalf("menu:\n%s\nwant:\n%s", got, want)
	}
	if lines[0] != "Ataecina · updated 1m ago" || !strings.Contains(lines[1], "spend_limit") {
		t.Fatalf("tooltip lines = %q", lines)
	}
}

func TestTheMenuExplainsAMissingOrUnreadableSnapshot(t *testing.T) {
	tray := New(Options{})
	entries, _ := tray.describe(domain.StatusReport{}, nil, taken)
	if entries[0].label != "No snapshot yet. Run `ataecina collect`, or enable ataecina-collect.timer." || entries[0].enabled {
		t.Fatalf("no snapshot: %+v", entries[0])
	}
	tray.problem = "Refresh failed: no systemd"
	entries, lines := tray.describe(domain.StatusReport{}, errors.New("loose permissions"), taken)
	if !strings.HasPrefix(entries[0].label, "The snapshot cannot be read: loose permissions") || entries[1].label != "Refresh failed: no systemd" {
		t.Fatalf("problems: %q", labels(entries))
	}
	if len(lines) != 2 || strings.Contains(strings.Join(labels(entries), ""), "updated") {
		t.Fatalf("no age without a snapshot: %q %q", labels(entries), lines)
	}
}

func TestAnEmptySnapshotSaysSo(t *testing.T) {
	snapshot, err := domain.NewSnapshot(taken, 5*time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	entries, lines := New(Options{}).describe(snapshot.StatusAt(taken), nil, taken)
	if entries[0].label != "The snapshot lists no subscriptions." || lines[0] != "Ataecina · updated just now" {
		t.Fatalf("empty snapshot: %q %q", labels(entries), lines)
	}
}

func TestTheMenuRevisionChangesOnlyWithItsEntries(t *testing.T) {
	m := &menu{}
	var announced []uint32
	m.layoutUpdated = func(revision uint32) { announced = append(announced, revision) }
	first := []entry{{id: refreshID, label: "Refresh now", enabled: true}}
	m.show(first)
	m.show([]entry{{id: refreshID, label: "Refresh now", enabled: true}})
	m.show(append(first, entry{id: quitID, label: "Quit", enabled: true}))
	if len(announced) != 2 || announced[1] != 2 {
		t.Fatalf("announced = %v", announced)
	}
	revision, root, err := m.GetLayout(rootID, -1, nil)
	if err != nil || revision != 2 || len(root.Children) != 2 {
		t.Fatalf("GetLayout = %d %+v %v", revision, root, err)
	}
	if _, single, err := m.GetLayout(quitID, 0, nil); err != nil || single.ID != quitID {
		t.Fatalf("GetLayout(quit) = %+v %v", single, err)
	}
	if _, _, err := m.GetLayout(42, 0, nil); err == nil {
		t.Fatal("an unknown parent is an error")
	}
	if label, err := m.GetProperty(quitID, "label"); err != nil || label.Value() != "Quit" {
		t.Fatalf("GetProperty = %v %v", label, err)
	}
	if _, err := m.GetProperty(quitID, "icon-data"); err == nil {
		t.Fatal("an unknown property is an error")
	}
	if _, err := m.GetProperty(42, "label"); err == nil {
		t.Fatal("an unknown item is an error")
	}
	if group, _ := m.GetGroupProperties([]int32{refreshID, 42}, nil); len(group) != 1 {
		t.Fatalf("GetGroupProperties = %+v", group)
	}
	separator := entry{id: separatorID, separator: true}.properties()
	if separator["type"].Value() != "separator" {
		t.Fatalf("separator = %v", separator)
	}
}

func TestOnlyClicksOnActionsAct(t *testing.T) {
	actions := make(chan action, 1)
	m := &menu{actions: actions}
	_ = m.Event(firstLineID, "clicked", dbus.MakeVariant(""), 0)
	_ = m.Event(refreshID, "hovered", dbus.MakeVariant(""), 0)
	if len(actions) != 0 {
		t.Fatal("a subscription line or a hover must not act")
	}
	if failed, _ := m.EventGroup([]event{{ID: refreshID, EventID: "clicked"}, {ID: quitID, EventID: "clicked"}}); len(failed) != 0 {
		t.Fatalf("EventGroup = %v", failed)
	}
	if got := <-actions; got != refreshAction || len(actions) != 0 {
		t.Fatalf("the first click wins while the loop is busy: %v", got)
	}
	if update, _ := m.AboutToShow(rootID); update {
		t.Fatal("AboutToShow needs no update")
	}
	if updates, failed, _ := m.AboutToShowGroup([]int32{rootID}); len(updates)+len(failed) != 0 {
		t.Fatal("AboutToShowGroup needs no update")
	}
	it := item{actions: actions}
	_ = it.Activate(0, 0)
	_ = it.ContextMenu(0, 0)
	_ = it.Scroll(1, "vertical")
	if len(actions) != 0 {
		t.Fatal("only a middle click acts on the item")
	}
	_ = it.SecondaryActivate(0, 0)
	if got := <-actions; got != refreshAction {
		t.Fatalf("middle click = %v", got)
	}
}

func TestTextIsSafeForMenusAndTooltips(t *testing.T) {
	if got := menuLabel("spend_limit"); got != "spend__limit" {
		t.Fatalf("menuLabel = %q", got)
	}
	if got := escapeMarkup("a < b & c > d"); got != "a &lt; b &amp; c &gt; d" {
		t.Fatalf("escapeMarkup = %q", got)
	}
}
