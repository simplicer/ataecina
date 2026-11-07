/*
 * Subscriptions whose source failed, or that ask for something (declare a
 * balance), folded under one line instead of mixed with what is left
 * (SPECS §8.3). The collector also notifies once when a source starts
 * failing.
 */
import QtQuick
import QtQuick.Layouts
import org.kde.plasma.components as PlasmaComponents3
import org.kde.kirigami as Kirigami

ColumnLayout {
    id: block

    property bool unfolded: false

    objectName: "warnings"
    spacing: Kirigami.Units.smallSpacing

    MouseArea {
        Layout.fillWidth: true
        Layout.preferredHeight: heading.implicitHeight
        cursorShape: Qt.PointingHandCursor
        onClicked: block.unfolded = !block.unfolded

        RowLayout {
            id: heading
            anchors.fill: parent
            spacing: Kirigami.Units.smallSpacing

            Kirigami.Icon {
                source: "dialog-warning"
                Layout.preferredWidth: Kirigami.Units.iconSizes.small
                Layout.preferredHeight: Kirigami.Units.iconSizes.small
            }
            PlasmaComponents3.Label {
                text: i18np("%1 warning", "%1 warnings", snapshot.warnings.length)
                font.weight: Font.DemiBold
                Layout.fillWidth: true
            }
            PlasmaComponents3.Label {
                visible: !block.unfolded
                text: snapshot.warnings.map(s => snapshot.name(s)).join(", ")
                opacity: 0.6
                elide: Text.ElideRight
                Layout.maximumWidth: Kirigami.Units.gridUnit * 12
            }
            Kirigami.Icon {
                source: block.unfolded ? "arrow-up" : "arrow-down"
                Layout.preferredWidth: Kirigami.Units.iconSizes.small
                Layout.preferredHeight: Kirigami.Units.iconSizes.small
                opacity: 0.4
            }
        }
    }

    Repeater {
        model: block.unfolded ? snapshot.warnings : []

        ColumnLayout {
            required property var modelData
            Layout.fillWidth: true
            Layout.leftMargin: Kirigami.Units.iconSizes.small + Kirigami.Units.smallSpacing
            spacing: 0

            PlasmaComponents3.Label {
                text: snapshot.name(modelData)
            }
            PlasmaComponents3.Label {
                text: snapshot.warningText(modelData)
                font: Kirigami.Theme.smallFont
                opacity: 0.7
                wrapMode: Text.Wrap
                Layout.fillWidth: true
            }
        }
    }
}
