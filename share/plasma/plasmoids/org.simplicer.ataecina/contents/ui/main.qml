/*
 * Ataecina — how much is left of each AI subscription, and when it is
 * reborn. The widget only runs `ataecina status --json` (SPECS §8.2) and,
 * for "refresh now", starts ataecina-collect.service through systemd. It
 * never collects, never reads keys and has no network code (ADR 0007).
 */
import QtQuick
import org.kde.plasma.plasmoid
import org.kde.plasma.core as PlasmaCore
import org.kde.plasma.plasma5support as Plasma5Support
import org.kde.kirigami as Kirigami

PlasmoidItem {
    id: root

    readonly property int warningPercent: 90
    readonly property int criticalPercent: 95

    property var subscriptions: []
    property var meta: ({})
    property string problem: ""
    property date now: new Date()

    readonly property string statusCommand: (Plasmoid.configuration.command || "ataecina") + " status --json"
    readonly property string refreshCommand: "systemctl --user start ataecina-collect.service"

    // The subscriptions to show, in the configured order.
    readonly property var shown: {
        const wanted = (Plasmoid.configuration.subscriptions || "").split(",").map(s => s.trim()).filter(s => s !== "");
        if (wanted.length === 0) {
            return subscriptions;
        }
        return wanted.map(id => subscriptions.find(s => s.id === id)).filter(s => s !== undefined);
    }

    // The most used meter among the shown subscriptions, or null.
    readonly property var worst: {
        let found = null;
        for (const s of shown) {
            for (const m of s.meters) {
                if (found === null || m.used_percent > found.meter.used_percent) {
                    found = {subscription: s, meter: m};
                }
            }
        }
        return found;
    }
    readonly property real worstPercent: worst === null ? -1 : worst.meter.used_percent

    readonly property bool dimmed: problem !== "" || shown.length === 0
        || shown.every(s => s.freshness !== "FRESH" || s.outcome !== "OK")

    readonly property color severityColor: {
        if (dimmed) {
            return Kirigami.Theme.disabledTextColor;
        }
        if (worstPercent >= criticalPercent) {
            return Kirigami.Theme.negativeTextColor;
        }
        if (worstPercent >= warningPercent) {
            return Kirigami.Theme.neutralTextColor;
        }
        return Kirigami.Theme.highlightColor;
    }

    Plasmoid.icon: "ataecina"
    Plasmoid.status: worstPercent >= warningPercent ? PlasmaCore.Types.NeedsAttentionStatus : PlasmaCore.Types.ActiveStatus
    toolTipMainText: i18n("Ataecina")
    toolTipSubText: problem !== "" ? problem : worstText()

    switchWidth: Kirigami.Units.gridUnit * 12
    switchHeight: Kirigami.Units.gridUnit * 12
    compactRepresentation: CompactRepresentation {}
    fullRepresentation: FullRepresentation {}

    Plasmoid.contextualActions: [
        PlasmaCore.Action {
            text: i18n("Refresh now")
            icon.name: "view-refresh"
            onTriggered: root.refresh()
        }
    ]

    Plasma5Support.DataSource {
        id: executable
        engine: "executable"
        connectedSources: []
        onNewData: function (source, data) {
            disconnectSource(source);
            if (source === root.statusCommand) {
                root.read(data["exit code"], data["stdout"], data["stderr"]);
            } else {
                afterRefresh.restart();
            }
        }
    }

    Timer {
        interval: Math.max(10, Plasmoid.configuration.refreshSeconds) * 1000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: root.update()
    }

    Timer {
        // The collector usually answers within a few seconds.
        id: afterRefresh
        interval: 4000
        onTriggered: root.update()
    }

    function update() {
        now = new Date();
        executable.connectSource(statusCommand);
    }

    function refresh() {
        executable.connectSource(refreshCommand);
    }

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

    // ----- formatting shared by the representations -----

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

    function countdown(iso) {
        if (!iso) {
            return "";
        }
        const minutes = Math.max(0, Math.round((new Date(iso) - now) / 60000));
        const days = Math.floor(minutes / 1440);
        const hours = Math.floor((minutes % 1440) / 60);
        if (days > 0) {
            return "↻ " + i18n("%1 d %2 h", days, hours);
        }
        if (hours > 0) {
            return "↻ " + i18n("%1 h %2 m", hours, minutes % 60);
        }
        return "↻ " + i18n("%1 m", minutes);
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

    function money(value) {
        if (!value) {
            return "";
        }
        return Number(value.amount).toLocaleString(Qt.locale(), "f", 2) + " " + value.currency;
    }

    function balanceText(subscription) {
        const parts = [];
        const approximate = subscription.fidelity === "REPORTED" ? "" : "≈ ";
        if (subscription.balance && subscription.balance.remaining) {
            parts.push(i18n("%1 left", approximate + money(subscription.balance.remaining)));
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

    function percentColor(percent) {
        if (percent >= criticalPercent) {
            return Kirigami.Theme.negativeTextColor;
        }
        if (percent >= warningPercent) {
            return Kirigami.Theme.neutralTextColor;
        }
        return Kirigami.Theme.textColor;
    }

    function worstText() {
        if (worst === null) {
            return shown.length === 0 ? i18n("No snapshot yet") : i18n("No windows to show");
        }
        const parts = [worst.subscription.id, windowLabel(worst.meter) + " " + i18n("%1 %", Math.round(worst.meter.used_percent))];
        if (worst.meter.resets_at) {
            parts.push(countdown(worst.meter.resets_at));
        }
        return parts.join(" · ");
    }
}
