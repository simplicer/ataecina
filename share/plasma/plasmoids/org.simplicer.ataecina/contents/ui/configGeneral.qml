import QtQuick
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami
import org.kde.kcmutils as KCM

KCM.SimpleKCM {
    property alias cfg_command: command.text
    property alias cfg_refreshSeconds: refresh.value
    property alias cfg_showPercentage: percentage.checked
    property alias cfg_subscriptions: subscriptions.text

    Kirigami.FormLayout {
        QQC2.TextField {
            id: command
            Kirigami.FormData.label: i18n("Command:")
            placeholderText: "ataecina"
        }
        QQC2.SpinBox {
            id: refresh
            Kirigami.FormData.label: i18n("Read the snapshot every (seconds):")
            from: 10
            to: 3600
        }
        QQC2.CheckBox {
            id: percentage
            text: i18n("Show the percentage in the panel")
        }
        QQC2.TextField {
            id: subscriptions
            Kirigami.FormData.label: i18n("Subscriptions to show:")
            placeholderText: i18n("all, in configuration order")
        }
        QQC2.Label {
            text: i18n("Comma-separated ids, in the order to show them.")
            font: Kirigami.Theme.smallFont
            opacity: 0.7
        }
    }
}
