import QtQuick
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami
import org.kde.kcmutils as KCM
import QtQuick.Layouts

KCM.SimpleKCM {
    id: page

    property alias cfg_command: command.text
    property alias cfg_language: language.value
    property alias cfg_refreshSeconds: refresh.value
    property alias cfg_showPercentage: percentage.checked
    property alias cfg_subscriptions: subscriptions.text

    // KDE's own i18n, which the Translator cannot reach from inside.
    function desktopI18n(text, ...args) {
        return i18n(text, ...args);
    }

    function desktopI18np(singular, plural, count, ...args) {
        return i18np(singular, plural, count, ...args);
    }

    // Shows the page in the language being picked.
    Translator {
        id: l10n
        language: language.value
        desktop: page.desktopI18n
        desktopPlural: page.desktopI18np
    }

    Kirigami.FormLayout {
        QQC2.ComboBox {
            id: language
            // "" follows the desktop.
            property string value: ""
            Kirigami.FormData.label: l10n.i18n("Language:")
            textRole: "text"
            valueRole: "value"
            model: [
                { value: "", text: l10n.i18n("The desktop's") },
                { value: "es", text: "Español" },
                { value: "ca", text: "Català" },
                { value: "eu", text: "Euskara" },
                { value: "gl", text: "Galego" },
                { value: "en", text: "English" }
            ]
            currentIndex: Math.max(0, indexOfValue(value))
            onActivated: value = currentValue
        }
        QQC2.Label {
            text: l10n.i18n("Desktop notifications follow `language` in config.json, or the desktop.")
            font: Kirigami.Theme.smallFont
            opacity: 0.7
            wrapMode: Text.Wrap
            Layout.maximumWidth: Kirigami.Units.gridUnit * 22
        }
        QQC2.TextField {
            id: command
            Kirigami.FormData.label: l10n.i18n("Command:")
            placeholderText: "ataecina"
        }
        QQC2.SpinBox {
            id: refresh
            Kirigami.FormData.label: l10n.i18n("Read the snapshot every (seconds):")
            from: 10
            to: 3600
        }
        QQC2.CheckBox {
            id: percentage
            text: l10n.i18n("Show what is left in the panel")
        }
        QQC2.TextField {
            id: subscriptions
            Kirigami.FormData.label: l10n.i18n("Subscriptions to show:")
            placeholderText: l10n.i18n("all, in configuration order")
        }
        QQC2.Label {
            text: l10n.i18n("Comma-separated ids, in the order to show them.")
            font: Kirigami.Theme.smallFont
            opacity: 0.7
        }
    }
}
