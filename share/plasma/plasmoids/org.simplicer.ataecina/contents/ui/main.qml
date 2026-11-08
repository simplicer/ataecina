/*
 * Ataecina — how much is left of each AI subscription, and when it is
 * reborn. The widget runs `ataecina status --json` (SPECS §8.2) and, when a
 * row's models unfold, `ataecina models --json <id>`; "refresh now" starts
 * ataecina-collect.service through systemd and "update models" runs
 * `ataecina update` (ADR 0016). It never collects, never reads keys and has
 * no network code (ADR 0007).
 * It fits a panel or the system tray.
 */
import QtQuick
import org.kde.plasma.plasmoid
import org.kde.plasma.core as PlasmaCore
import org.kde.plasma.plasma5support as Plasma5Support
import org.kde.kirigami as Kirigami

PlasmoidItem {
    id: root

    readonly property string command: Plasmoid.configuration.command || "ataecina"
    readonly property string statusCommand: command + " status --json"
    readonly property string modelsPrefix: command + " models --json "
    readonly property string updateCommand: command + " update"
    property bool updating: false
    property string updateProblem: ""
    readonly property string refreshCommand: "systemctl --user start ataecina-collect.service"

    Snapshot {
        id: snapshot
        wanted: Plasmoid.configuration.subscriptions || ""
    }

    Plasmoid.icon: "ataecina"
    Plasmoid.status: snapshot.worstPercent >= snapshot.warningPercent ? PlasmaCore.Types.NeedsAttentionStatus : PlasmaCore.Types.ActiveStatus
    toolTipMainText: i18n("Ataecina")
    toolTipSubText: snapshot.problem !== "" ? snapshot.problem : snapshot.worstText()

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
                snapshot.read(data["exit code"], data["stdout"], data["stderr"]);
            } else if (source.startsWith(root.modelsPrefix)) {
                snapshot.readModels(data["stdout"]);
            } else if (source === root.updateCommand) {
                root.updating = false;
                root.updateProblem = data["exit code"] === 0 ? "" : (data["stderr"] || "").trim();
                for (const id of Object.keys(snapshot.models)) {
                    root.loadModels(id);
                }
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
        snapshot.now = new Date();
        executable.connectSource(statusCommand);
    }

    // Single-quoted for the shell, quotes escaped.
    function quoted(text) {
        return "'" + text.split("'").join("'\\''") + "'";
    }

    function loadModels(id) {
        executable.connectSource(modelsPrefix + quoted(id));
    }

    function updateModels() {
        updating = true;
        updateProblem = "";
        executable.connectSource(updateCommand);
    }

    function refresh() {
        executable.connectSource(refreshCommand);
    }

    function configure() {
        Plasmoid.internalAction("configure").trigger();
    }
}
