import { useEffect } from 'react'
import { Panel } from './components/Panel'
import { PianoMap } from './components/PianoMap'
import { radio, useRadio } from './radio'
import { playable } from './time'
import { usePianos } from './usePianos'

export default function App() {
    const { pianos, now, error } = usePianos()
    const tuned = useRadio()

    // keep the tuned station's segment list fresh
    useEffect(() => {
        const p = pianos.find((p) => p.id === tuned.pianoId)
        if (p) radio.update(p.id, playable(p, now))
    }, [pianos, now, tuned.pianoId])

    // called straight from click handlers, so the browser lets audio start
    const tune = (id: string) => {
        const p = pianos.find((p) => p.id === id)
        if (p) radio.tune(p.id, playable(p, now))
    }

    return (
        <main className="grid h-dvh grid-rows-[50dvh_1fr] md:grid-cols-[1fr_360px] md:grid-rows-1">
            <PianoMap
                pianos={pianos}
                now={now}
                tunedId={tuned.pianoId}
                playing={tuned.status === 'playing'}
                onTune={tune}
            />
            <Panel
                pianos={pianos}
                now={now}
                error={error}
                radio={tuned}
                onTune={tune}
                onResume={() => radio.resume()}
                onStop={() => radio.stop()}
            />
        </main>
    )
}
