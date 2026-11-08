// Command widget-translations writes the Plasma widget's translation table
// (contents/ui/translations.js) from its PO files. Run it from the
// repository root after changing a translation; tests/widget checks that
// the table is up to date.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/simplicer/ataecina/internal/widgetcatalog"
)

const widget = "share/plasma/plasmoids/org.simplicer.ataecina"

func main() {
	table, err := widgetcatalog.Build(filepath.Join(widget, "translate"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "widget-translations:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(widget, "contents", "ui", "translations.js"), table, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "widget-translations:", err)
		os.Exit(1)
	}
}
