import { ROSE_SVG } from './components/PianoMap'
import { describe, look } from './time'
import { useLevel, useMuseum } from './useMuseum'

// The projector window, a first version: the tuned piano's rose, swelling with the loudness of
// what plays on the speakers. The same rose styles as the map markers: blooming while tuned in,
// turning while playing, coloured by the piano's state.
export function Projector() {
    const view = useMuseum()
    const level = useLevel()
    const piano = view?.pianos.find((p) => p.id === view.tuned)
    const playing = view?.status === 'playing'
    const swell = 1 + Math.min(level * 2.5, 0.8)

    return (
        <main className="flex h-dvh flex-col items-center justify-center gap-[8vmin] overflow-hidden bg-black text-white">
            <div className="transition-transform duration-100 ease-out" style={{ transform: `scale(${swell})` }}>
                <div
                    className="rose-marker pointer-events-none"
                    style={{ width: '28vmin', height: '28vmin' }}
                    data-state={piano && view ? look(piano, view.now) : 'idle'}
                    data-tuned={piano ? '' : undefined}
                    data-playing={playing ? '' : undefined}
                    dangerouslySetInnerHTML={{ __html: ROSE_SVG }}
                />
            </div>
            <div className="text-center">
                <h1 className="text-[5vmin] font-bold">{piano ? piano.name : 'Rhapsody Rosarium'}</h1>
                <p className="text-[2.5vmin] text-white/60">
                    {piano && view ? (playing ? 'playing now' : describe(piano, view.now)) : 'Tune in to the city.'}
                </p>
            </div>
        </main>
    )
}
