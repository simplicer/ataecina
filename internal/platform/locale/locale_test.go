package locale_test

import (
	"testing"

	"codeberg.org/simplicer/ataecina/internal/platform/locale"
)

func TestTheSessionLanguageIsFound(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want locale.Language
	}{
		{map[string]string{"LANG": "es_ES.UTF-8"}, locale.Spanish},
		{map[string]string{"LANG": "ca_ES.UTF-8@valencia"}, locale.Catalan},
		{map[string]string{"LANG": "eu_ES.UTF-8"}, locale.Basque},
		{map[string]string{"LANG": "gl_ES.UTF-8"}, locale.Galician},
		{map[string]string{"LANG": "en_GB.UTF-8"}, locale.English},
		{map[string]string{"LANG": "fr_FR.UTF-8"}, locale.English},
		{map[string]string{"LANG": "es_ES.UTF-8", "LANGUAGE": "fr:gl:es"}, locale.Galician},
		{map[string]string{"LANG": "es_ES.UTF-8", "LC_MESSAGES": "eu_ES.UTF-8"}, locale.Basque},
		{map[string]string{"LANG": "es_ES.UTF-8", "LC_ALL": "C"}, locale.English},
		{map[string]string{"LANG": "C.UTF-8", "LANGUAGE": "es"}, locale.English},
		{map[string]string{}, locale.English},
	} {
		if got := locale.FromEnvironment(func(name string) string { return c.env[name] }); got != c.want {
			t.Fatalf("%v: %s, want %s", c.env, got, c.want)
		}
	}
}
