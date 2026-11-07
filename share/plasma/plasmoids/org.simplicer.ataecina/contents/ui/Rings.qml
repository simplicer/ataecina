/*
 * Concentric growth rings, outer ring first: each one shows what is left
 * of a window (SPECS §8.3). Grey when the reading cannot be trusted.
 */
import QtQuick

Canvas {
    id: rings

    // Parallel lists, outer ring first: what is left (0 to 1) and its colour.
    property var fractions: []
    property var colors: []
    property bool muted: false
    property color mutedColor: "grey"
    // How many rings the empty state draws.
    property int emptyRings: 1

    onFractionsChanged: requestPaint()
    onColorsChanged: requestPaint()
    onMutedChanged: requestPaint()
    onMutedColorChanged: requestPaint()
    onWidthChanged: requestPaint()
    onHeightChanged: requestPaint()

    onPaint: {
        const context = getContext("2d");
        context.reset();
        const count = Math.max(fractions.length, emptyRings);
        const size = Math.min(width, height);
        const thickness = Math.max(2, size / (4 + count * 2.5));
        const gap = thickness * 0.5;
        const top = -Math.PI / 2;
        context.lineCap = "round";
        context.lineWidth = thickness;
        for (let index = 0; index < count; index++) {
            const radius = size / 2 - thickness / 2 - index * (thickness + gap);
            if (radius <= thickness) {
                break;
            }
            const color = muted || index >= colors.length ? mutedColor : colors[index];
            const fraction = index < fractions.length ? Math.max(0, Math.min(1, fractions[index])) : 0;
            context.strokeStyle = color;
            context.globalAlpha = 0.2;
            context.beginPath();
            context.arc(width / 2, height / 2, radius, 0, 2 * Math.PI);
            context.stroke();
            if (fraction > 0) {
                context.globalAlpha = 1;
                context.beginPath();
                context.arc(width / 2, height / 2, radius, top, top + 2 * Math.PI * fraction);
                context.stroke();
            }
        }
        context.globalAlpha = 1;
    }
}
