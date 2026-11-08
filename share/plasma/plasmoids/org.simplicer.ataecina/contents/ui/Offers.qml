/*
 * Coding plan offers the maintainer publishes (SPECS §12, ADR 0017),
 * folded under one line: provider and plan, the offer, its price and
 * period, "sponsored" for an affiliate link. A click opens the link in the
 * browser; the widget fetches nothing itself (`ataecina offers --json`).
 */
import QtQuick
import QtQuick.Layouts
import org.kde.plasma.components as PlasmaComponents3
import org.kde.kirigami as Kirigami

ColumnLayout {
    id: block

    property bool unfolded: false

    objectName: "offers"
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
                source: "bookmark-new"
                Layout.preferredWidth: Kirigami.Units.iconSizes.small
                Layout.preferredHeight: Kirigami.Units.iconSizes.small
            }
            PlasmaComponents3.Label {
                text: i18np("%1 offer", "%1 offers", snapshot.offers.length)
                font.weight: Font.DemiBold
                Layout.fillWidth: true
            }
            PlasmaComponents3.Label {
                visible: !block.unfolded
                text: snapshot.offers.map(o => o.provider).filter((p, i, all) => all.indexOf(p) === i).join(", ")
                textFormat: Text.PlainText
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
        model: block.unfolded ? snapshot.offers : []

        MouseArea {
            id: offer
            required property var modelData
            Layout.fillWidth: true
            Layout.leftMargin: Kirigami.Units.iconSizes.small + Kirigami.Units.smallSpacing
            Layout.preferredHeight: lines.implicitHeight
            cursorShape: Qt.PointingHandCursor
            hoverEnabled: true
            onClicked: Qt.openUrlExternally(modelData.url)

            ColumnLayout {
                id: lines
                anchors.fill: parent
                spacing: 0

                RowLayout {
                    Layout.fillWidth: true

                    PlasmaComponents3.Label {
                        text: offer.modelData.provider + " · " + offer.modelData.plan
                        textFormat: Text.PlainText
                        font.underline: offer.containsMouse
                        elide: Text.ElideRight
                        Layout.fillWidth: true
                    }
                    PlasmaComponents3.Label {
                        text: snapshot.offerPrice(offer.modelData)
                        textFormat: Text.PlainText
                    }
                }
                PlasmaComponents3.Label {
                    visible: text !== ""
                    text: offer.modelData.title
                    textFormat: Text.PlainText
                    font: Kirigami.Theme.smallFont
                    opacity: 0.7
                    wrapMode: Text.Wrap
                    Layout.fillWidth: true
                }
                PlasmaComponents3.Label {
                    visible: offer.modelData.sponsored
                    text: i18n("Sponsored link")
                    font: Kirigami.Theme.smallFont
                    opacity: 0.5
                }
            }
        }
    }
}
