/*
 * Settings page for the subscriptions of config.json: lists them from
 * `ataecina subscriptions --json`, adds one with `ataecina subscriptions
 * add` (the source chosen from `ataecina sources --json`, only its
 * required options asked) and removes one with `ataecina subscriptions
 * remove`. The CLI validates every change; the widget never writes the
 * file itself. Keys are set on the Keys page.
 */
import QtQuick
import QtQuick.Controls as QQC2
import QtQuick.Layouts
import org.kde.kirigami as Kirigami
import org.kde.kcmutils as KCM
import org.kde.plasma.plasma5support as Plasma5Support

KCM.SimpleKCM {
    id: page

    property string cfg_command: "ataecina"
    readonly property string command: cfg_command || "ataecina"
    readonly property string listCommand: command + " subscriptions --json"
    readonly property string sourcesCommand: command + " sources --json"
    readonly property string refreshCommand: "systemctl --user start ataecina-collect.service"

    property var subscriptions: []
    property var sources: []
    property var values: ({})
    property string removing: ""
    property string message: ""

    readonly property var selected: sourceBox.currentIndex >= 0 && sourceBox.currentIndex < sources.length ? sources[sourceBox.currentIndex] : null

    function quoted(text) {
        return "'" + text.split("'").join("'\\''") + "'";
    }

    function validId(id) {
        return new RegExp("^[a-z0-9][a-z0-9-]{0,31}$").test(id);
    }

    // The provider's name, or provider-2, provider-3… when it is taken.
    function suggestedId(source) {
        const base = (source.provider || source.id.split("-")[0]).toLowerCase();
        const taken = subscriptions.map(s => s.id);
        if (taken.indexOf(base) < 0) {
            return base;
        }
        for (let n = 2; n < 100; n++) {
            if (taken.indexOf(base + "-" + n) < 0) {
                return base + "-" + n;
            }
        }
        return base;
    }

    function optionLabel(option) {
        switch (option) {
        case "account":
            return i18n("Cloudflare account id:");
        case "login":
            return i18n("GitHub login:");
        case "monthly_allowance":
            return i18n("Monthly allowance (requests or credits):");
        case "team":
            return i18n("xAI team id:");
        case "email":
            return i18n("Your e-mail in the team:");
        case "provider":
            return i18n("Provider Heimdall follows:");
        }
        return option + ":";
    }

    function add() {
        const id = idField.text.trim();
        if (!selected || !validId(id)) {
            message = i18n("The id takes lowercase letters, digits and dashes.");
            return;
        }
        let line = command + " subscriptions add " + quoted(id) + " --source " + quoted(selected.id);
        if (planField.text.trim() !== "") {
            line += " --plan " + quoted(planField.text.trim());
        }
        for (const option of selected.required) {
            const value = (values[option] || "").trim();
            if (value === "") {
                message = i18n("Fill in every field.");
                return;
            }
            line += " --set " + quoted(option + "=" + value);
        }
        run(line);
    }

    function remove(id) {
        if (removing !== id) {
            removing = id;
            return;
        }
        removing = "";
        if (validId(id)) {
            run(command + " subscriptions remove " + quoted(id));
        }
    }

    function run(line) {
        message = "";
        executable.connectSource(line);
    }

    Plasma5Support.DataSource {
        id: executable
        engine: "executable"
        connectedSources: []
        onNewData: function (source, data) {
            disconnectSource(source);
            if (source === page.listCommand || source === page.sourcesCommand) {
                try {
                    const parsed = JSON.parse(data["stdout"]).data;
                    if (source === page.listCommand) {
                        page.subscriptions = parsed;
                    } else {
                        page.sources = parsed;
                    }
                    if (idField.text === "" && page.selected) {
                        idField.text = page.suggestedId(page.selected);
                    }
                } catch (error) {
                    page.message = (data["stderr"] || "").trim();
                }
                return;
            }
            if (source === page.refreshCommand) {
                return;
            }
            if (data["exit code"] === 0) {
                const added = page.selected && page.selected.needs_key && source.indexOf(" subscriptions add ") >= 0;
                page.message = added ? i18n("Added. Set its key on the Keys page.") : i18n("Saved. Reading the subscriptions again…");
                page.values = {};
                idField.text = "";
                planField.text = "";
                connectSource(page.listCommand);
                connectSource(page.refreshCommand);
            } else {
                page.message = (data["stderr"] || "").trim();
            }
        }
    }

    Component.onCompleted: {
        executable.connectSource(sourcesCommand);
        executable.connectSource(listCommand);
    }

    ColumnLayout {
        spacing: Kirigami.Units.largeSpacing

        Kirigami.Heading {
            text: i18n("Subscriptions")
            level: 3
        }
        QQC2.Label {
            text: i18n("Two subscriptions of the same provider need different ids, and each its own key.")
            wrapMode: Text.Wrap
            opacity: 0.7
            Layout.fillWidth: true
        }

        Repeater {
            model: page.subscriptions

            RowLayout {
                required property var modelData
                Layout.fillWidth: true

                QQC2.Label {
                    text: modelData.id
                    font.weight: Font.DemiBold
                    Layout.preferredWidth: Kirigami.Units.gridUnit * 8
                }
                QQC2.Label {
                    text: modelData.plan ? modelData.source + " · " + modelData.plan : modelData.source
                    opacity: 0.7
                    elide: Text.ElideRight
                    Layout.fillWidth: true
                }
                QQC2.Button {
                    icon.name: "list-remove"
                    text: page.removing === modelData.id ? i18n("Remove it") : i18n("Remove…")
                    onClicked: page.remove(modelData.id)
                }
            }
        }

        Kirigami.Heading {
            text: i18n("Add a subscription")
            level: 3
        }
        GridLayout {
            columns: 2
            Layout.fillWidth: true

            QQC2.Label {
                text: i18n("Source:")
            }
            QQC2.ComboBox {
                id: sourceBox
                model: page.sources
                textRole: "id"
                Layout.fillWidth: true
                onActivated: {
                    page.values = {};
                    if (page.selected) {
                        idField.text = page.suggestedId(page.selected);
                    }
                }
            }
            QQC2.Label {
                text: i18n("Id:")
            }
            QQC2.TextField {
                id: idField
                placeholderText: "minimax-2"
                Layout.fillWidth: true
            }
            QQC2.Label {
                text: i18n("Plan (optional):")
            }
            QQC2.TextField {
                id: planField
                Layout.fillWidth: true
            }
            Repeater {
                model: page.selected ? page.selected.required : []

                QQC2.Label {
                    required property var modelData
                    text: page.optionLabel(modelData)
                    Layout.row: 3 + index
                    Layout.column: 0
                    required property int index
                }
            }
            Repeater {
                model: page.selected ? page.selected.required : []

                QQC2.TextField {
                    required property var modelData
                    required property int index
                    Layout.row: 3 + index
                    Layout.column: 1
                    Layout.fillWidth: true
                    onTextChanged: page.values[modelData] = text
                }
            }
        }
        QQC2.Button {
            enabled: page.selected !== null && idField.text.trim() !== ""
            icon.name: "list-add"
            text: i18n("Add")
            onClicked: page.add()
        }

        QQC2.Label {
            visible: text !== ""
            text: page.message
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
    }
}
