/*
 * Popup form: one block per subscription with a bar per window, the
 * countdown to rebirth, the balance, fidelity and outcome; a header with
 * the snapshot's age and "refresh now" (SPECS §8.3).
 */
import QtQuick
import QtQuick.Layouts
import org.kde.plasma.components as PlasmaComponents3
import org.kde.plasma.extras as PlasmaExtras
import org.kde.kirigami as Kirigami

PlasmaExtras.Representation {
    id: full

    Layout.minimumWidth: Kirigami.Units.gridUnit * 20
    Layout.minimumHeight: Kirigami.Units.gridUnit * 10
    Layout.preferredWidth: Kirigami.Units.gridUnit * 24
    Layout.preferredHeight: Kirigami.Units.gridUnit * 22

    header: PlasmaExtras.PlasmoidHeading {
        RowLayout {
            anchors.fill: parent

            Kirigami.Heading {
                text: i18n("Ataecina")
                level: 3
                Layout.fillWidth: true
            }
            PlasmaComponents3.Label {
                text: root.meta.snapshot_taken_at ? i18n("updated %1", root.age(root.meta.snapshot_taken_at)) : ""
                opacity: 0.7
            }
            PlasmaComponents3.ToolButton {
                icon.name: "view-refresh"
                onClicked: root.refresh()
                PlasmaComponents3.ToolTip {
                    text: i18n("Refresh now")
                }
            }
        }
    }

    PlasmaComponents3.ScrollView {
        anchors.fill: parent

        ListView {
            model: root.shown
            spacing: Kirigami.Units.largeSpacing
            clip: true
            delegate: SubscriptionDelegate {
                width: ListView.view.width
            }
        }
    }

    PlasmaExtras.PlaceholderMessage {
        anchors.centerIn: parent
        width: parent.width - Kirigami.Units.gridUnit * 4
        visible: root.problem !== "" || root.shown.length === 0
        iconName: root.problem !== "" ? "dialog-warning" : "ataecina"
        text: root.problem !== "" ? root.problem : i18n("No snapshot yet")
        explanation: root.problem !== "" ? "" : i18n("Run `ataecina collect`, or enable ataecina-collect.timer.")
    }
}
