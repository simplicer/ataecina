// Package opencodezen derives the OpenCode Zen balance: the owner's
// declared top-ups minus the cost OpenCode itself records for its
// `opencode` provider, read with `opencode stats` (SPECS §3).
package opencodezen

import (
	"bytes"
	"encoding/json"
	"errors"

	"codeberg.org/simplicer/ataecina/internal/allowances/domain"
)

// zenProvider is the providerID OpenCode gives to OpenCode Zen models.
const zenProvider = "opencode"

// errUnexpectedStats reports `opencode stats` output Ataecina cannot read.
var errUnexpectedStats = errors.New("unexpected opencode stats output")

// DecodeSpend sums the cost of every model entry whose
// model.providerID is "opencode" in `opencode stats --models --json`
// output. Entries are found wherever they sit in the document; an output
// with no model entry and no "models" key at all is unexpected.
func DecodeSpend(output []byte) (domain.Amount, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return domain.Amount{}, errUnexpectedStats
	}
	tally := spendTally{}
	if err := tally.walk(document); err != nil {
		return domain.Amount{}, err
	}
	if tally.entries == 0 && !tally.sawModels {
		return domain.Amount{}, errUnexpectedStats
	}
	return tally.zen, nil
}

type spendTally struct {
	zen       domain.Amount
	entries   int
	sawModels bool
}

func (t *spendTally) walk(node any) error {
	switch value := node.(type) {
	case []any:
		for _, item := range value {
			if err := t.walk(item); err != nil {
				return err
			}
		}
	case map[string]any:
		if _, ok := value["models"]; ok {
			t.sawModels = true
		}
		if err := t.count(value); err != nil {
			return err
		}
		for _, child := range value {
			if err := t.walk(child); err != nil {
				return err
			}
		}
	}
	return nil
}

// count adds one model entry: {"model": {"providerID": …}, "cost": …}.
func (t *spendTally) count(entry map[string]any) error {
	model, ok := entry["model"].(map[string]any)
	if !ok {
		return nil
	}
	provider, ok := model["providerID"].(string)
	if !ok {
		return nil
	}
	cost, ok := entry["cost"].(json.Number)
	if !ok {
		return errUnexpectedStats
	}
	amount, err := domain.ParseAmount(cost.String())
	if err != nil || amount.IsNegative() {
		return errUnexpectedStats
	}
	t.entries++
	if provider == zenProvider {
		t.zen = t.zen.Add(amount)
		if t.zen.IsNegative() {
			return errUnexpectedStats // the sum overflowed: the output is junk
		}
	}
	return nil
}
