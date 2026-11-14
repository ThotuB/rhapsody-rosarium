// mirrors the go server's GET /api/state

export type Segment = {
    url: string
    at: string
    seq: number | null // the node's window number, null if it doesn't send them
}

// Whether `next` is the window straight after `prev`, so the two can play back to back.
// A skipped number (a window that never arrived) or a reset (the node restarted) means a gap.
// Without numbers there's no way to tell, so they're assumed to continue.
export function continues(prev: Segment, next: Segment) {
    if (prev.seq === null || next.seq === null) return true
    return next.seq === prev.seq + 1
}

// playing, silent, offline and new are worked out by the server, broken and gone are set by hand in pianos.json
export type PianoState = 'playing' | 'silent' | 'offline' | 'new' | 'broken' | 'gone'

export type Piano = {
    id: string
    name: string
    lat: number
    lng: number
    state: PianoState
    note: string | null // set by hand in pianos.json
    lastSeen: string | null // last heartbeat or upload from its node
    lastPlayed: string | null
    segments: Segment[] // oldest first
}

export type State = {
    now: string
    pianos: Piano[]
}

export async function fetchState(signal?: AbortSignal): Promise<State> {
    const res = await fetch('/api/state', { signal })
    if (!res.ok) throw new Error(`GET /api/state: ${res.status}`)
    return res.json()
}
