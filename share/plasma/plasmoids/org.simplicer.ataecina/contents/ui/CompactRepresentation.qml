/*
 * Panel and tray form: one ring around the Ataecina icon showing what is
 * left of the window closest to running out, optionally with that
 * percentage beside it in a panel (SPECS §8.3).
 */
import QtQuick
import QtQuick.Layouts
import org.kde.plasma.plasmoid
import org.kde.plasma.core as PlasmaCore
import org.kde.plasma.components as PlasmaComponents3
import org.kde.kirigami as Kirigami

MouseArea {
    id: compact

    readonly property bool inTray: (Plasmoid.containmentDisplayHints & PlasmaCore.Types.ContainmentDrawsPlasmoidHeading) !== 0
    readonly property real used: snapshot.worstPercent
    readonly property real leftPercent: Math.max(0, 100 - used)
    readonly property color ringColor: snapshot.dimmed ? Kirigami.Theme.disabledTextColor : snapshot.toneColor(used, snapshot.growth[0])

    Layout.minimumWidth: row.implicitWidth
    Layout.preferredWidth: row.implicitWidth
    hoverEnabled: true
    acceptedButtons: Qt.LeftButton | Qt.MiddleButton
    onClicked: function (mouse) {
        if (mouse.button === Qt.MiddleButton) {
            root.refresh();
        } else {
            root.expanded = !root.expanded;
        }
    }

    RowLayout {
        id: row
        anchors.fill: parent
        spacing: Kirigami.Units.smallSpacing

        Item {
            Layout.fillHeight: true
            Layout.preferredWidth: height

            Kirigami.Icon {
                anchors.fill: parent
                anchors.margins: Math.round(parent.height * 0.22)
                source: Plasmoid.icon
                active: compact.containsMouse
            }

            Rings {
                anchors.fill: parent
                fractions: compact.used >= 0 ? [compact.leftPercent / 100] : []
                colors: [compact.ringColor]
            }
        }

        PlasmaComponents3.Label {
            visible: !compact.inTray && Plasmoid.configuration.showPercentage && compact.used >= 0
            text: l10n.i18n("%1%", Math.round(compact.leftPercent))
            color: snapshot.dimmed ? Kirigami.Theme.disabledTextColor : snapshot.toneColor(compact.used, Kirigami.Theme.textColor)
            Layout.fillHeight: true
            verticalAlignment: Text.AlignVCenter
        }
    }
}
