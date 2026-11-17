import { noteName, notesAt, pianoKeys, WHITE_KEYS, type PianoNote } from './piano'
import './piano-keyboard.css'

export function PianoKeyboard({
    notes,
    position,
    playing,
}: {
    notes: PianoNote[]
    position: number
    playing: boolean
}) {
    const active = playing && position >= 0 ? notesAt(notes, position) : new Map<number, number>()
    const pitches = [...active.keys()].sort((a, b) => a - b)
    return (
        <div
            className="piano-keyboard"
            role="img"
            aria-label={`88-key piano. ${pitches.length ? `Active notes: ${pitches.map(noteName).join(', ')}` : 'No active notes.'}`}
        >
            {pianoKeys.map((key) => (
                <div
                    key={key.pitch}
                    className={`piano-key ${key.black ? 'black-key' : 'white-key'} ${active.has(key.pitch) ? 'is-active' : ''}`}
                    style={{
                        left: `${(key.left / WHITE_KEYS) * 100}%`,
                        width: `${((key.black ? 0.64 : 1) / WHITE_KEYS) * 100}%`,
                    }}
                >
                    {!key.black && key.pitch % 12 === 0 && <span>{key.name}</span>}
                </div>
            ))}
        </div>
    )
}
