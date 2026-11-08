// Package locale picks the language of the owner's session for the few
// texts Ataecina writes itself, such as desktop notifications. It follows
// gettext: the locale is LC_ALL, else LC_MESSAGES, else LANG; a C or POSIX
// locale is English; otherwise the first supported language of LANGUAGE,
// then of the locale, wins. Supported: Spanish, Catalan, Basque, Galician
// and English (the default).
package locale

import "strings"

// Language is a supported language code.
type Language string

// Supported languages.
const (
	English  Language = "en"
	Spanish  Language = "es"
	Catalan  Language = "ca"
	Basque   Language = "eu"
	Galician Language = "gl"
)

var supported = map[string]Language{"en": English, "es": Spanish, "ca": Catalan, "eu": Basque, "gl": Galician}

// FromEnvironment reads the session's language through getenv.
func FromEnvironment(getenv func(string) string) Language {
	base := ""
	for _, variable := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := getenv(variable); value != "" {
			base = value
			break
		}
	}
	if base == "" || base == "C" || base == "POSIX" || strings.HasPrefix(base, "C.") {
		return English
	}
	for _, candidate := range append(strings.Split(getenv("LANGUAGE"), ":"), base) {
		if language, ok := supported[code(candidate)]; ok {
			return language
		}
	}
	return English
}

// code is the language part of "ca_ES.UTF-8@valencia".
func code(locale string) string {
	end := strings.IndexAny(locale, "_.@")
	if end >= 0 {
		locale = locale[:end]
	}
	return strings.ToLower(locale)
}
