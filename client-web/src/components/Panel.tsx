import type { Piano, PianoState } from '../api'
import type { RadioSnapshot } from '../radio'
import { describe, formatAgo, look } from '../time'
import { Visualizer } from './Visualizer'

// shown while tuned in with nothing to play
const WAITING: Partial<Record<PianoState, string>> = {
    offline: 'This piano is offline right now.',
    new: "This piano hasn't connected yet.",
    broken: 'This piano is broken.',
    gone: 'This piano is gone.',
}

type Props = {
    pianos: Piano[]
    now: string
    error: string | null
    radio: RadioSnapshot
    onTune: (id: string) => void
    onResume: () => void
    onStop: () => void
}

export function Panel({ pianos, now, error, radio, onTune, onResume, onStop }: Props) {
    const tuned = pianos.find((p) => p.id === radio.pianoId)
    const pill = 'mt-1.5 cursor-pointer rounded-full border border-line px-4.5 py-1.5'

    return (
        <aside className="flex flex-col gap-4 overflow-y-auto border-line bg-surface p-4 max-md:border-t md:border-l md:p-5">
            <header>
                <h1 className="text-[22px] font-bold tracking-[-0.3px]">Rhapsody Rosarium</h1>
                <p className="text-muted">Tune in to the city.</p>
            </header>

            {error && <p className="rounded-lg bg-rose/15 px-3 py-2">Can't reach the server ({error}).</p>}

            <section className="flex flex-col items-center gap-2 text-center" aria-live="polite">
                <Visualizer />
                {tuned ? (
                    <div className="flex flex-col items-center gap-1">
                        <h2 className="text-lg font-bold">{tuned.name}</h2>
                        <p className="text-muted">
                            {radio.status === 'waiting'
                                ? (WAITING[tuned.state] ?? 'Waiting for someone to play…')
                                : radio.segment && `Played ${formatAgo(radio.segment.at, now)}`}
                        </p>
                        {tuned.note && <p className="text-sm text-muted italic">{tuned.note}</p>}
                        {radio.status === 'blocked' ? (
                            <button type="button" className={pill} onClick={onResume}>
                                Tap to listen
                            </button>
                        ) : (
                            <button type="button" className={pill} onClick={onStop}>
                                Stop
                            </button>
                        )}
                    </div>
                ) : (
                    <p className="text-muted">Pick a piano on the map to tune in.</p>
                )}
            </section>

            <ul className="flex flex-col gap-1">
                {pianos.map((p) => (
                    <li key={p.id}>
                        <button
                            type="button"
                            className={`flex w-full cursor-pointer items-center gap-2.5 rounded-[10px] px-3 py-2.5 text-left hover:bg-rose/10 ${p.id === radio.pianoId ? 'bg-rose/10' : ''}`}
                            data-state={look(p, now)}
                            onClick={() => onTune(p.id)}
                        >
                            <span className="dot" />
                            <span className="flex-1">
                                {p.name}
                                {p.note && <span className="block text-xs text-muted">{p.note}</span>}
                            </span>
                            <span className="text-sm text-muted">{describe(p, now)}</span>
                        </button>
                    </li>
                ))}
            </ul>
        </aside>
    )
}
