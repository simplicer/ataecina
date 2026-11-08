package desktopnotify

import (
	"fmt"
	"time"

	"codeberg.org/simplicer/ataecina/internal/platform/locale"
)

// wording is the text of the notifications in one language. Sources'
// hints are shown as they come (English).
type wording struct {
	windows    map[string]string
	failures   map[string]string
	summary    string // subscription, window, percentage
	above      string // threshold percentage
	reborn     string // time to rebirth
	unreadable string
	percent    string // a number with its percent sign
	days       string
	hours      string
	minutes    string
}

var wordings = map[locale.Language]wording{
	locale.English: {
		windows:    map[string]string{"FIVE_HOURS": "5 hours", "DAILY": "day", "WEEKLY": "week", "MONTHLY": "month", "BILLING_CYCLE": "billing cycle", "OTHER": "window"},
		failures:   map[string]string{"KEY_MISSING": "key missing", "BROKEN": "broken", "UNAVAILABLE": "unavailable", "LOCKED": "locked", "UNSUPPORTED": "unsupported", "PARTIAL": "partial"},
		summary:    "%s · %s at %s",
		above:      "Above %s of this window.",
		reborn:     "Reborn in %s.",
		unreadable: "Ataecina cannot read this subscription until it is fixed.",
		percent:    "%d%%", days: "%d d", hours: "%d h", minutes: "%d min",
	},
	locale.Spanish: {
		windows:    map[string]string{"FIVE_HOURS": "5 horas", "DAILY": "día", "WEEKLY": "semana", "MONTHLY": "mes", "BILLING_CYCLE": "ciclo de facturación", "OTHER": "ventana"},
		failures:   map[string]string{"KEY_MISSING": "falta la clave", "BROKEN": "rota", "UNAVAILABLE": "no disponible", "LOCKED": "bloqueada", "UNSUPPORTED": "no soportada", "PARTIAL": "parcial"},
		summary:    "%s · %s al %s",
		above:      "Por encima del %s de esta ventana.",
		reborn:     "Renace en %s.",
		unreadable: "Ataecina no puede leer esta suscripción hasta que se arregle.",
		percent:    "%d %%", days: "%d d", hours: "%d h", minutes: "%d min",
	},
	locale.Catalan: {
		windows:    map[string]string{"FIVE_HOURS": "5 hores", "DAILY": "dia", "WEEKLY": "setmana", "MONTHLY": "mes", "BILLING_CYCLE": "cicle de facturació", "OTHER": "finestra"},
		failures:   map[string]string{"KEY_MISSING": "falta la clau", "BROKEN": "trencada", "UNAVAILABLE": "no disponible", "LOCKED": "bloquejada", "UNSUPPORTED": "no compatible", "PARTIAL": "parcial"},
		summary:    "%s · %s al %s",
		above:      "Per sobre del %s d'aquesta finestra.",
		reborn:     "Renaix d'aquí a %s.",
		unreadable: "Ataecina no pot llegir aquesta subscripció fins que s'arregli.",
		percent:    "%d %%", days: "%d d", hours: "%d h", minutes: "%d min",
	},
	locale.Basque: {
		windows:    map[string]string{"FIVE_HOURS": "5 ordu", "DAILY": "eguna", "WEEKLY": "astea", "MONTHLY": "hilabetea", "BILLING_CYCLE": "fakturazio-zikloa", "OTHER": "leihoa"},
		failures:   map[string]string{"KEY_MISSING": "gakoa falta da", "BROKEN": "hautsita", "UNAVAILABLE": "ez dago erabilgarri", "LOCKED": "blokeatuta", "UNSUPPORTED": "onartu gabea", "PARTIAL": "partziala"},
		summary:    "%s · %s · %s",
		above:      "Leiho honen %s gainditu da.",
		reborn:     "%s barru berpiztuko da.",
		unreadable: "Ataecinak ezin du harpidetza hau irakurri konpondu arte.",
		percent:    "%%%d", days: "%d e", hours: "%d h", minutes: "%d min",
	},
	locale.Galician: {
		windows:    map[string]string{"FIVE_HOURS": "5 horas", "DAILY": "día", "WEEKLY": "semana", "MONTHLY": "mes", "BILLING_CYCLE": "ciclo de facturación", "OTHER": "ventá"},
		failures:   map[string]string{"KEY_MISSING": "falta a chave", "BROKEN": "rota", "UNAVAILABLE": "non dispoñible", "LOCKED": "bloqueada", "UNSUPPORTED": "non compatible", "PARTIAL": "parcial"},
		summary:    "%s · %s ao %s",
		above:      "Por riba do %s desta ventá.",
		reborn:     "Renace en %s.",
		unreadable: "Ataecina non pode ler esta subscrición ata que se arranxe.",
		percent:    "%d %%", days: "%d d", hours: "%d h", minutes: "%d min",
	},
}

func wordingFor(language locale.Language) wording {
	if found, ok := wordings[language]; ok {
		return found
	}
	return wordings[locale.English]
}

func (w wording) window(kind string) string {
	if name, ok := w.windows[kind]; ok {
		return name
	}
	return w.windows["OTHER"]
}

func (w wording) failure(outcome string) string {
	if name, ok := w.failures[outcome]; ok {
		return name
	}
	return outcome
}

// duration is "1 d 3 h", "2 h 10 min" or "45 min".
func (w wording) duration(span time.Duration) string {
	minutes := int(span.Round(time.Minute) / time.Minute)
	days, hours := minutes/1440, minutes%1440/60
	switch {
	case days > 0:
		return fmt.Sprintf(w.days, days) + " " + fmt.Sprintf(w.hours, hours)
	case hours > 0:
		return fmt.Sprintf(w.hours, hours) + " " + fmt.Sprintf(w.minutes, minutes%60)
	}
	return fmt.Sprintf(w.minutes, minutes)
}
