/*
 * One subscription in the popup: growth rings, the name and plan, when the
 * closest window is reborn, and what is left in large figures. A click
 * unfolds every window, the balance and the hint (SPECS §8.3).
 */
import QtQuick
import QtQuick.Layouts
import org.kde.plasma.components as PlasmaComponents3
import org.kde.kirigami as Kirigami

ColumnLayout {
    id: row

    required property var modelData
    readonly property var subscription: modelData
    readonly property bool lively: snapshot.lively(subscription)
    readonly property color headlineColor: figureColor(snapshot.headlineUsed(subscription))
    property bool unfolded: false

    objectName: "row-" + subscription.id
    spacing: Kirigami.Units.smallSpacing

    function figureColor(usedPercent) {
        if (!lively) {
            return Kirigami.Theme.disabledTextColor;
        }
        return snapshot.toneColor(usedPercent, Kirigami.Theme.textColor);
    }

    function badgeColor() {
        if (snapshot.failed(subscription)) {
            return Kirigami.Theme.negativeTextColor;
        }
        if (!lively) {
            return Kirigami.Theme.disabledTextColor;
        }
        return snapshot.toneColor(snapshot.headlineUsed(subscription), snapshot.growth[0]);
    }

    MouseArea {
        Layout.fillWidth: true
        Layout.preferredHeight: summary.implicitHeight
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: row.unfolded = !row.unfolded

        RowLayout {
            id: summary
            anchors.fill: parent
            spacing: Kirigami.Units.largeSpacing

            Rings {
                Layout.preferredWidth: Kirigami.Units.gridUnit * 2.4
                Layout.preferredHeight: Kirigami.Units.gridUnit * 2.4
                Layout.alignment: Qt.AlignVCenter
                fractions: snapshot.ringFractions(row.subscription)
                colors: snapshot.ringColors(row.subscription)
                muted: !row.lively
                mutedColor: Kirigami.Theme.disabledTextColor

                PlasmaComponents3.Label {
                    anchors.centerIn: parent
                    text: snapshot.badge(row.subscription)
                    font.weight: Font.Bold
                    color: row.badgeColor()
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                spacing: 0

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Kirigami.Units.smallSpacing

                    PlasmaComponents3.Label {
                        text: snapshot.name(row.subscription)
                        font.weight: Font.DemiBold
                        font.pointSize: Kirigami.Theme.defaultFont.pointSize * 1.15
                    }
                    PlasmaComponents3.Label {
                        text: snapshot.plan(row.subscription)
                        opacity: 0.6
                        elide: Text.ElideRight
                        Layout.fillWidth: true
                    }
                }
                PlasmaComponents3.Label {
                    text: snapshot.subtitle(row.subscription)
                    opacity: 0.7
                    elide: Text.ElideRight
                    Layout.fillWidth: true
                }
            }

            RowLayout {
                spacing: 0
                Layout.alignment: Qt.AlignVCenter

                PlasmaComponents3.Label {
                    visible: text !== ""
                    text: snapshot.approximate(row.subscription)
                    opacity: 0.6
                }
                PlasmaComponents3.Label {
                    text: snapshot.headline(row.subscription)
                    color: row.headlineColor
                    font.weight: Font.DemiBold
                    font.pointSize: Kirigami.Theme.defaultFont.pointSize * (snapshot.headlineIsPercent(row.subscription) ? 1.8 : 1.4)
                }
                PlasmaComponents3.Label {
                    visible: text !== ""
                    text: snapshot.headlineUnit(row.subscription)
                    color: row.headlineColor
                    opacity: 0.7
                    Layout.alignment: Qt.AlignBottom
                    Layout.leftMargin: text === "%" ? 0 : Kirigami.Units.smallSpacing
                    Layout.bottomMargin: Kirigami.Units.smallSpacing / 2
                }
            }

            Kirigami.Icon {
                source: row.unfolded ? "arrow-up" : "arrow-down"
                Layout.preferredWidth: Kirigami.Units.iconSizes.small
                Layout.preferredHeight: Kirigami.Units.iconSizes.small
                opacity: 0.4
            }
        }
    }

    // The unfolded detail: every window, then the balance and the hint.
    ColumnLayout {
        visible: row.unfolded
        Layout.fillWidth: true
        Layout.leftMargin: Kirigami.Units.gridUnit * 2.4 + Kirigami.Units.largeSpacing
        spacing: Kirigami.Units.smallSpacing

        Repeater {
            model: row.subscription.meters

            RowLayout {
                required property var modelData
                Layout.fillWidth: true
                spacing: Kirigami.Units.smallSpacing

                PlasmaComponents3.Label {
                    text: snapshot.meterLabel(row.subscription, modelData)
                    opacity: 0.7
                    elide: Text.ElideRight
                    Layout.preferredWidth: Kirigami.Units.gridUnit * 4
                }
                Rectangle {
                    Layout.fillWidth: true
                    Layout.preferredHeight: Kirigami.Units.smallSpacing * 1.5
                    radius: height / 2
                    color: Qt.rgba(Kirigami.Theme.textColor.r, Kirigami.Theme.textColor.g, Kirigami.Theme.textColor.b, 0.1)

                    Rectangle {
                        width: parent.width * snapshot.leftOf(modelData) / 100
                        height: parent.height
                        radius: parent.radius
                        color: row.lively ? snapshot.toneColor(modelData.used_percent, snapshot.growth[0]) : Kirigami.Theme.disabledTextColor
                    }
                }
                PlasmaComponents3.Label {
                    text: modelData.estimated ? i18n("reborn") : i18n("%1 % left", Math.round(snapshot.leftOf(modelData)))
                    color: row.figureColor(modelData.used_percent)
                    horizontalAlignment: Text.AlignRight
                    Layout.preferredWidth: Kirigami.Units.gridUnit * 4
                }
                PlasmaComponents3.Label {
                    text: snapshot.countdown(modelData.resets_at)
                    opacity: 0.6
                    Layout.preferredWidth: Kirigami.Units.gridUnit * 4
                }
            }
        }

        PlasmaComponents3.Label {
            visible: text !== ""
            text: snapshot.balanceText(row.subscription)
            Layout.fillWidth: true
            wrapMode: Text.Wrap
        }
        PlasmaComponents3.Label {
            visible: text !== ""
            text: [snapshot.fidelityText(row.subscription), snapshot.outcomeText(row.subscription)].filter(s => s !== "").join(" · ")
            font: Kirigami.Theme.smallFont
            opacity: 0.6
        }
        PlasmaComponents3.Label {
            visible: text !== ""
            text: row.subscription.hint || ""
            font: Kirigami.Theme.smallFont
            opacity: 0.7
            Layout.fillWidth: true
            wrapMode: Text.Wrap
        }
        PlasmaComponents3.Label {
            visible: row.subscription.observed_at !== null
            text: i18n("observed %1", snapshot.age(row.subscription.observed_at))
            font: Kirigami.Theme.smallFont
            opacity: 0.5
        }
    }
}
