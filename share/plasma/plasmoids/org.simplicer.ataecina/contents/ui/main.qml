/*
 * Ataecina — how much is left of each AI subscription, and when it is
 * reborn. The widget only runs `ataecina status --json` (SPECS §8.2) and,
 * for "refresh now", starts ataecina-collect.service through systemd. It
 * never collects, never reads keys and has no network code (ADR 0007).
 * It fits a panel or the system tray.
 */
import QtQuick
import org.kde.plasma.plasmoid
import org.kde.plasma.core as PlasmaCore
import org.kde.plasma.plasma5support as Plasma5Support
import org.kde.kirigami as Kirigami

PlasmoidItem {
    id: root

    readonly property string statusCommand: (Plasmoid.configuration.command || "ataecina") + " status --json"
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

    function refresh() {
        executable.connectSource(refreshCommand);
    }

    function configure() {
        Plasmoid.internalAction("configure").trigger();
    }
}
