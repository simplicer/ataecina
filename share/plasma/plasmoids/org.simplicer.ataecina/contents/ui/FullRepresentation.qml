/*
 * Popup form: one rounded card with a row per subscription
 * and a footer with when the snapshot was taken, "refresh now" and the
 * settings (SPECS §8.3).
 */
import QtQuick
import QtQuick.Controls as QQC2
import QtQuick.Layouts
import org.kde.plasma.components as PlasmaComponents3
import org.kde.plasma.extras as PlasmaExtras
import org.kde.kirigami as Kirigami

PlasmaExtras.Representation {
    id: full

    readonly property color ink: Kirigami.Theme.textColor

    // A rounded, faintly tinted card.
    component Card: Rectangle {
        readonly property color ink: Kirigami.Theme.textColor
        radius: Kirigami.Units.gridUnit * 0.75
        color: Qt.rgba(ink.r, ink.g, ink.b, 0.04)
        border.color: Qt.rgba(ink.r, ink.g, ink.b, 0.08)
    }

    Layout.minimumWidth: Kirigami.Units.gridUnit * 20
    Layout.minimumHeight: Kirigami.Units.gridUnit * 10
    Layout.preferredWidth: Kirigami.Units.gridUnit * 23
    Layout.preferredHeight: Math.min(Kirigami.Units.gridUnit * 34, content.implicitHeight + Kirigami.Units.gridUnit * 5)

    footer: PlasmaExtras.PlasmoidHeading {
        position: QQC2.ToolBar.Footer

        contentItem: RowLayout {
            PlasmaComponents3.Label {
                text: snapshot.meta.snapshot_taken_at ? i18n("Checked %1", snapshot.clock(snapshot.meta.snapshot_taken_at)) : ""
                opacity: 0.7
                Layout.fillWidth: true
            }
            PlasmaComponents3.ToolButton {
                icon.name: "view-refresh"
                display: QQC2.AbstractButton.IconOnly
                text: i18n("Refresh now")
                onClicked: root.refresh()
                PlasmaComponents3.ToolTip {
                    text: i18n("Refresh now")
                }
            }
            PlasmaComponents3.ToolButton {
                icon.name: "configure"
                display: QQC2.AbstractButton.IconOnly
                text: i18n("Configure Ataecina…")
                onClicked: root.configure()
                PlasmaComponents3.ToolTip {
                    text: i18n("Configure Ataecina…")
                }
            }
        }
    }

    PlasmaComponents3.ScrollView {
        id: scroll
        anchors.fill: parent
        visible: snapshot.problem === "" && (snapshot.listed.length > 0 || snapshot.warnings.length > 0)
        contentWidth: availableWidth

        ColumnLayout {
            id: content
            width: scroll.availableWidth
            spacing: Kirigami.Units.largeSpacing

            // What is left, one row per subscription.
            Card {
                id: card
                visible: snapshot.listed.length > 0
                Layout.fillWidth: true
                implicitHeight: rows.implicitHeight + Kirigami.Units.largeSpacing * 2

                ColumnLayout {
                    id: rows
                    anchors.fill: parent
                    anchors.margins: Kirigami.Units.largeSpacing
                    spacing: Kirigami.Units.largeSpacing

                    Repeater {
                        model: snapshot.listed

                        ColumnLayout {
                            id: entry
                            required property var modelData
                            required property int index
                            Layout.fillWidth: true
                            spacing: Kirigami.Units.largeSpacing

                            Rectangle {
                                visible: entry.index > 0
                                Layout.fillWidth: true
                                Layout.preferredHeight: 1
                                color: Qt.rgba(full.ink.r, full.ink.g, full.ink.b, 0.08)
                            }
                            SubscriptionRow {
                                modelData: entry.modelData
                                Layout.fillWidth: true
                            }
                        }
                    }
                }
            }

            // What failed or asks for something, folded.
            Card {
                visible: snapshot.warnings.length > 0
                Layout.fillWidth: true
                implicitHeight: warnings.implicitHeight + Kirigami.Units.largeSpacing * 2

                Warnings {
                    id: warnings
                    anchors.fill: parent
                    anchors.margins: Kirigami.Units.largeSpacing
                }
            }
        }
    }

    PlasmaExtras.PlaceholderMessage {
        anchors.centerIn: parent
        width: parent.width - Kirigami.Units.gridUnit * 4
        visible: !scroll.visible
        iconName: snapshot.problem !== "" ? "dialog-warning" : "ataecina"
        text: snapshot.problem !== "" ? snapshot.problem : i18n("No snapshot yet")
        explanation: snapshot.problem !== "" ? "" : i18n("Run `ataecina collect`, or enable ataecina-collect.timer.")
    }
}
