/*
 * One subscription in the popup: name and plan, badges, one bar per
 * window, the balance and when it was observed.
 */
import QtQuick
import QtQuick.Layouts
import org.kde.plasma.components as PlasmaComponents3
import org.kde.kirigami as Kirigami

ColumnLayout {
    id: delegate

    required property var modelData
    readonly property var subscription: modelData

    spacing: Kirigami.Units.smallSpacing

    RowLayout {
        Layout.fillWidth: true

        PlasmaComponents3.Label {
            text: delegate.subscription.plan ? delegate.subscription.id + " · " + delegate.subscription.plan : delegate.subscription.id
            font.weight: Font.DemiBold
            Layout.fillWidth: true
            elide: Text.ElideRight
        }
        PlasmaComponents3.Label {
            visible: text !== ""
            text: root.fidelityText(delegate.subscription)
            font: Kirigami.Theme.smallFont
            opacity: 0.7
        }
        PlasmaComponents3.Label {
            visible: text !== ""
            text: root.outcomeText(delegate.subscription)
            color: delegate.subscription.outcome === "PARTIAL" || delegate.subscription.outcome === "UNSUPPORTED"
                ? Kirigami.Theme.disabledTextColor : Kirigami.Theme.negativeTextColor
            font: Kirigami.Theme.smallFont
        }
    }

    Repeater {
        model: delegate.subscription.meters

        RowLayout {
            required property var modelData
            Layout.fillWidth: true
            spacing: Kirigami.Units.smallSpacing

            PlasmaComponents3.Label {
                text: root.meterLabel(delegate.subscription, modelData)
                Layout.preferredWidth: Kirigami.Units.gridUnit * 4
                elide: Text.ElideRight
            }
            PlasmaComponents3.ProgressBar {
                from: 0
                to: 100
                value: Math.min(100, modelData.used_percent)
                Layout.fillWidth: true
                opacity: delegate.subscription.freshness === "FRESH" ? 1 : 0.5
            }
            PlasmaComponents3.Label {
                text: modelData.estimated ? i18n("0 % (reborn)") : i18n("%1 %", Math.round(modelData.used_percent))
                color: root.percentColor(modelData.used_percent)
                horizontalAlignment: Text.AlignRight
                Layout.preferredWidth: Kirigami.Units.gridUnit * 3
            }
            PlasmaComponents3.Label {
                text: root.countdown(modelData.resets_at)
                opacity: 0.7
                Layout.preferredWidth: Kirigami.Units.gridUnit * 5
            }
        }
    }

    PlasmaComponents3.Label {
        visible: text !== ""
        text: root.balanceText(delegate.subscription)
        Layout.fillWidth: true
        wrapMode: Text.Wrap
    }

    PlasmaComponents3.Label {
        visible: text !== ""
        text: delegate.subscription.hint || ""
        font: Kirigami.Theme.smallFont
        opacity: 0.7
        Layout.fillWidth: true
        wrapMode: Text.Wrap
    }

    PlasmaComponents3.Label {
        visible: delegate.subscription.observed_at !== null && delegate.subscription.freshness !== "FRESH"
        text: i18n("stale") + " · " + i18n("observed %1", root.age(delegate.subscription.observed_at))
        font: Kirigami.Theme.smallFont
        color: Kirigami.Theme.disabledTextColor
    }
}
