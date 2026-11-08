package configfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/simplicer/ataecina/internal/allowances/infrastructure/catalog"
	"codeberg.org/simplicer/ataecina/internal/platform/privatefs"
)

// NewSubscription is a subscription the owner adds: its id, its source and
// the options config.json writes for it, as text (numbers and switches are
// written as such).
type NewSubscription struct {
	ID      string
	Source  string
	Plan    string
	Key     string
	Options map[string]string
}

var (
	numberOptions = map[string]bool{"monthly_allowance": true, "port": true, "app_server_after_minutes": true}
	switchOptions = map[string]bool{"app_server": true}
	topLevelOrder = []string{"version", "interval_minutes", "alerts", "network", "offers", "language", "subscriptions"}
)

// ErrNoSuchSubscription is a subscription id config.json does not have.
var ErrNoSuchSubscription = errors.New("no subscription with that id")

// ErrSubscriptionExists is a subscription id config.json already has.
var ErrSubscriptionExists = errors.New("a subscription with that id already exists")

// AddSubscription appends a subscription to config.json. The file is
// replaced only when the whole result passes the same validation as Load.
func AddSubscription(path string, sources catalog.Catalog, item NewSubscription) error {
	entry, err := subscriptionObject(item)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return edit(path, sources, func(subscriptions []json.RawMessage) ([]json.RawMessage, error) {
		if slices.ContainsFunc(subscriptions, func(raw json.RawMessage) bool { return idOf(raw) == item.ID }) {
			return nil, fmt.Errorf("%w: %q", ErrSubscriptionExists, item.ID)
		}
		return append(subscriptions, entry), nil
	})
}

// RemoveSubscription drops one subscription from config.json; its sealed
// key, if any, stays until `ataecina keys forget`.
func RemoveSubscription(path string, sources catalog.Catalog, id string) error {
	return edit(path, sources, func(subscriptions []json.RawMessage) ([]json.RawMessage, error) {
		kept := make([]json.RawMessage, 0, len(subscriptions))
		for _, raw := range subscriptions {
			if idOf(raw) != id {
				kept = append(kept, raw)
			}
		}
		if len(kept) == len(subscriptions) {
			return nil, fmt.Errorf("%w: %q", ErrNoSuchSubscription, id)
		}
		return kept, nil
	})
}

// subscriptionObject writes id, source, plan, key and the options in that
// order.
func subscriptionObject(item NewSubscription) (json.RawMessage, error) {
	var out bytes.Buffer
	out.WriteString("{")
	write := func(name string, value any) {
		if out.Len() > 1 {
			out.WriteString(",")
		}
		encoded, _ := json.Marshal(name)
		out.Write(encoded)
		out.WriteString(":")
		encoded, _ = json.Marshal(value)
		out.Write(encoded)
	}
	write("id", item.ID)
	write("source", item.Source)
	if item.Plan != "" {
		write("plan", item.Plan)
	}
	if item.Key != "" {
		write("key", item.Key)
	}
	names := make([]string, 0, len(item.Options))
	for name := range item.Options {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		value := item.Options[name]
		switch {
		case numberOptions[name]:
			number, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("%q must be a number", name)
			}
			write(name, number)
		case switchOptions[name]:
			on, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("%q must be true or false", name)
			}
			write(name, on)
		default:
			write(name, value)
		}
	}
	out.WriteString("}")
	return out.Bytes(), nil
}

// edit changes the subscriptions of config.json and writes the file back,
// atomically and 0600, one subscription per line, only if it validates.
// idOf is a subscription object's id, empty when it has none.
func idOf(raw json.RawMessage) string {
	var named struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &named)
	return named.ID
}

func edit(path string, sources catalog.Catalog, change func([]json.RawMessage) ([]json.RawMessage, error)) error {
	if err := privatefs.RequireDir(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := privatefs.ReadFile(path, maxBytes)
	if err != nil {
		return err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	var subscriptions []json.RawMessage
	if raw, ok := top["subscriptions"]; ok {
		if err := json.Unmarshal(raw, &subscriptions); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalid, err)
		}
	}
	if subscriptions, err = change(subscriptions); err != nil {
		return err
	}
	rendered, err := render(top, subscriptions)
	if err != nil {
		return err
	}
	if _, err := Decode(rendered, sources); err != nil {
		return err
	}
	return privatefs.WriteFile(path, rendered)
}

// render writes the known top-level keys in their usual order, then any
// other, each value on one line, and one subscription per line.
func render(top map[string]json.RawMessage, subscriptions []json.RawMessage) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("{\n")
	names := append([]string(nil), topLevelOrder...)
	var others []string
	for name := range top {
		if !slices.Contains(topLevelOrder, name) {
			others = append(others, name)
		}
	}
	slices.Sort(others)
	names = append(names, others...)
	first := true
	for _, name := range names {
		raw, ok := top[name]
		if !ok && name != "subscriptions" {
			continue
		}
		if !first {
			out.WriteString(",\n")
		}
		first = false
		key, _ := json.Marshal(name)
		out.WriteString("  ")
		out.Write(key)
		out.WriteString(": ")
		if name == "subscriptions" {
			out.WriteString("[")
			for i, item := range subscriptions {
				line, err := oneLine(item)
				if err != nil {
					return nil, err
				}
				if i > 0 {
					out.WriteString(",")
				}
				out.WriteString("\n    ")
				out.WriteString(line)
			}
			out.WriteString("\n  ]")
			continue
		}
		line, err := oneLine(raw)
		if err != nil {
			return nil, err
		}
		out.WriteString(line)
	}
	out.WriteString("\n}\n")
	return out.Bytes(), nil
}

// oneLine writes a JSON value on one line, keeping the key order:
// `{ "id": "zai", "thresholds": [90, 95] }`.
func oneLine(raw json.RawMessage) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var out strings.Builder
	if err := writeValue(decoder, &out); err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return out.String(), nil
}

func writeValue(decoder *json.Decoder, out *strings.Builder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch value := token.(type) {
	case json.Delim:
		closing, open := "]", value == '['
		if value == '{' {
			closing = " }"
			out.WriteString("{ ")
		} else {
			out.WriteString("[")
		}
		for i := 0; decoder.More(); i++ {
			if i > 0 {
				out.WriteString(", ")
			}
			if !open {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				encoded, _ := json.Marshal(key)
				out.Write(encoded)
				out.WriteString(": ")
			}
			if err := writeValue(decoder, out); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if closing == " }" && strings.HasSuffix(out.String(), "{ ") {
			trimmed := strings.TrimSuffix(out.String(), " ")
			out.Reset()
			out.WriteString(trimmed)
			closing = "}"
		}
		out.WriteString(closing)
	case json.Number:
		out.WriteString(value.String())
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		out.Write(encoded)
	}
	return nil
}
