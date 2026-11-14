import { useEffect } from 'react'
import { beaconListening, postListening } from './api'

// an id for this tab, made up here: the server only needs to tell listeners apart.
// not crypto.randomUUID(), browsers only have it on https and the venue's server is plain http
const CLIENT = Math.random().toString(36).slice(2, 12)
const EVERY = 5000 // ms; the server forgets a listener after 15 s without hearing from it

// Tells the server which piano this tab listens to, right away on every change and again every few
// seconds while tuned in. Muted, paused or hidden doesn't matter, tuned in counts as listening.
export function useListening(pianoId: string | null) {
    useEffect(() => {
        // a single post per change, so a station switch can't arrive as "stopped" after the new piano
        postListening(CLIENT, pianoId ?? '')
        if (!pianoId) return
        const timer = window.setInterval(() => postListening(CLIENT, pianoId), EVERY)
        const leave = () => beaconListening(CLIENT, '')
        window.addEventListener('pagehide', leave)
        return () => {
            window.clearInterval(timer)
            window.removeEventListener('pagehide', leave)
        }
    }, [pianoId])
}
