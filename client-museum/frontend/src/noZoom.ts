// The museum's windows run on an installation screen, so visitors mustn't zoom the page itself:
// not with a pinch on a touchscreen or touchpad, Ctrl + scroll, or Ctrl + plus/minus. The map keeps
// its own zoom, it handles those events before they get here.
export function preventPageZoom() {
    // Ctrl + scroll, and touchpad pinches (which arrive as wheel events with ctrlKey set)
    window.addEventListener(
        'wheel',
        (e) => {
            if (e.ctrlKey) e.preventDefault()
        },
        { passive: false },
    )
    // Ctrl/Cmd + plus, minus, equals or 0
    window.addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && ['+', '-', '=', '0'].includes(e.key)) e.preventDefault()
    })
    // webkit's own pinch gestures
    for (const type of ['gesturestart', 'gesturechange', 'gestureend']) {
        window.addEventListener(type, (e) => e.preventDefault(), { passive: false })
    }
    // two finger pinches on a touchscreen, outside the map
    window.addEventListener(
        'touchmove',
        (e) => {
            const onMap = e.target instanceof Element && e.target.closest('.maplibregl-map')
            if (e.touches.length > 1 && !onMap) e.preventDefault()
        },
        { passive: false },
    )
}
