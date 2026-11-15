import { Events } from '@wailsio/runtime'
import { useEffect, useState } from 'react'
import type { PianoNote } from './piano'

export type Playback = {
    piano: string
    segment: string
    position: number
    duration: number
    playing: boolean
    notes: PianoNote[]
    analysisError: string
}

// Sent at 20 Hz from the speaker's sample position, rather than running an
// unrelated browser audio player or estimating from server timestamps.
export function usePlayback() {
    const [playback, setPlayback] = useState<Playback | null>(null)
    useEffect(() => Events.On('playback', (event) => setPlayback(event.data as Playback)), [])
    return playback
}
