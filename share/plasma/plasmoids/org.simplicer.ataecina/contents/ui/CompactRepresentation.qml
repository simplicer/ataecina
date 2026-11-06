/*
 * Panel form: a ring around the Ataecina icon showing the worst meter,
 * optionally with its percentage (SPECS §8.3).
 */
import QtQuick
import QtQuick.Layouts
import org.kde.plasma.plasmoid
import org.kde.plasma.components as PlasmaComponents3
import org.kde.kirigami as Kirigami

MouseArea {
    id: compact

    readonly property real percent: root.worstPercent
    readonly property color ringColor: root.severityColor

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

    onPercentChanged: ring.requestPaint()
    onRingColorChanged: ring.requestPaint()

    RowLayout {
        id: row
        anchors.fill: parent
        spacing: Kirigami.Units.smallSpacing

        Item {
            Layout.fillHeight: true
            Layout.preferredWidth: height

            Kirigami.Icon {
                anchors.fill: parent
                anchors.margins: Math.round(parent.height * 0.2)
                source: Plasmoid.icon
                active: compact.containsMouse
            }

            Canvas {
                id: ring
                anchors.fill: parent
                onPaint: {
                    const context = getContext("2d");
                    context.reset();
                    const thickness = Math.max(2, Math.round(ring.height / 10));
                    const radius = Math.min(ring.width, ring.height) / 2 - thickness / 2;
                    const start = -Math.PI / 2;
                    context.lineWidth = thickness;
                    context.lineCap = "round";
                    context.strokeStyle = Qt.rgba(compact.ringColor.r, compact.ringColor.g, compact.ringColor.b, 0.25);
                    context.beginPath();
                    context.arc(ring.width / 2, ring.height / 2, radius, 0, 2 * Math.PI);
                    context.stroke();
                    if (compact.percent > 0) {
                        context.strokeStyle = compact.ringColor;
                        context.beginPath();
                        context.arc(ring.width / 2, ring.height / 2, radius, start, start + 2 * Math.PI * Math.min(1, compact.percent / 100));
                        context.stroke();
                    }
                }
            }
        }

        PlasmaComponents3.Label {
            visible: Plasmoid.configuration.showPercentage && compact.percent >= 0
            text: i18n("%1 %", Math.round(compact.percent))
            color: root.dimmed ? Kirigami.Theme.disabledTextColor : root.percentColor(compact.percent)
            Layout.fillHeight: true
            verticalAlignment: Text.AlignVCenter
        }
    }
}
