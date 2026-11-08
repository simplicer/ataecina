/*
 * One subscription in the popup: growth rings for what is known, the name
 * and plan, the money left of its total, and what is left of every window
 * (5 h, week, month) with its rebirth. A click unfolds every bucket, the
 * balance and the hint (SPECS §8.3).
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
    readonly property color moneyColor: figureColor(snapshot.moneyUsedPercent(subscription))
    property bool unfolded: false
    property bool showModels: false

    objectName: "row-" + subscription.id
    spacing: Kirigami.Units.smallSpacing

    function figureColor(usedPercent) {
        if (!lively) {
            return Kirigami.Theme.disabledTextColor;
        }
        return snapshot.toneColor(usedPercent, Kirigami.Theme.textColor);
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

            // Stale readings fade; unknown windows draw nothing.
            Rings {
                Layout.preferredWidth: Kirigami.Units.gridUnit * 2.4
                Layout.preferredHeight: Kirigami.Units.gridUnit * 2.4
                Layout.alignment: Qt.AlignTop
                fractions: snapshot.ringFractions(row.subscription)
                colors: snapshot.ringColors(row.subscription)
                emptyRings: 0
                muted: !row.lively
                mutedColor: Kirigami.Theme.disabledTextColor
                opacity: row.lively ? 1 : 0.5

                PlasmaComponents3.Label {
                    anchors.centerIn: parent
                    text: snapshot.currencyMark(row.subscription)
                    font.weight: Font.Bold
                    color: row.moneyColor
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                spacing: Kirigami.Units.smallSpacing / 2

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
                    PlasmaComponents3.Label {
                        visible: text !== ""
                        text: snapshot.moneyLine(row.subscription)
                        color: row.moneyColor
                        font.weight: Font.DemiBold
                    }
                    Kirigami.Icon {
                        source: row.unfolded ? "arrow-up" : "arrow-down"
                        Layout.preferredWidth: Kirigami.Units.iconSizes.small
                        Layout.preferredHeight: Kirigami.Units.iconSizes.small
                        opacity: 0.4
                    }
                }

                // What is left of every window, shortest first.
                RowLayout {
                    visible: windowColumns.count > 0
                    Layout.fillWidth: true
                    spacing: Kirigami.Units.gridUnit

                    Repeater {
                        id: windowColumns
                        model: snapshot.windows(row.subscription)

                        ColumnLayout {
                            required property var modelData
                            spacing: 0

                            PlasmaComponents3.Label {
                                text: snapshot.windowLabel(modelData)
                                font: Kirigami.Theme.smallFont
                                opacity: 0.6
                            }
                            RowLayout {
                                spacing: 0
                                // Basque writes the sign first ("% 76").
                                layoutDirection: snapshot.percentSignFirst ? Qt.RightToLeft : Qt.LeftToRight

                                PlasmaComponents3.Label {
                                    text: snapshot.leftText(modelData)
                                    color: row.figureColor(modelData.used_percent)
                                    font.weight: Font.DemiBold
                                    font.pointSize: Kirigami.Theme.defaultFont.pointSize * 1.45
                                }
                                PlasmaComponents3.Label {
                                    text: "%"
                                    color: row.figureColor(modelData.used_percent)
                                    opacity: 0.7
                                    Layout.alignment: Qt.AlignBottom
                                    Layout.bottomMargin: Kirigami.Units.smallSpacing / 2
                                }
                            }
                            PlasmaComponents3.Label {
                                text: snapshot.resetText(modelData)
                                font: Kirigami.Theme.smallFont
                                opacity: 0.6
                            }
                        }
                    }
                    Item {
                        Layout.fillWidth: true
                    }
                }

                PlasmaComponents3.Label {
                    visible: text !== ""
                    text: snapshot.note(row.subscription)
                    font: Kirigami.Theme.smallFont
                    opacity: 0.6
                    elide: Text.ElideRight
                    Layout.fillWidth: true
                }
            }
        }
    }

    // The unfolded detail: every bucket and window, then the balance and
    // the hint.
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
                    text: i18n("%1% left", snapshot.leftText(modelData))
                    color: row.figureColor(modelData.used_percent)
                    horizontalAlignment: Text.AlignRight
                    // Translations can be longer than "100 % left".
                    Layout.preferredWidth: Math.max(implicitWidth, Kirigami.Units.gridUnit * 4)
                }
                PlasmaComponents3.Label {
                    text: snapshot.resetText(modelData)
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
        // The model catalogue: names come from models.dev, so they are
        // only ever shown as plain text.
        PlasmaComponents3.Label {
            visible: text !== ""
            text: snapshot.modelsText(row.subscription)
            Layout.fillWidth: true
            wrapMode: Text.Wrap
        }
        PlasmaComponents3.Label {
            visible: text !== ""
            text: snapshot.freeModelsText(row.subscription)
            textFormat: Text.PlainText
            font: Kirigami.Theme.smallFont
            opacity: 0.8
            Layout.fillWidth: true
            wrapMode: Text.Wrap
        }
        PlasmaComponents3.Label {
            visible: text !== ""
            text: snapshot.newModelsText(row.subscription)
            textFormat: Text.PlainText
            font: Kirigami.Theme.smallFont
            opacity: 0.8
            Layout.fillWidth: true
            wrapMode: Text.Wrap
        }
        PlasmaComponents3.ToolButton {
            visible: !!row.subscription.offerings
            icon.name: row.showModels ? "go-up" : "go-down"
            text: row.showModels ? i18n("Hide models") : i18n("Models you can use")
            onClicked: {
                row.showModels = !row.showModels;
                if (row.showModels) {
                    root.loadModels(row.subscription.id);
                }
            }
        }
        // Every model the subscription can use with what is left, best
        // ranked first: name, intelligence index, price per 1M tokens.
        GridLayout {
            visible: row.showModels
            columns: 3
            columnSpacing: Kirigami.Units.largeSpacing
            rowSpacing: 0
            Layout.fillWidth: true

            Repeater {
                model: row.showModels ? snapshot.availableModels(row.subscription.id) : []

                delegate: PlasmaComponents3.Label {
                    required property var modelData
                    required property int index
                    text: modelData.name
                    textFormat: Text.PlainText
                    elide: Text.ElideRight
                    font: Kirigami.Theme.smallFont
                    Layout.row: index
                    Layout.column: 0
                    Layout.fillWidth: true
                }
            }
            Repeater {
                model: row.showModels ? snapshot.availableModels(row.subscription.id) : []

                delegate: PlasmaComponents3.Label {
                    required property var modelData
                    required property int index
                    text: snapshot.intelligenceText(modelData)
                    font: Kirigami.Theme.smallFont
                    horizontalAlignment: Text.AlignRight
                    Layout.row: index
                    Layout.column: 1
                }
            }
            Repeater {
                model: row.showModels ? snapshot.availableModels(row.subscription.id) : []

                delegate: PlasmaComponents3.Label {
                    required property var modelData
                    required property int index
                    text: snapshot.priceText(modelData)
                    font: Kirigami.Theme.smallFont
                    opacity: 0.7
                    Layout.row: index
                    Layout.column: 2
                }
            }
        }
        PlasmaComponents3.Label {
            visible: row.showModels && snapshot.moreModels(row.subscription.id) > 0
            text: i18np("%1 more", "%1 more", snapshot.moreModels(row.subscription.id))
            font: Kirigami.Theme.smallFont
            opacity: 0.6
        }
        RowLayout {
            visible: row.showModels
            Layout.fillWidth: true

            PlasmaComponents3.Label {
                // Attribution the Artificial Analysis API asks for.
                text: snapshot.intelligenceCredit()
                font: Kirigami.Theme.smallFont
                opacity: 0.6
                wrapMode: Text.Wrap
                Layout.fillWidth: true
            }
            PlasmaComponents3.ToolButton {
                icon.name: "internet-web-browser"
                display: PlasmaComponents3.AbstractButton.IconOnly
                text: "artificialanalysis.ai"
                onClicked: Qt.openUrlExternally("https://artificialanalysis.ai/")
            }
            PlasmaComponents3.ToolButton {
                enabled: !root.updating
                icon.name: "view-refresh"
                text: root.updating ? i18n("Updating…") : i18n("Update models")
                onClicked: root.updateModels()
            }
        }
        PlasmaComponents3.Label {
            visible: row.showModels && root.updateProblem !== ""
            text: root.updateProblem
            font: Kirigami.Theme.smallFont
            wrapMode: Text.Wrap
            Layout.fillWidth: true
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
