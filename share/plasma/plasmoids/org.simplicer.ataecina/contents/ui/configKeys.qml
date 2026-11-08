/*
 * Settings page for keys and balances (ADR 0014): which subscriptions need
 * a key and whether it works, from `ataecina status --json`. "Set key…"
 * opens KDE's masked password dialog, whose answer goes through a pipe
 * straight into `ataecina keys set`: the widget never sees the key.
 */
import QtQuick
import QtQuick.Controls as QQC2
import QtQuick.Layouts
import org.kde.kirigami as Kirigami
import org.kde.kcmutils as KCM
import org.kde.plasma.plasma5support as Plasma5Support

KCM.SimpleKCM {
    id: page

    // The settings dialog fills it with the widget's own setting.
    property string cfg_command: "ataecina"
    property string cfg_language: ""
    readonly property string command: cfg_command || "ataecina"
    readonly property string statusCommand: command + " status --json"
    readonly property string dialogCheck: "command -v kdialog"
    readonly property string refreshCommand: "systemctl --user start ataecina-collect.service"
    readonly property string sourcesCommand: command + " sources --json"

    property var subscriptions: []
    property bool dialogs: true
    property string message: ""

    // Sources that read with a key of the owner's, from `ataecina sources`.
    property var keySources: []
    readonly property var keyed: subscriptions.filter(s => keySources.indexOf(s.source) >= 0)
    readonly property var declared: subscriptions.filter(s => s.source === "OPENCODE_ZEN")

    // Single-quoted for the shell, quotes escaped.
    function quoted(text) {
        return "'" + text.split("'").join("'\\''") + "'";
    }

    // A subscription id as config.json allows it (SPECS §9).
    function validId(id) {
        return new RegExp("^[a-z0-9][a-z0-9-]{0,31}$").test(id);
    }

    // A rejected key also reads KEY_MISSING; its error class tells them apart.
    function keyState(subscription) {
        if (subscription.error_class === "KEY_REJECTED") {
            return l10n.i18n("key rejected");
        }
        if (subscription.outcome === "KEY_MISSING") {
            return l10n.i18n("no key");
        }
        if (subscription.outcome === "OK" || subscription.outcome === "PARTIAL") {
            return l10n.i18n("works");
        }
        if (subscription.outcome === "BROKEN") {
            return l10n.i18n("key accepted, answer not understood");
        }
        return l10n.i18n("not read yet");
    }

    function keyColor(subscription) {
        if (subscription.error_class === "KEY_REJECTED" || subscription.outcome === "KEY_MISSING") {
            return Kirigami.Theme.negativeTextColor;
        }
        if (subscription.outcome === "OK" || subscription.outcome === "PARTIAL") {
            return Kirigami.Theme.positiveTextColor;
        }
        return Kirigami.Theme.neutralTextColor;
    }

    function hasKey(subscription) {
        return subscription.outcome !== "KEY_MISSING" || subscription.error_class === "KEY_REJECTED";
    }

    function balanceState(subscription) {
        const remaining = subscription.balance ? subscription.balance.remaining : null;
        if (!remaining) {
            return l10n.i18n("no balance declared");
        }
        return l10n.i18n("%1 left", Number(remaining.amount).toLocaleString(l10n.locale, "f", 2) + " " + remaining.currency);
    }

    function setKey(subscription) {
        if (!validId(subscription.id)) {
            return;
        }
        const prompt = l10n.i18n("API key for %1. It is sealed at once and never shown.", subscription.id);
        run("kdialog --title Ataecina --password " + quoted(prompt) + " | " + command + " keys set " + quoted(subscription.id));
    }

    function declareBalance(subscription) {
        if (!validId(subscription.id)) {
            return;
        }
        const prompt = l10n.i18n("Current balance of %1, in US dollars:", subscription.id);
        // Declaring reads the spending so far with opencode, which pnpm may
        // keep off plasmashell's PATH: the same directories as the collector unit.
        const path = "PATH=\"$HOME/.local/bin:$HOME/.local/share/pnpm:$HOME/.local/share/pnpm/bin:$PATH\" ";
        run("amount=$(kdialog --title Ataecina --inputbox " + quoted(prompt) + ") && " + path + command + " balance declare " + quoted(subscription.id) + " --balance \"$amount\"");
    }

    function run(line) {
        message = "";
        executable.connectSource(line);
    }

    // KDE's own i18n, which the Translator cannot reach from inside.
    function desktopI18n(text, ...args) {
        return i18n(text, ...args);
    }

    function desktopI18np(singular, plural, count, ...args) {
        return i18np(singular, plural, count, ...args);
    }

    Translator {
        id: l10n
        language: page.cfg_language
        desktop: page.desktopI18n
        desktopPlural: page.desktopI18np
    }

    Plasma5Support.DataSource {
        id: executable
        engine: "executable"
        connectedSources: []
        onNewData: function (source, data) {
            disconnectSource(source);
            if (source === page.dialogCheck) {
                page.dialogs = data["exit code"] === 0;
                return;
            }
            if (source === page.sourcesCommand) {
                try {
                    page.keySources = JSON.parse(data["stdout"]).data.filter(s => s.needs_key).map(s => s.label);
                } catch (error) {
                    page.message = (data["stderr"] || "").trim();
                }
                return;
            }
            if (source === page.statusCommand) {
                try {
                    page.subscriptions = JSON.parse(data["stdout"]).data.subscriptions;
                } catch (error) {
                    page.message = (data["stderr"] || "").trim();
                }
                return;
            }
            if (source === page.refreshCommand) {
                reread.restart();
                return;
            }
            if (data["exit code"] === 0) {
                page.message = l10n.i18n("Saved. Reading the subscriptions again…");
                connectSource(page.refreshCommand);
            } else {
                page.message = (data["stderr"] || "").trim();
            }
        }
    }

    Timer {
        // The collector usually answers within a few seconds.
        id: reread
        interval: 4000
        onTriggered: executable.connectSource(page.statusCommand)
    }

    Component.onCompleted: {
        executable.connectSource(dialogCheck);
        executable.connectSource(sourcesCommand);
        executable.connectSource(statusCommand);
    }

    ColumnLayout {
        spacing: Kirigami.Units.largeSpacing

        Kirigami.Heading {
            text: l10n.i18n("API keys")
            level: 3
        }
        QQC2.Label {
            text: l10n.i18n("Keys go from a masked dialog straight to `ataecina keys set`.") + " " + l10n.i18n("The widget never sees them.")
            wrapMode: Text.Wrap
            opacity: 0.7
            Layout.fillWidth: true
        }
        QQC2.Label {
            visible: !page.dialogs
            text: l10n.i18n("Without kdialog, set keys in a terminal: `ataecina keys set <name>`.")
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
        QQC2.Label {
            visible: page.keyed.length === 0
            text: l10n.i18n("No subscription needs a key.")
            opacity: 0.7
        }

        Repeater {
            model: page.keyed

            RowLayout {
                required property var modelData
                Layout.fillWidth: true

                QQC2.Label {
                    text: modelData.id
                    font.weight: Font.DemiBold
                    Layout.preferredWidth: Kirigami.Units.gridUnit * 8
                }
                QQC2.Label {
                    text: page.keyState(modelData)
                    color: page.keyColor(modelData)
                    Layout.fillWidth: true
                }
                QQC2.Button {
                    enabled: page.dialogs
                    icon.name: "dialog-password"
                    text: page.hasKey(modelData) ? l10n.i18n("Replace key…") : l10n.i18n("Set key…")
                    onClicked: page.setKey(modelData)
                }
            }
        }

        Kirigami.Heading {
            visible: page.declared.length > 0
            text: l10n.i18n("Balances")
            level: 3
        }
        Repeater {
            model: page.declared

            RowLayout {
                required property var modelData
                Layout.fillWidth: true

                QQC2.Label {
                    text: modelData.id
                    font.weight: Font.DemiBold
                    Layout.preferredWidth: Kirigami.Units.gridUnit * 8
                }
                QQC2.Label {
                    text: page.balanceState(modelData)
                    Layout.fillWidth: true
                }
                QQC2.Button {
                    enabled: page.dialogs
                    icon.name: "wallet-open"
                    text: l10n.i18n("Declare balance…")
                    onClicked: page.declareBalance(modelData)
                }
            }
        }

        Kirigami.Heading {
            text: l10n.i18n("Intelligence index")
            level: 3
        }
        QQC2.Label {
            text: l10n.i18n("A free Artificial Analysis key ranks the models of each subscription. Only `ataecina update` uses it. Without it, the index comes through OpenRouter when you have an OpenRouter key.")
            wrapMode: Text.Wrap
            opacity: 0.7
            Layout.fillWidth: true
        }
        RowLayout {
            Layout.fillWidth: true

            QQC2.Label {
                text: "artificial-analysis"
                font.weight: Font.DemiBold
                Layout.fillWidth: true
            }
            QQC2.Button {
                icon.name: "internet-web-browser"
                text: l10n.i18n("Get a key")
                onClicked: Qt.openUrlExternally("https://artificialanalysis.ai/")
            }
            QQC2.Button {
                enabled: page.dialogs
                icon.name: "dialog-password"
                text: l10n.i18n("Set key…")
                onClicked: page.setKey({ id: "artificial-analysis" })
            }
        }

        QQC2.Label {
            visible: text !== ""
            text: page.message
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
    }
}
