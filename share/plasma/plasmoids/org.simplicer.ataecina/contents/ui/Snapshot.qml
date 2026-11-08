/*
 * The snapshot as the widget sees it: what `ataecina status --json` said
 * (SPECS §8.2), which subscriptions get a row, which a warning, and how to
 * word and colour each figure. No Plasma type here, so the popup can be
 * drawn on its own.
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
    // Whether the language writes the percent sign before the number
    // (Basque: "% 76"), read from the translation of "%1%".
    readonly property bool percentSignFirst: i18n("%1%", "0").trim().startsWith("%")

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
    readonly property var listed: shown.filter(s => placement(s) === "row")
    readonly property var warnings: shown.filter(s => placement(s) === "warning")

    // The most used meter among the listed subscriptions, or null.
    readonly property var worst: {
        let found = null;
        for (const s of listed) {
            const meter = closest(s);
            if (meter !== null && (found === null || meter.used_percent > found.meter.used_percent)) {
                found = {subscription: s, meter: meter};
            }
        }
        return found;
    }
    readonly property real worstPercent: worst === null ? -1 : worst.meter.used_percent

    readonly property bool dimmed: problem !== "" || listed.length === 0 || listed.every(s => !lively(s))

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

    // ----- where each subscription goes -----

    function failed(subscription) {
        return subscription.outcome !== "OK" && subscription.outcome !== "PARTIAL";
    }

    // A fresh reading that a source answered, even in part.
    function lively(subscription) {
        return subscription.freshness === "FRESH" && !failed(subscription);
    }

    // "row" when there is something to show (windows, money left, or the
    // spending of a source that cannot know what is left yet, such as an
    // undeclared OpenCode Zen balance), "warning" when the source failed or
    // asks for something it cannot do without, and "hidden" when it answers
    // but has nothing worth showing: a free credit used up, or only what was
    // spent of a finished one.
    function placement(subscription) {
        if (failed(subscription)) {
            return "warning";
        }
        if (subscription.meters.length > 0 || moneyLeft(subscription) > 0 || spentOnly(subscription)) {
            return "row";
        }
        return subscription.outcome === "PARTIAL" && subscription.hint ? "warning" : "hidden";
    }

    // A partial reading that knows only the money spent.
    function spentOnly(subscription) {
        const balance = subscription.balance;
        return subscription.outcome === "PARTIAL" && balance !== null && !balance.remaining && !!balance.spent;
    }

    // ----- windows -----

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

    // The first bucket's windows, shortest first (5 h, week, month); other
    // buckets show when the row unfolds.
    function windows(subscription) {
        if (subscription.meters.length === 0) {
            return [];
        }
        const bucket = subscription.meters[0].bucket;
        return subscription.meters.filter(m => m.bucket === bucket).sort((a, b) => windowLength(a) - windowLength(b)).slice(0, growth.length);
    }

    // Rings, outer first: one per known window, longest outside; for money
    // left of a known total, one ring; nothing when nothing is known.
    function ringFractions(subscription) {
        const outerFirst = windows(subscription).reverse();
        if (outerFirst.length > 0) {
            return outerFirst.map(m => leftOf(m) / 100);
        }
        const total = moneyTotal(subscription);
        return total > 0 ? [moneyLeft(subscription) / total] : [];
    }

    function ringColors(subscription) {
        const outerFirst = windows(subscription).reverse();
        if (outerFirst.length > 0) {
            return outerFirst.map((m, index) => toneColor(m.used_percent, growth[index]));
        }
        return [toneColor(moneyUsedPercent(subscription), growth[0])];
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

    // ----- money -----

    function moneyLeft(subscription) {
        return subscription.balance && subscription.balance.remaining ? Number(subscription.balance.remaining.amount) : 0;
    }

    // What was left plus what was spent, or 0 when either is unknown.
    function moneyTotal(subscription) {
        if (!subscription.balance || !subscription.balance.remaining || !subscription.balance.spent) {
            return 0;
        }
        return Number(subscription.balance.remaining.amount) + Number(subscription.balance.spent.amount);
    }

    function moneyUsedPercent(subscription) {
        const total = moneyTotal(subscription);
        return total > 0 ? 100 * (1 - moneyLeft(subscription) / total) : 0;
    }

    function money(value) {
        if (!value) {
            return "";
        }
        // Rounding must not print "-0.00".
        const amount = Math.abs(Number(value.amount)) < 0.005 ? 0 : Number(value.amount);
        return amountText(amount, value.currency);
    }

    function amountText(amount, currency) {
        const symbol = currencySymbols[currency];
        if (symbol === undefined) {
            return amount.toLocaleString(Qt.locale(), "f", 2) + " " + currency;
        }
        return amount.toLocaleCurrencyString(Qt.locale(), symbol);
    }

    // The currency inside a money ring.
    function currencyMark(subscription) {
        if (subscription.meters.length > 0 || moneyLeft(subscription) <= 0) {
            return "";
        }
        const currency = subscription.balance.remaining.currency;
        const symbol = currencySymbols[currency];
        return symbol === undefined ? currency : symbol;
    }

    // "$6.80 of $70.00", or "$6.80 left" when the total is unknown, or
    // "$1.20 spent" when only the spending is known; empty when nothing is
    // left.
    function moneyLine(subscription) {
        if (spentOnly(subscription)) {
            return i18n("%1 spent", money(subscription.balance.spent));
        }
        if (moneyLeft(subscription) <= 0) {
            return "";
        }
        const left = approximate(subscription) + money(subscription.balance.remaining);
        const total = moneyTotal(subscription);
        if (total > 0) {
            return i18n("%1 of %2", left, amountText(total, subscription.balance.remaining.currency));
        }
        return i18n("%1 left", left);
    }

    function approximate(subscription) {
        return subscription.fidelity === "REPORTED" ? "" : "≈ ";
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

    // Short label of a window.
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

    function meterLabel(subscription, meter) {
        const first = subscription.meters.length > 0 ? subscription.meters[0].bucket : null;
        const label = windowLabel(meter);
        return meter.bucket && meter.bucket !== first ? meter.bucket + " " + label : label;
    }

    // What is left of a window, "≈" when estimated after a rebirth.
    function leftText(meter) {
        return (meter.estimated ? "≈ " : "") + Math.round(leftOf(meter));
    }

    // When a window is reborn, or that it already was.
    function resetText(meter) {
        if (meter.resets_at) {
            return countdown(meter.resets_at);
        }
        return meter.estimated ? i18n("reborn") : "";
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

    // The small line under a row: how a stale reading gets fresh again, and
    // banked resets.
    function note(subscription) {
        const parts = [];
        if (subscription.freshness !== "FRESH") {
            switch (subscription.source) {
            case "CLAUDE_STATUSLINE":
                parts.push(i18n("Updates when you use Claude Code"));
                break;
            case "CODEX_LOGS":
                parts.push(i18n("Updates when you use Codex"));
                break;
            default:
                parts.push(i18n("Stale, observed %1", age(subscription.observed_at)));
            }
        }
        if (spentOnly(subscription) && subscription.source === "OPENCODE_ZEN") {
            parts.push(i18n("Declare its balance in the settings to see what is left"));
        }
        if (subscription.banked_resets && subscription.banked_resets.available > 0) {
            parts.push(i18np("%1 banked reset", "%1 banked resets", subscription.banked_resets.available));
        }
        return parts.join(" · ");
    }

    // Why a subscription is a warning.
    function warningText(subscription) {
        const reason = failed(subscription) ? outcomeText(subscription) : "";
        if (!subscription.hint) {
            return reason;
        }
        return reason === "" ? subscription.hint : reason + ": " + subscription.hint;
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
        return parts.join(" · ");
    }

    // ----- model catalogue -----

    // How many models the subscription's provider lists and how many are
    // free; a plan whose every model costs nothing includes them all.
    function modelsText(subscription) {
        const offerings = subscription.offerings;
        if (!offerings || offerings.models === 0) {
            return "";
        }
        if (offerings.free === offerings.models) {
            return i18np("%1 model included", "%1 models included", offerings.models);
        }
        const count = i18np("%1 model", "%1 models", offerings.models);
        return offerings.free === 0 ? count : count + " · " + i18np("%1 free", "%1 free", offerings.free);
    }

    // A few free models by name, when only some are free.
    function freeModelsText(subscription) {
        const offerings = subscription.offerings;
        if (!offerings || offerings.free === 0 || offerings.free === offerings.models) {
            return "";
        }
        const shown = offerings.free_models.slice(0, 6).map(m => m.name);
        const more = offerings.free - shown.length;
        if (more > 0) {
            shown.push(i18np("%1 more", "%1 more", more));
        }
        return i18n("Free: %1", shown.join(", "));
    }

    // The newest models, released in the last 30 days.
    function newModelsText(subscription) {
        const offerings = subscription.offerings;
        if (!offerings || offerings.new_models.length === 0) {
            return "";
        }
        return i18n("New: %1", offerings.new_models.slice(0, 4).map(m => m.name).join(", "));
    }

    function outcomeText(subscription) {
        // A rejected key also reads KEY_MISSING; its error class tells them apart.
        if (subscription.error_class === "KEY_REJECTED") {
            return i18n("key rejected");
        }
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
        const parts = [name(worst.subscription), windowLabel(worst.meter) + " " + i18n("%1% left", Math.round(leftOf(worst.meter)))];
        if (worst.meter.resets_at) {
            parts.push(countdown(worst.meter.resets_at));
        }
        return parts.join(" · ");
    }
}
