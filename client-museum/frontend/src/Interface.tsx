import type { PianoState } from './api'
import { PianoMap } from './components/PianoMap'
import { LedPicker } from './LedPicker'
import { describe, formatAgo, look } from './time'
import { stop, tune, useMuseum } from './useMuseum'

// shown while tuned in with nothing to play
const WAITING: Partial<Record<PianoState, string>> = {
    offline: 'This piano is offline right now.',
    new: "This piano hasn't connected yet.",
    broken: 'This piano is broken.',
    gone: 'This piano is gone.',
}

// The window on the installation's screen: the map, what plays on the speakers, and the stations.
export function Interface() {
    const view = useMuseum()
    if (!view) return <p className="p-6 text-muted">Starting…</p>

    const { pianos, now } = view
    const tuned = pianos.find((p) => p.id === view.tuned)
    const pill = 'mt-1.5 cursor-pointer rounded-full border border-line px-4.5 py-1.5'

    return (
        <main className="grid h-dvh grid-cols-[1fr_380px]">
            <PianoMap
                pianos={pianos}
                now={now}
                tunedId={view.tuned || null}
                playing={view.status === 'playing'}
                onTune={tune}
            />
            <aside className="flex flex-col gap-4 overflow-y-auto border-l border-line bg-surface p-5">
                <header>
                    <h1 className="flex items-center gap-2 text-[22px] font-bold tracking-[-0.3px]">
                        Rhapsody Rosarium
                        <span className="rounded-full bg-rose/15 px-2 py-0.5 text-xs font-medium text-rose">
                            museum
                        </span>
                    </h1>
                    <p className="text-muted">Tune in to the city.</p>
                </header>

                {view.serverError && (
                    <p className="rounded-lg bg-rose/15 px-3 py-2">Can't reach the server ({view.serverError}).</p>
                )}

                <section className="flex flex-col items-center gap-1 py-4 text-center" aria-live="polite">
                    {tuned ? (
                        <>
                            <h2 className="text-lg font-bold">{tuned.name}</h2>
                            <p className="text-muted">
                                {view.status === 'waiting'
                                    ? (WAITING[tuned.state] ?? 'Waiting for someone to play…')
                                    : view.segment &&
                                      `Playing on the speakers, played ${formatAgo(view.segment.at, now)}`}
                            </p>
                            {tuned.note && <p className="text-sm text-muted italic">{tuned.note}</p>}
                            <p className="text-xs text-muted">
                                {view.behind.toFixed(1)} s behind the piano · {view.buffered.toFixed(1)} s lined up ·{' '}
                                {view.underruns} underruns
                            </p>
                            <button type="button" className={pill} onClick={stop}>
                                Stop
                            </button>
                            <LedPicker piano={tuned} />
                        </>
                    ) : (
                        <p className="text-muted">Pick a piano to play it on the speakers.</p>
                    )}
                </section>

                <ul className="flex flex-col gap-1">
                    {pianos.map((p) => (
                        <li key={p.id}>
                            <button
                                type="button"
                                className={`flex w-full cursor-pointer items-center gap-2.5 rounded-[10px] px-3 py-2.5 text-left hover:bg-rose/10 ${p.id === view.tuned ? 'bg-rose/10' : ''}`}
                                data-state={look(p, now)}
                                onClick={() => tune(p.id)}
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
        </main>
    )
}
