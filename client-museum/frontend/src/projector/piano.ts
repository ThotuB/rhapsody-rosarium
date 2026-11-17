export type PianoNote = { pitch: number; start: number; duration: number; velocity: number }
export type Transcription = { notes: PianoNote[]; duration: number; analysisSeconds: number }
export const MAX_WAV_BYTES = 32 * 1024 * 1024
export const MAX_WAV_SECONDS = 180
export const TRANSCRIBER = 'http://127.0.0.1:8765'
const names = ['C', 'C♯', 'D', 'D♯', 'E', 'F', 'F♯', 'G', 'G♯', 'A', 'A♯', 'B']
export const noteName = (pitch: number) => `${names[pitch % 12]}${Math.floor(pitch / 12) - 1}`
export const isBlack = (pitch: number) => [1, 3, 6, 8, 10].includes(pitch % 12)
let whites = 0
export const pianoKeys = Array.from({ length: 88 }, (_, i) => {
    const pitch = i + 21
    const black = isBlack(pitch)
    const left = black ? whites - 0.32 : whites++
    return { pitch, black, left, name: noteName(pitch) }
})
export const WHITE_KEYS = whites

export function readTranscription(value: unknown): Transcription {
    const data = value as Partial<Transcription> | null
    if (
        !data ||
        !Array.isArray(data.notes) ||
        !Number.isFinite(data.duration) ||
        data.duration! <= 0 ||
        data.duration! > MAX_WAV_SECONDS ||
        !Number.isFinite(data.analysisSeconds) ||
        data.analysisSeconds! < 0
    ) {
        throw new Error('The note recognition helper returned an invalid result.')
    }
    const notes = data.notes.map((note) => {
        if (
            !note ||
            !Number.isInteger(note.pitch) ||
            note.pitch < 21 ||
            note.pitch > 108 ||
            !Number.isFinite(note.start) ||
            note.start < 0 ||
            note.start >= data.duration! ||
            !Number.isFinite(note.duration) ||
            note.duration <= 0 ||
            note.start + note.duration > data.duration! + 0.01 ||
            !Number.isFinite(note.velocity) ||
            note.velocity < 0 ||
            note.velocity > 127
        ) {
            throw new Error('The note recognition helper returned invalid note timing.')
        }
        return note
    })
    return {
        notes: notes.sort((a, b) => a.start - b.start),
        duration: data.duration!,
        analysisSeconds: data.analysisSeconds!,
    }
}

export function notesAt(notes: PianoNote[], seconds: number): Map<number, number> {
    const active = new Map<number, number>()
    for (const note of notes) {
        if (note.start > seconds) break
        if (seconds < note.start + note.duration)
            active.set(note.pitch, Math.max(active.get(note.pitch) ?? 0, note.velocity))
    }
    return active
}

export function clock(seconds: number): string {
    const whole = Math.max(0, Math.floor(seconds))
    return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, '0')}`
}
