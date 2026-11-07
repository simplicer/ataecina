/*
 * The snapshot as the widget sees it: what `ataecina status --json` said
 * (SPECS §8.2), the subscriptions to show and how to word and colour each
 * figure. No Plasma type here, so the popup can be drawn on its own.
 */
import QtQuick

QtObject {
    id: snapshot

    // Default alert thresholds (SPECS §8.3), in percent used.
    readonly property int warningPercent: 90
    readonly property int criticalPercent: 95

    // Ataecina's palette: growth rings from the outer, longest window in;
    // amber and coral when little is left.
    readonly property var growth: ["#2e8b74", "#4caf50", "#9ccc65"]
    readonly property color lowColor: "#e0a13a"
    readonly property color criticalColor: "#e2604a"

    property var subscriptions: []
    property var meta: ({})
    property string problem: ""
    property date now: new Date()
    // Comma-separated subscription ids, in order; empty shows all.
    property string wanted: ""

    readonly property var sourceNames: ({
            "CLAUDE_STATUSLINE": "Claude",
            "CODEX_LOGS": "Codex",
            "CODEX_APP_SERVER": "Codex",
            "ZAI_API": "Z.ai",
            "MINIMAX_API": "MiniMax",
            "COMMANDCODE_API": "Command Code",
            "OPENROUTER_API": "OpenRouter",
            "VERCEL_API": "Vercel AI Gateway",
            "OLLAMA_API": "Ollama Cloud",
            "ORCAROUTER_API": "Orca Router",
            "CLOUDFLARE_API": "Workers AI",
            "OPENCODE_ZEN": "OpenCode Zen",
            "COPILOT_GH": "Copilot",
            "ALIBABA_CLI": "Alibaba Model Studio"
        })

    readonly property var currencySymbols: ({
            "USD": "$",
            "EUR": "€",
            "GBP": "£",
            "CNY": "¥"
        })

    readonly property var shown: {
        const ids = wanted.split(",").map(s => s.trim()).filter(s => s !== "");
        if (ids.length === 0) {
            return subscriptions;
        }
        return ids.map(id => subscriptions.find(s => s.id === id)).filter(s => s !== undefined);
    }

    // The most used meter among the shown subscriptions, or null.
    readonly property var worst: {
        let found = null;
        for (const s of shown) {
            const meter = closest(s);
            if (meter !== null && (found === null || meter.used_percent > found.meter.used_percent)) {
                found = {subscription: s, meter: meter};
            }
        }
        return found;
    }
    readonly property real worstPercent: worst === null ? -1 : worst.meter.used_percent

    readonly property bool dimmed: problem !== "" || shown.length === 0 || shown.every(s => !lively(s))

    function read(exitCode, stdout, stderr) {
        if (exitCode !== 0) {
            problem = i18n("ataecina status failed: %1", (stderr || "").trim() || exitCode);
            return;
        }
        try {
            const document = JSON.parse(stdout);
            subscriptions = document.data.subscriptions;
            meta = document.meta;
            problem = "";
        } catch (error) {
            problem = i18n("ataecina status gave something the widget cannot read");
        }
    }

    // ----- what to show -----

    // A fresh reading that a source answered, even in part.
    function lively(subscription) {
        return subscription.freshness === "FRESH" && (subscription.outcome === "OK" || subscription.outcome === "PARTIAL");
    }

    function leftOf(meter) {
        return Math.max(0, 100 - meter.used_percent);
    }

    // The meter closest to running out, or null.
    function closest(subscription) {
        let found = null;
        for (const meter of subscription.meters) {
            if (found === null || meter.used_percent > found.used_percent) {
                found = meter;
            }
        }
        return found;
    }

    // Minutes of a window; a billing cycle or a month counts as 30 days.
    function windowLength(meter) {
        if (meter.window_minutes) {
            return meter.window_minutes;
        }
        return meter.window === "MONTHLY" || meter.window === "BILLING_CYCLE" ? 43200 : 0;
    }

    // The meters drawn as rings: the first bucket's windows, longest first.
    function ringMeters(subscription) {
        if (subscription.meters.length === 0) {
            return [];
        }
        const bucket = subscription.meters[0].bucket;
        return subscription.meters.filter(m => m.bucket === bucket).sort((a, b) => windowLength(b) - windowLength(a)).slice(0, growth.length);
    }

    function ringFractions(subscription) {
        return ringMeters(subscription).map(m => leftOf(m) / 100);
    }

    // A balance without windows gets one faint ring of the first growth
    // colour, with its currency in the middle.
    function ringColors(subscription) {
        if (subscription.meters.length === 0 && subscription.balance) {
            return [toneColor(headlineUsed(subscription), growth[0])];
        }
        return ringMeters(subscription).map((m, index) => toneColor(m.used_percent, growth[index]));
    }

    function failed(subscription) {
        return subscription.outcome !== "OK" && subscription.outcome !== "PARTIAL";
    }

    // What sits inside the rings: "!" when the source failed, the currency
    // of a balance without windows, or nothing.
    function badge(subscription) {
        if (failed(subscription)) {
            return "!";
        }
        if (subscription.meters.length > 0 || !subscription.balance) {
            return "";
        }
        const value = subscription.balance.remaining || subscription.balance.spent;
        const symbol = currencySymbols[value.currency];
        return symbol === undefined ? value.currency : symbol;
    }

    function toneColor(usedPercent, healthy) {
        if (usedPercent >= criticalPercent) {
            return criticalColor;
        }
        if (usedPercent >= warningPercent) {
            return lowColor;
        }
        return healthy;
    }

    // ----- wording -----

    function name(subscription) {
        const twins = shown.filter(s => s.source === subscription.source).length;
        const known = sourceNames[subscription.source];
        return twins > 1 || known === undefined ? subscription.id : known;
    }

    // PRO → Pro, INDIVIDUAL_PRO → Individual Pro.
    function plan(subscription) {
        if (!subscription.plan) {
            return "";
        }
        return subscription.plan.split("_").map(word => word.charAt(0) + word.slice(1).toLowerCase()).join(" ");
    }

    // Short label of a window inside a list of windows.
    function windowLabel(meter) {
        switch (meter.window) {
        case "FIVE_HOURS":
            return i18n("5 h");
        case "DAILY":
            return i18n("day");
        case "WEEKLY":
            return i18n("week");
        case "MONTHLY":
            return i18n("month");
        case "BILLING_CYCLE":
            return i18n("cycle");
        }
        const minutes = meter.window_minutes || 0;
        if (minutes % 1440 === 0) {
            return i18n("%1 d", minutes / 1440);
        }
        if (minutes % 60 === 0) {
            return i18n("%1 h", minutes / 60);
        }
        return i18n("%1 min", minutes);
    }

    // Name of a window at the start of a sentence.
    function windowName(meter) {
        switch (meter.window) {
        case "FIVE_HOURS":
            return i18n("5-hour");
        case "DAILY":
            return i18n("Daily");
        case "WEEKLY":
            return i18n("Weekly");
        case "MONTHLY":
            return i18n("Monthly");
        case "BILLING_CYCLE":
            return i18n("Billing cycle");
        }
        return windowLabel(meter);
    }

    function meterLabel(subscription, meter) {
        const first = subscription.meters.length > 0 ? subscription.meters[0].bucket : null;
        const label = windowLabel(meter);
        return meter.bucket && meter.bucket !== first ? meter.bucket + " " + label : label;
    }

    function duration(iso) {
        const minutes = Math.max(0, Math.round((new Date(iso) - now) / 60000));
        const days = Math.floor(minutes / 1440);
        const hours = Math.floor((minutes % 1440) / 60);
        if (days > 0) {
            return i18n("%1 d %2 h", days, hours);
        }
        if (hours > 0) {
            return i18n("%1 h %2 m", hours, minutes % 60);
        }
        return i18n("%1 m", minutes);
    }

    function countdown(iso) {
        return iso ? "↻ " + duration(iso) : "";
    }

    function age(iso) {
        if (!iso) {
            return "";
        }
        const minutes = Math.max(0, Math.round((now - new Date(iso)) / 60000));
        if (minutes < 1) {
            return i18n("just now");
        }
        if (minutes < 60) {
            return i18np("%1 min ago", "%1 min ago", minutes);
        }
        if (minutes < 2880) {
            return i18np("%1 hour ago", "%1 hours ago", Math.round(minutes / 60));
        }
        return i18np("%1 day ago", "%1 days ago", Math.round(minutes / 1440));
    }

    function clock(iso) {
        return iso ? Qt.formatTime(new Date(iso), Qt.locale().timeFormat(Locale.ShortFormat)) : "";
    }

    function money(value) {
        if (!value) {
            return "";
        }
        // Rounding must not print "-0.00".
        const amount = Math.abs(Number(value.amount)) < 0.005 ? 0 : Number(value.amount);
        const symbol = currencySymbols[value.currency];
        if (symbol === undefined) {
            return amount.toLocaleString(Qt.locale(), "f", 2) + " " + value.currency;
        }
        return amount.toLocaleCurrencyString(Qt.locale(), symbol);
    }

    // The second line of a row: when the closest window is reborn, or what
    // went wrong, or what was spent.
    function subtitle(subscription) {
        if (failed(subscription)) {
            return subscription.hint ? outcomeText(subscription) + " · " + subscription.hint : outcomeText(subscription);
        }
        if (subscription.freshness !== "FRESH") {
            return i18n("Stale, observed %1", age(subscription.observed_at));
        }
        const meter = closest(subscription);
        if (meter === null) {
            // The large figure already shows what was spent when nothing is left to show.
            const both = subscription.balance && subscription.balance.remaining && subscription.balance.spent;
            return both ? i18n("%1 spent", money(subscription.balance.spent)) : subscription.hint || "";
        }
        let when = windowName(meter);
        if (meter.resets_at) {
            when = i18n("%1 resets in %2", windowName(meter), duration(meter.resets_at));
        } else if (meter.estimated) {
            when = windowName(meter) + " · " + i18n("reborn");
        }
        const parts = [when];
        if (subscription.balance && subscription.balance.remaining) {
            parts.push(i18n("%1 left", approximate(subscription) + money(subscription.balance.remaining)));
        }
        return parts.join(" · ");
    }

    // The large figure of a row: what is left of the closest window, or of
    // the balance, or nothing.
    function headline(subscription) {
        const meter = closest(subscription);
        if (meter !== null) {
            return Math.round(leftOf(meter)).toString();
        }
        if (subscription.balance && subscription.balance.remaining) {
            return money(subscription.balance.remaining);
        }
        if (subscription.balance && subscription.balance.spent) {
            return money(subscription.balance.spent);
        }
        return "—";
    }

    function headlineIsPercent(subscription) {
        return closest(subscription) !== null;
    }

    // The small word after the large figure.
    function headlineUnit(subscription) {
        if (headlineIsPercent(subscription)) {
            return "%";
        }
        const spentOnly = subscription.balance && !subscription.balance.remaining && subscription.balance.spent;
        return spentOnly ? i18n("spent") : "";
    }

    // Used percent behind the large figure: an exhausted balance counts as
    // fully used.
    function headlineUsed(subscription) {
        const meter = closest(subscription);
        if (meter !== null) {
            return meter.used_percent;
        }
        if (subscription.balance && subscription.balance.remaining) {
            return Number(subscription.balance.remaining.amount) <= 0 ? 100 : 0;
        }
        return 0;
    }

    function approximate(subscription) {
        return subscription.fidelity === "REPORTED" || headline(subscription) === "—" ? "" : "≈ ";
    }

    function balanceText(subscription) {
        const parts = [];
        if (subscription.balance && subscription.balance.remaining) {
            parts.push(i18n("%1 left", approximate(subscription) + money(subscription.balance.remaining)));
        }
        if (subscription.balance && subscription.balance.spent) {
            parts.push(i18n("%1 spent", money(subscription.balance.spent)));
        }
        if (subscription.credits) {
            parts.push(subscription.credits.unlimited ? i18n("unlimited credits") : i18n("%1 credits", subscription.credits.available));
        }
        if (subscription.banked_resets && subscription.banked_resets.available > 0) {
            parts.push(i18np("%1 banked reset", "%1 banked resets", subscription.banked_resets.available));
        }
        return parts.join(" · ");
    }

    function outcomeText(subscription) {
        switch (subscription.outcome) {
        case "PARTIAL":
            return i18n("partial");
        case "BROKEN":
            return i18n("broken");
        case "UNAVAILABLE":
            return i18n("unavailable");
        case "KEY_MISSING":
            return i18n("key missing");
        case "LOCKED":
            return i18n("locked");
        case "UNSUPPORTED":
            return i18n("unsupported");
        }
        return "";
    }

    function fidelityText(subscription) {
        switch (subscription.fidelity) {
        case "DERIVED":
            return i18n("derived");
        case "OBSERVED":
            return i18n("observed");
        case "DECLARED":
            return i18n("declared");
        }
        return "";
    }

    function worstText() {
        if (worst === null) {
            return shown.length === 0 ? i18n("No snapshot yet") : i18n("No windows to show");
        }
        const parts = [name(worst.subscription), windowLabel(worst.meter) + " " + i18n("%1 % left", Math.round(leftOf(worst.meter)))];
        if (worst.meter.resets_at) {
            parts.push(countdown(worst.meter.resets_at));
        }
        return parts.join(" · ");
    }
}
