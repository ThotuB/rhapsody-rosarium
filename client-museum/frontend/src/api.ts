// The shapes the museum controller sends. Segment, PianoState and Piano are copied from
// client-web/src/api.ts (the server's /api/state, passed on by the go core); View is museum.View in go.

export type Segment = {
    url: string
    at: string
    seq: number | null // the node's window number, null if it doesn't send them
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
    listeners: number // clients tuned in to it, the museum included
    museum: boolean // the museum is one of them
}

// everything the windows show, sent by the controller as the "state" event
export type View = {
    now: string // the server's clock at the last poll
    pianos: Piano[]
    tuned: string // the tuned piano's id, '' when off
    status: 'off' | 'playing' | 'waiting'
    segment: Segment | null // the segment playing now
    buffered: number // seconds of audio lined up
    behind: number // seconds since the piano played what's playing now
    underruns: number // times playback ran dry
    serverError: string // why the last poll failed, '' if it worked
}
