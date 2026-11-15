import { useEffect, useRef, useState } from 'react'
import { noteName, notesAt } from '../piano'
import { introVideo, tune, useMuseum } from '../useMuseum'
import { usePlayback } from '../usePlayback'
import { PianoKeyboard } from './PianoKeyboard'
import { PianoMap } from './PianoMap'
import './stream-projector.css'

export function StreamProjector() {
    const view = useMuseum()
    const playback = usePlayback()
    const video = useRef<HTMLVideoElement>(null)
    const [phase, setPhase] = useState<'intro' | 'map'>('intro')
    const [videoError, setVideoError] = useState('')
    // starts with sound: the museum's web view lets videos autoplay unmuted (if it refuses, "Play film" shows)
    const [muted, setMuted] = useState(false)
    const [blocked, setBlocked] = useState(false)
    // where the video is loaded from: the app's own path, or on Linux a local server (see video.go)
    const [videoSrc, setVideoSrc] = useState<string>()
    useEffect(() => {
        introVideo().then((info) => setVideoSrc(info.url)).catch(() => setVideoSrc('/projector-intro.mp4'))
    }, [])
    const piano = view?.pianos.find((p) => p.id === view.tuned)
    // Drop old-piano events immediately when the operator selects another station.
    const current = playback?.piano === view?.tuned ? playback : null
    const active = current?.playing ? [...notesAt(current.notes, current.position).keys()].sort((a, b) => a - b) : []

    function showMap() {
        video.current?.pause()
        setPhase('map')
    }

    // Says where the video was looked for: either no file is there, or one is and the web view can't play it.
    async function explainVideoError() {
        const playback = video.current?.error?.message || `media error ${video.current?.error?.code ?? '?'}`
        const info = await introVideo().catch(() => null)
        if (!info) setVideoError(`The opening video could not be loaded (${playback}). Check the -video path, or continue to the map.`)
        else if (!info.found) setVideoError(`No opening video at ${info.path}. Put it there or start with -video <path>, or continue to the map.`)
        else setVideoError(`The opening video at ${info.path} (${Math.round(info.size / 2 ** 20)} MB) was found but can't be played (${playback}). On Linux this is usually a missing GStreamer H.264 decoder. Or continue to the map.`)
    }

    async function playIntro(restart: boolean) {
        const element = video.current
        if (!element) return
        if (restart) element.currentTime = 0
        setPhase('intro')
        setBlocked(false)
        try { await element.play() } catch { setBlocked(true) }
    }

    return <main className="stream-projector" data-phase={phase}>
        <section className="projector-map-area" aria-label="Map of the pianos" inert={phase === 'intro'}>
            <PianoMap projection pianos={view?.pianos ?? []} now={view?.now ?? new Date().toISOString()}
                tunedId={view?.tuned || null} playing={view?.status === 'playing'} onTune={tune} />
            <div className="projector-title"><span>RHAPSODY ROSARIUM</span><h1>Listen to the city.</h1></div>
            {view?.serverError && <p className="projector-connection" role="status">Waiting for the piano server…</p>}
        </section>

        <section className="projector-piano-area" aria-label="Piano following the museum audio" inert={phase === 'intro'}>
            <header className="projector-piano-heading">
                <div><span className="projector-live-dot" data-playing={current?.playing || undefined} />
                    <strong>{piano?.name ?? 'Choose a piano in the museum window'}</strong>
                    <span className="projector-stream-status">{!piano ? 'Ready to listen' : current?.analysisError
                        ? current.analysisError : current?.playing
                            ? current.notes.length ? 'Following the sound' : 'No notes detected in this segment'
                            : 'Waiting for audio / recognizing notes…'}</span></div>
                <span className="projector-note-names">{active.map(noteName).join(' · ') || '—'}</span>
            </header>
            <PianoKeyboard notes={current?.notes ?? []} position={current?.position ?? 0} playing={current?.playing ?? false} />
            <div className="projector-piano-foot"><span>ROSARIUM</span><span>88 KEYS · LIVE AUDIO</span></div>
        </section>

        <section className="projector-video-layer" aria-label="Opening film" aria-hidden={phase !== 'intro'} inert={phase !== 'intro'}>
            <video ref={video} src={videoSrc} autoPlay muted={muted} playsInline preload="auto"
                onEnded={showMap} onPlay={() => setBlocked(false)}
                onError={() => void explainVideoError()}
                onCanPlay={() => { if (phase === 'intro' && video.current?.paused) void playIntro(false) }} />
            <div className="projector-video-shade" />
            <div className="projector-film-caption"><span>RHAPSODY ROSARIUM</span><p>A city. A piano. A moment.</p></div>
            {(videoError || blocked) && <div className="projector-video-message" role="status">
                <p>{videoError || 'Press Play to start the opening film.'}</p>
                {!videoError && <button onClick={() => playIntro(false)}>Play film</button>}
                <button onClick={showMap}>Continue to map</button>
            </div>}
        </section>
        <nav className="projector-tools" aria-label="Projector controls">
            {phase === 'intro' ? <>
                <button onClick={() => setMuted(!muted)}>{muted ? 'Enable video sound' : 'Mute video'}</button>
                <button onClick={showMap}>Skip intro →</button>
            </> : <button onClick={() => playIntro(true)}>Replay intro</button>}
        </nav>
    </main>
}
