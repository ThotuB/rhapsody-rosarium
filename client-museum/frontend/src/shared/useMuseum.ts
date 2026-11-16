import { Events } from '@wailsio/runtime'
import { useEffect, useState } from 'react'
import { IntroAction, MuseumService, type IntroState } from '../../bindings/rhapsody-rosarium/museum'
import type { View } from './api'

// The controller's view: fetched once when the window opens, then pushed as "state" events.
export function useMuseum(): View | null {
    const [view, setView] = useState<View | null>(null)
    useEffect(() => {
        MuseumService.State().then((v) => setView(v as unknown as View))
        return Events.On('state', (event) => setView(event.data as unknown as View))
    }, [])
    return view
}

// Loudness of what's playing, 0 to 1, pushed about 20 times a second.
export function useLevel(): number {
    const [level, setLevel] = useState(0)
    useEffect(() => Events.On('level', (event) => setLevel(event.data as number)), [])
    return level
}

export const tune = (id: string) => MuseumService.Tune(id)
export const stop = () => MuseumService.Stop()
// the colour of the light at a piano, "#rrggbb"; its node shows it within a heartbeat (~2 s)
export const setLED = (id: string, colour: string) => MuseumService.SetLED(id, colour)
// where the projector's opening video is read from (-video), and whether a file is there
export const introVideo = () => MuseumService.IntroVideo()

// The projector's opening video is controlled from the museum window: the actions go to the projector
// as "intro-action" events, and it reports what it does back as "intro" events.
export type { IntroState }
export { IntroAction }
export const controlIntro = (action: IntroAction) => MuseumService.ControlIntro(action)
export const reportIntro = (state: IntroState) => MuseumService.ReportIntro(state)

// What the projector's opening video does, for the museum window's controls.
export function useIntro(): IntroState | null {
    const [intro, setIntro] = useState<IntroState | null>(null)
    useEffect(() => {
        MuseumService.Intro().then(setIntro)
        return Events.On('intro', (event) => setIntro(event.data as IntroState))
    }, [])
    return intro
}
