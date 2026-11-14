import type { Piano, Segment } from './api'

// tuning in only reaches this far back, so listening always feels live
const LIVE_WINDOW_MS = 60_000

// How a piano is drawn (the data-state on markers and list items): the server's state,
// with silent split by how recently it was played.
export type Look = 'live' | 'recent' | 'idle' | 'offline' | 'new' | 'broken' | 'gone'

// both are server timestamps, `now` from the latest poll
export function age(at: string, now: string) {
    return Date.parse(now) - Date.parse(at)
}

// what the radio may play: audio from the last minute, and only while the piano's node is connected
export function playable(piano: Piano, now: string): Segment[] {
    if (piano.state !== 'playing' && piano.state !== 'silent') return []
    return piano.segments.filter((s) => age(s.at, now) < LIVE_WINDOW_MS)
}

export function look(piano: Piano, now: string): Look {
    switch (piano.state) {
        case 'playing':
            return 'live'
        case 'silent':
            return piano.lastPlayed && age(piano.lastPlayed, now) < 15 * 60_000 ? 'recent' : 'idle'
        default:
            return piano.state
    }
}

// a few words on what the piano is doing, for the station list and the marker's label
export function describe(piano: Piano, now: string) {
    switch (piano.state) {
        case 'playing':
            return 'playing now'
        case 'silent':
            return piano.lastPlayed ? `played ${formatAgo(piano.lastPlayed, now)}` : 'nobody played yet'
        case 'offline':
            return 'offline'
        case 'new':
            return 'not connected yet'
        case 'broken':
            return 'broken'
        case 'gone':
            return 'gone'
    }
}

export function formatAgo(at: string, now: string) {
    const s = Math.max(0, age(at, now) / 1000)
    if (s < 45) return 'just now'
    const m = Math.round(s / 60)
    if (m < 60) return `${m} min ago`
    const h = Math.round(m / 60)
    if (h < 24) return `${h} h ago`
    return `${Math.round(h / 24)} d ago`
}
