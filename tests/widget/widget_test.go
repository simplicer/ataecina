// Package widget checks the Plasma widget statically: no QML runtime is
// needed in CI (ADR 0007). It keeps the package metadata in step with
// VERSION, the configuration schema in step with the QML, every
// user-visible string translated, and every field the widget reads inside
// the public `status --json` contract (SPECS §8.2).
package widget

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/statusjson"
)

const (
	repository = "../.."
	plasmoid   = repository + "/share/plasma/plasmoids/org.simplicer.ataecina"
)

var (
	settingRead    = regexp.MustCompile(`configuration\.(\w+)`)
	settingEdited  = regexp.MustCompile(`property alias cfg_(\w+)`)
	settingEntry   = regexp.MustCompile(`entry name="(\w+)"`)
	translatable   = regexp.MustCompile(`\bi18np?\("((?:[^"\\]|\\.)*)"`)
	importLine     = regexp.MustCompile(`(?m)^import .*$`)
	declaredName   = regexp.MustCompile(`(?:function|property \w+|property alias|id:|const|let)\s+(\w+)`)
	parameterList  = regexp.MustCompile(`function\s*\w*\s*\(([^)]*)\)`)
	memberAccessed = regexp.MustCompile(`\b[a-z]\w*\.([a-z][a-z0-9_]*)\b`)
)

func read(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// qmlFiles returns every QML file of the widget, keyed by its path.
func qmlFiles(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(plasmoid + "/contents/*/*.qml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no QML files: %v", err)
	}
	files := make(map[string]string, len(paths))
	for _, path := range paths {
		files[path] = read(t, path)
	}
	return files
}

// code blanks comments and the content of string literals, keeping their
// quotes, so that the code around them can be scanned.
func code(source string) string {
	var out strings.Builder
	for i := 0; i < len(source); i++ {
		switch {
		case strings.HasPrefix(source[i:], "//"):
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				return out.String()
			}
			i += end - 1
		case strings.HasPrefix(source[i:], "/*"):
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				return out.String()
			}
			i += end + 3
		case source[i] == '"' || source[i] == '\'' || source[i] == '`':
			quote := source[i]
			out.WriteByte(quote)
			for i++; i < len(source) && source[i] != quote; i++ {
				if source[i] == '\\' {
					i++
				}
			}
			out.WriteByte(quote)
		default:
			out.WriteByte(source[i])
		}
	}
	return out.String()
}

func TestTheWidgetPackageIsInStepWithTheProject(t *testing.T) {
	var metadata struct {
		Structure string `json:"KPackageStructure"`
		Plugin    struct {
			ID      string `json:"Id"`
			Name    string `json:"Name"`
			Icon    string `json:"Icon"`
			Version string `json:"Version"`
		} `json:"KPlugin"`
		MinimumAPI string `json:"X-Plasma-API-Minimum-Version"`
		// The system tray offers the widget among its entries.
		NotificationArea         string `json:"X-Plasma-NotificationArea"`
		NotificationAreaCategory string `json:"X-Plasma-NotificationAreaCategory"`
	}
	if err := json.Unmarshal([]byte(read(t, plasmoid+"/metadata.json")), &metadata); err != nil {
		t.Fatal(err)
	}
	version := strings.TrimSpace(read(t, repository+"/VERSION"))
	switch {
	case metadata.Structure != "Plasma/Applet":
		t.Fatalf("KPackageStructure = %q", metadata.Structure)
	case metadata.Plugin.ID != "org.simplicer.ataecina" || metadata.Plugin.Name != "Ataecina":
		t.Fatalf("plugin = %q %q", metadata.Plugin.ID, metadata.Plugin.Name)
	case metadata.Plugin.Version != version:
		t.Fatalf("KPlugin.Version = %q, VERSION = %q", metadata.Plugin.Version, version)
	case metadata.MinimumAPI != "6.0":
		t.Fatalf("X-Plasma-API-Minimum-Version = %q", metadata.MinimumAPI)
	case metadata.NotificationArea != "true" || metadata.NotificationAreaCategory != "SystemServices":
		t.Fatalf("the system tray cannot offer the widget: %q %q", metadata.NotificationArea, metadata.NotificationAreaCategory)
	}
	for _, path := range []string{
		plasmoid + "/contents/ui/main.qml",
		plasmoid + "/contents/config/main.xml",
		repository + "/share/icons/hicolor/scalable/apps/" + metadata.Plugin.Icon + ".svg",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTheConfigurationSchemaMatchesTheQML(t *testing.T) {
	var schema struct {
		Entries []struct {
			Name string `xml:"name,attr"`
		} `xml:"group>entry"`
	}
	if err := xml.Unmarshal([]byte(read(t, plasmoid+"/contents/config/main.xml")), &schema); err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{}
	for _, entry := range schema.Entries {
		declared[entry.Name] = true
	}
	isRead := map[string]bool{}
	isEdited := map[string]bool{}
	for path, source := range qmlFiles(t) {
		for _, match := range settingRead.FindAllStringSubmatch(code(source), -1) {
			if !declared[match[1]] {
				t.Errorf("%s reads undeclared setting %q", filepath.Base(path), match[1])
			}
			isRead[match[1]] = true
		}
		for _, match := range settingEdited.FindAllStringSubmatch(source, -1) {
			if !declared[match[1]] {
				t.Errorf("%s edits undeclared setting %q", filepath.Base(path), match[1])
			}
			isEdited[match[1]] = true
		}
	}
	for name := range declared {
		if !isRead[name] || !isEdited[name] {
			t.Errorf("setting %q is read %v and edited %v", name, isRead[name], isEdited[name])
		}
	}
}

func TestTheQMLIsBalanced(t *testing.T) {
	pairs := map[byte]byte{')': '(', ']': '[', '}': '{'}
	for path, source := range qmlFiles(t) {
		var open []byte
		for _, char := range []byte(code(source)) {
			switch char {
			case '(', '[', '{':
				open = append(open, char)
			case ')', ']', '}':
				if len(open) == 0 || open[len(open)-1] != pairs[char] {
					t.Fatalf("%s: unexpected %q", filepath.Base(path), char)
				}
				open = open[:len(open)-1]
			}
		}
		if len(open) != 0 {
			t.Fatalf("%s: %d unclosed brackets", filepath.Base(path), len(open))
		}
	}
}

var (
	poField        = regexp.MustCompile(`^(msgid|msgid_plural|msgstr(?:\[\d+\])?) "(.*)"$`)
	poContinuation = regexp.MustCompile(`^"(.*)"$`)
)

// fullMessages reads every msgid → msgstr pair of a PO file with their
// continuation lines; plural entries keep their first form.
func fullMessages(t *testing.T, path string) map[string]string {
	t.Helper()
	found := map[string]string{}
	fields, field := map[string]string{}, ""
	keep := func() {
		if id := fields["msgid"]; id != "" {
			if translation, ok := fields["msgstr"]; ok {
				found[id] = translation
			} else {
				found[id] = fields["msgstr[0]"]
			}
		}
		fields, field = map[string]string{}, ""
	}
	for _, line := range strings.Split(read(t, path), "\n") {
		if match := poField.FindStringSubmatch(line); match != nil {
			field = match[1]
			fields[field] = match[2]
			continue
		}
		if match := poContinuation.FindStringSubmatch(line); match != nil && field != "" {
			fields[field] += match[1]
			continue
		}
		if strings.TrimSpace(line) == "" {
			keep()
		}
	}
	keep()
	return found
}

// A translation merged with an older one (msgmerge's fuzzy matching) shows
// both texts joined in the widget.
func TestNoTranslationJoinsAnother(t *testing.T) {
	spanish := fullMessages(t, plasmoid+"/translate/es.po")
	for msgid, translation := range spanish {
		if len(translation) > 2*len(msgid)+20 {
			t.Errorf("es.po translates %q into something far longer: %q", msgid, translation)
		}
		for other, otherTranslation := range spanish {
			if other != msgid && len(otherTranslation) > 15 && strings.Contains(translation, otherTranslation) {
				t.Errorf("es.po's translation of %q holds the one of %q", msgid, other)
			}
		}
	}
}

func TestEveryStringIsInTheTemplateAndTranslated(t *testing.T) {
	template := fullMessages(t, plasmoid+"/translate/template.pot")
	spanish := fullMessages(t, plasmoid+"/translate/es.po")
	used := map[string]bool{}
	for path, source := range qmlFiles(t) {
		for _, match := range translatable.FindAllStringSubmatch(source, -1) {
			used[match[1]] = true
			if _, ok := template[match[1]]; !ok {
				t.Errorf("%s: %q is missing from template.pot; regenerate it (README)", filepath.Base(path), match[1])
			}
		}
	}
	for msgid := range template {
		if !used[msgid] {
			t.Errorf("template.pot has %q, which no QML uses", msgid)
		}
		if spanish[msgid] == "" {
			t.Errorf("es.po does not translate %q", msgid)
		}
	}
}

// contractKeys renders a report that fills every optional field and
// returns every property name of the resulting document.
func contractKeys(t *testing.T) map[string]bool {
	t.Helper()
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	usd, _ := domain.NewCurrency("USD")
	amount, _ := domain.ParseAmount("4.20")
	money := domain.NewMoney(amount, usd)
	balance, _ := domain.NewBalance(&money, &money)
	credits := domain.NewCredits(amount, false)
	banked, _ := domain.NewBankedResets(1)
	window, _ := domain.OtherWindow(90)
	reading, err := domain.NewReading(domain.ReadingFacts{
		Subscription: "example", Provider: "example", Source: "example-api", Plan: "PLUS",
		Fidelity: domain.FidelityReported, Outcome: domain.OutcomePartial, ObservedAt: at,
		Meters:  []domain.Meter{domain.NewMeter(domain.NewBucketID("main"), window, 50, at.Add(time.Hour))},
		Balance: &balance, Credits: &credits, BankedResets: &banked,
		ErrorClass: domain.ErrorUnavailable, Hint: "hint",
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := domain.NewSnapshot(at, 5*time.Minute, []domain.Reading{reading})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	model := statusjson.Model{ID: "m", Name: "M", ReleasedOn: at}
	catalogue := map[string]statusjson.Offerings{"example": {
		Catalogue: "example", Name: "Example", Models: 2, FreeModels: []statusjson.Model{model}, NewModels: []statusjson.Model{model},
	}}
	if err := statusjson.Write(&out, snapshot.StatusAt(at), statusjson.Meta{Version: "0.0.0", GeneratedAt: at, Offerings: catalogue, OfferingsRefreshedAt: at}); err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	var walk func(value any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, inner := range value {
				keys[key] = true
				walk(inner)
			}
		case []any:
			for _, inner := range value {
				walk(inner)
			}
		}
	}
	walk(document)
	return keys
}

// Members of JavaScript and QML types the widget uses; anything else in
// snake_case or lower case after a dot must be a contract field, a
// setting, or a name the widget declares itself.
var platformMembers = []string{
	"every", "filter", "find", "floor", "join", "length", "map", "max", "min",
	"name", "parse", "push", "restart", "round", "split", "trim", "weight",
	"view", "width", "height", "button", "expanded", "icon", "configuration",
	"text", "value", "checked", "r", "g", "b", "fill", "margins",
	"arc", "reset", "stroke", "slice", "visible", "small", "policy",
}

// cliMembers are the properties of the other JSON documents the settings
// read: `ataecina sources --json` and `ataecina subscriptions --json`
// (SPECS §8.5).
var cliMembers = []string{
	"id", "label", "provider", "fidelity", "needs_key", "options", "required",
	"takes_declarations", "source", "plan", "key",
}

func TestTheWidgetReadsOnlyTheStatusContract(t *testing.T) {
	known := contractKeys(t)
	for _, member := range append(platformMembers, cliMembers...) {
		known[member] = true
	}
	files := qmlFiles(t)
	for _, source := range files {
		for _, match := range declaredName.FindAllStringSubmatch(source, -1) {
			known[match[1]] = true
		}
		for _, match := range parameterList.FindAllStringSubmatch(source, -1) {
			for _, name := range strings.Split(match[1], ",") {
				known[strings.TrimSpace(name)] = true
			}
		}
	}
	for _, match := range settingEntry.FindAllStringSubmatch(read(t, plasmoid+"/contents/config/main.xml"), -1) {
		known[match[1]] = true
	}
	var unknown []string
	for path, source := range files {
		for _, match := range memberAccessed.FindAllStringSubmatch(code(importLine.ReplaceAllString(source, "")), -1) {
			if !known[match[1]] {
				unknown = append(unknown, filepath.Base(path)+": "+match[0])
			}
		}
	}
	slices.Sort(unknown)
	if len(unknown) > 0 {
		t.Fatalf("the widget reads names outside the status contract:\n%s", strings.Join(slices.Compact(unknown), "\n"))
	}
}
