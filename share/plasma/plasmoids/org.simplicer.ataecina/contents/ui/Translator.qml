/*
 * The widget's language. KDE's i18n always speaks the desktop's language,
 * so a language picked in the settings (Spanish, Catalan, Basque, Galician
 * or English) is read from translations.js, generated from the PO files
 * (`go run ./cmd/widget-translations`). With no language picked, the
 * desktop's own i18n, handed over by the owner of this object, is used.
 * `locale` formats money, numbers and times for the same language.
 */
import QtQuick
import "translations.js" as Catalogue

QtObject {
    // "" follows the desktop; es, ca, eu, gl or en picks one.
    property string language: ""
    // The desktop's i18n and i18np (they cannot be reached from here,
    // where those names are this object's own).
    property var desktop: null
    property var desktopPlural: null

    readonly property var messages: language !== "" && language !== "en" ? (Catalogue.table[language] || {}) : null
    readonly property var locale: language === "" ? Qt.locale() : Qt.locale(language === "en" ? "en_US" : language + "_ES")

    function fill(text, args) {
        let filled = text;
        for (let i = 0; i < args.length; i++) {
            filled = filled.split("%" + (i + 1)).join(String(args[i]));
        }
        return filled;
    }

    function i18n(text, ...args) {
        if (language === "" && desktop) {
            return desktop(text, ...args);
        }
        const translated = messages ? messages[text] : undefined;
        return fill(typeof translated === "string" ? translated : text, args);
    }

    function i18np(singular, plural, count, ...args) {
        if (language === "" && desktopPlural) {
            return desktopPlural(singular, plural, count, ...args);
        }
        const forms = messages ? messages[singular] : undefined;
        const one = count === 1;
        const text = Array.isArray(forms) ? forms[one ? 0 : 1] : (one ? singular : plural);
        return fill(text, [count].concat(args));
    }
}
