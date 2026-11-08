package desktopnotify_test

import (
	"testing"
	"time"

	"codeberg.org/simplicer/ataecina/internal/alerts/domain"
	"codeberg.org/simplicer/ataecina/internal/alerts/infrastructure/desktopnotify"
	"codeberg.org/simplicer/ataecina/internal/platform/locale"
)

func TestNotificationsSpeakTheSessionLanguage(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	alert := domain.Alert{
		Instance:    domain.WindowInstance{Subscription: "zai", Window: "FIVE_HOURS", ResetsAt: now.Add(26*time.Hour + 5*time.Minute)},
		Threshold:   90,
		UsedPercent: 91.6,
	}
	for language, want := range map[locale.Language][2]string{
		locale.English:  {"zai · 5 hours at 92%", "Above 90% of this window. Reborn in 1 d 2 h."},
		locale.Spanish:  {"zai · 5 horas al 92 %", "Por encima del 90 % de esta ventana. Renace en 1 d 2 h."},
		locale.Catalan:  {"zai · 5 hores al 92 %", "Per sobre del 90 % d'aquesta finestra. Renaix d'aquí a 1 d 2 h."},
		locale.Basque:   {"zai · 5 ordu · %92", "Leiho honen %90 gainditu da. 1 e 2 h barru berpiztuko da."},
		locale.Galician: {"zai · 5 horas ao 92 %", "Por riba do 90 % desta ventá. Renace en 1 d 2 h."},
		"fr":            {"zai · 5 hours at 92%", "Above 90% of this window. Reborn in 1 d 2 h."},
	} {
		if summary, body := desktopnotify.Text(alert, now, language); summary != want[0] || body != want[1] {
			t.Fatalf("%s: %q / %q", language, summary, body)
		}
	}
	if summary, body := desktopnotify.FailureText(domain.SourceAlert{Subscription: "zen", Failure: "KEY_MISSING"}, locale.Spanish); summary != "zen · falta la clave" || body == "" {
		t.Fatalf("failure = %q / %q", summary, body)
	}
	soon := domain.Alert{Instance: domain.WindowInstance{Subscription: "s", Window: "WEEKLY", ResetsAt: now.Add(45 * time.Minute)}, Threshold: 95, UsedPercent: 95}
	if _, body := desktopnotify.Text(soon, now, locale.Galician); body != "Por riba do 95 % desta ventá. Renace en 45 min." {
		t.Fatalf("minutes = %q", body)
	}
	hours := domain.Alert{Instance: domain.WindowInstance{Subscription: "s", Window: "SOMETHING", ResetsAt: now.Add(2*time.Hour + 10*time.Minute)}, Threshold: 95, UsedPercent: 95}
	if summary, body := desktopnotify.Text(hours, now, locale.English); summary != "s · window at 95%" || body != "Above 95% of this window. Reborn in 2 h 10 min." {
		t.Fatalf("hours = %q / %q", summary, body)
	}
}
