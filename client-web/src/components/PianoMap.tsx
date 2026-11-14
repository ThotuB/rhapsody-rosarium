import {
    LngLatBounds,
    Map as MapLibreMap,
    Marker,
    NavigationControl,
    setWorkerUrl,
    type LngLatBoundsLike,
} from 'maplibre-gl'
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'
import 'maplibre-gl/dist/maplibre-gl.css'
import { useEffect, useRef } from 'react'
import type { Piano } from '../api'
import { describe, look } from '../time'

// vite pre-bundles maplibre, which breaks its own lookup of the worker file next to it
setWorkerUrl(workerUrl)

const STYLE = 'https://tiles.openfreemap.org/styles/dark'

// A rose seen from above: two rings of petals around a heart. The petals start furled into a bud
// and open (see .rose-marker in index.css) while the piano is tuned in.
const PETAL = 'M0 0C-6.5-3-8.5-13 0-17.5C8.5-13 6.5-3 0 0Z'
const ring = (cls: string, count: number, offset: number, first: number) =>
    Array.from(
        { length: count },
        (_, i) =>
            `<path class="petal ${cls}" d="${PETAL}" style="--a:${offset + (i * 360) / count}deg;--i:${first + i}"/>`,
    ).join('')
const ROSE_SVG =
    '<svg viewBox="-20 -20 40 40" aria-hidden="true">' +
    ring('outer', 5, 0, 0) +
    ring('inner', 5, 36, 5) +
    '<circle class="heart" r="2.8"/>' +
    '</svg>'
const CITY_CENTER: [number, number] = [21.2257, 45.7537]
// Timișoara's administrative boundary box from OpenStreetMap (west-south, east-north).
// The map can't be panned past it or zoomed out further than it fits.
const CITY_BOUNDS: LngLatBoundsLike = [
    [21.1117, 45.6838],
    [21.3272, 45.8342],
]

type Props = {
    pianos: Piano[]
    now: string
    tunedId: string | null
    playing: boolean // the tuned piano is actually sounding, not waiting or blocked
    onTune: (id: string) => void
}

export function PianoMap({ pianos, now, tunedId, playing, onTune }: Props) {
    const container = useRef<HTMLDivElement>(null)
    const map = useRef<MapLibreMap | null>(null)
    const markers = useRef(new Map<string, Marker>())
    const fitted = useRef(false)
    const onTuneRef = useRef(onTune)

    useEffect(() => {
        onTuneRef.current = onTune
    })

    useEffect(() => {
        const m = new MapLibreMap({
            container: container.current!,
            style: STYLE,
            center: CITY_CENTER,
            zoom: 11,
            maxBounds: CITY_BOUNDS,
            renderWorldCopies: false,
            attributionControl: { compact: true },
        })
        m.addControl(new NavigationControl({ showCompass: false }), 'top-right')
        map.current = m
        const ms = markers.current
        return () => {
            m.remove()
            map.current = null
            ms.clear()
            fitted.current = false
        }
    }, [])

    // markers are created once per piano and updated in place on every poll (`now` changes each time)
    useEffect(() => {
        const m = map.current
        if (!m) return
        const seen = new Set<string>()
        for (const p of pianos) {
            seen.add(p.id)
            let marker = markers.current.get(p.id)
            if (!marker) {
                // maplibre positions this element with a transform, so no transform utilities on it
                const el = document.createElement('button')
                el.type = 'button'
                el.className = 'rose-marker group'
                el.innerHTML =
                    `<span class="rose-halo"></span>${ROSE_SVG}` +
                    // the name only shows on hover, or keyboard focus; screen readers get the aria-label
                    '<span class="label pointer-events-none absolute top-full left-1/2 -translate-x-1/2 opacity-0 transition-[top,opacity] duration-300 group-hover:opacity-100 group-focus-visible:opacity-100 group-data-tuned:top-[calc(100%+22px)] rounded-lg bg-white px-2 py-px text-xs whitespace-nowrap text-[#1d1a1f] shadow-soft"></span>'
                el.addEventListener('click', (e) => {
                    e.stopPropagation()
                    onTuneRef.current(p.id)
                })
                marker = new Marker({ element: el }).setLngLat([p.lng, p.lat]).addTo(m)
                markers.current.set(p.id, marker)
            }
            marker.setLngLat([p.lng, p.lat])
            const el = marker.getElement()
            el.dataset.state = look(p, now)
            el.toggleAttribute('data-tuned', p.id === tunedId)
            el.toggleAttribute('data-playing', p.id === tunedId && playing)
            el.querySelector('.label')!.textContent = p.name
            el.setAttribute('aria-label', `${p.name}, ${describe(p, now)}`)
        }
        for (const [id, marker] of markers.current) {
            if (!seen.has(id)) {
                marker.remove()
                markers.current.delete(id)
            }
        }
        if (!fitted.current && pianos.length) {
            fitted.current = true
            const bounds = new LngLatBounds()
            for (const p of pianos) bounds.extend([p.lng, p.lat])
            m.fitBounds(bounds, { padding: 80, maxZoom: 16, duration: 0 })
        }
    }, [pianos, now, tunedId, playing])

    return <div ref={container} className="relative min-h-0" />
}
