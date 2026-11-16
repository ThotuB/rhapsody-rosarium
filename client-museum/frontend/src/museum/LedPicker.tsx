import { useEffect, useRef, useState } from 'react'
import type { Piano } from '../shared/api'
import { setLED } from '../shared/useMuseum'

const PRESETS = ['#ff00ff', '#ff0000', '#ff6000', '#ffc000', '#00ff00', '#00ffff', '#0000ff', '#ffffff']

// Sets the colour of the light at the piano, on its node. The node picks it up with its next
// heartbeat, so it changes there within a couple of seconds.
export function LedPicker({ piano }: { piano: Piano }) {
    const [error, setError] = useState<string | null>(null)
    const custom = useRef<HTMLInputElement>(null)

    const choose = (colour: string) => {
        setError(null)
        setLED(piano.id, colour).catch((e: unknown) => setError(String(e)))
    }

    // the custom colour is sent once it's picked, not on every step of dragging through the picker
    // (react's onChange fires for each one)
    useEffect(() => {
        const input = custom.current
        if (!input) return
        const onPicked = () => choose(input.value)
        input.addEventListener('change', onPicked)
        return () => input.removeEventListener('change', onPicked)
    })

    const swatch = 'size-7 cursor-pointer rounded-full border border-line'
    const chosen = 'outline-2 outline-offset-2 outline-ink'
    return (
        <div className="mt-2 flex flex-col items-center gap-1.5">
            <p className="text-xs text-muted">Light at the piano</p>
            <div className="flex flex-wrap justify-center gap-2">
                {PRESETS.map((colour) => (
                    <button
                        key={colour}
                        type="button"
                        aria-label={`Light colour ${colour}`}
                        className={`${swatch} ${piano.led === colour ? chosen : ''}`}
                        style={{ background: colour }}
                        onClick={() => choose(colour)}
                    />
                ))}
                <input
                    ref={custom}
                    type="color"
                    aria-label="Another light colour"
                    className={`${swatch} ${PRESETS.includes(piano.led) ? '' : chosen}`}
                    defaultValue={piano.led}
                    key={piano.id}
                />
            </div>
            {error && <p className="text-xs text-rose">Couldn't set the colour ({error})</p>}
        </div>
    )
}
