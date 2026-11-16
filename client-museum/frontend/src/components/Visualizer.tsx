import { Events } from '@wailsio/runtime'
import { useEffect, useRef } from 'react'

// A rose whose petals follow the spectrum of what plays on the speakers. Breathes quietly when silent.
// The web app's rose, drawn from the Go player's "spectrum" events instead of a browser analyser,
// since here the audio doesn't go through the web view.
export function Visualizer() {
    const ref = useRef<HTMLCanvasElement>(null)

    useEffect(() => {
        const canvas = ref.current!
        const g = canvas.getContext('2d')!
        // the latest bands from Go (~20 a second), and what's drawn, eased towards them every frame
        // like an analyser's smoothing
        let target: number[] = []
        let bands: number[] = []
        const off = Events.On('spectrum', (event) => {
            target = event.data as number[]
        })
        let raf = 0

        const draw = (now: number) => {
            raf = requestAnimationFrame(draw)
            const dpr = window.devicePixelRatio || 1
            const w = Math.round(canvas.clientWidth * dpr)
            const h = Math.round(canvas.clientHeight * dpr)
            if (canvas.width !== w || canvas.height !== h) {
                canvas.width = w
                canvas.height = h
            }

            if (bands.length !== target.length) bands = target.map(() => 0)
            let level = 0
            for (let i = 0; i < bands.length; i++) {
                bands[i] += (target[i] - bands[i]) * 0.2
                level += bands[i]
            }
            if (bands.length) level /= bands.length
            const energy = (f: number) =>
                bands.length ? bands[Math.min(bands.length - 1, Math.floor(f * bands.length))] : 0

            // fade the previous frame instead of clearing, leaving trails
            g.globalCompositeOperation = 'destination-out'
            g.fillStyle = 'rgba(0, 0, 0, 0.14)'
            g.fillRect(0, 0, w, h)
            g.globalCompositeOperation = 'source-over'

            const R = Math.min(w, h) * 0.17
            const breathe = 1 + 0.04 * Math.sin(now / 700)
            const hue = 340 + level * 50
            for (const [petals, scale, spin] of [
                [5, 1, 1],
                [7, 0.6, -1.6],
            ]) {
                g.beginPath()
                for (let i = 0; i <= 360; i++) {
                    const th = (i / 360) * Math.PI * 2
                    const f = Math.abs(Math.sin((th * petals) / 2))
                    const r = R * scale * breathe * (0.55 + 0.45 * f) * (1 + 1.6 * energy(f))
                    const a = th + (spin * now) / 9000
                    const x = w / 2 + r * Math.cos(a)
                    const y = h / 2 + r * Math.sin(a)
                    if (i === 0) g.moveTo(x, y)
                    else g.lineTo(x, y)
                }
                g.closePath()
                g.fillStyle = `hsl(${hue} 75% 58% / 0.08)`
                g.strokeStyle = `hsl(${hue} 75% 58% / ${0.5 + level})`
                g.lineWidth = 1.5 * dpr
                g.fill()
                g.stroke()
            }
        }

        raf = requestAnimationFrame(draw)
        return () => {
            cancelAnimationFrame(raf)
            off()
        }
    }, [])

    return <canvas ref={ref} className="aspect-square w-full max-w-[280px]" aria-hidden="true" />
}
