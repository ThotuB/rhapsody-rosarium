import { useSyncExternalStore } from 'react'
import { continues, type Segment } from './api'

// The audio engine lives outside React, with one AudioContext for the whole page.
// Segments are fetched and decoded ahead of time, then scheduled back to back on the audio clock,
// so one segment starts on the exact sample the previous one ends. An <audio> element would
// stop between files to load the next one, which is the hiccup this avoids.

export type RadioStatus = 'off' | 'playing' | 'waiting' | 'blocked' // blocked: the browser wants a click first

export type RadioSnapshot = {
    pianoId: string | null
    status: RadioStatus
    segment: Segment | null // the one playing now
}

const STATIC_SECONDS = 0.9
// how far behind the piano listeners are: about START_BACK + 1 segments (2 s each from node-sim),
// the newest is still being recorded. fewer is closer to live but cuts out when one arrives late
const START_BACK = 2 // tuning in starts this many segments before the newest
const AHEAD = 2 // segments decoded and scheduled after the one playing
const LEAD = 0.1 // seconds of head start when playback starts on tuning in
const CUSHION = 2 // seconds of head start when it starts again after running dry, so the next segment can arrive
const FADE = 0.03 // seconds to fade out and in around a gap
const PAUSE = 0.25 // seconds of silence where a window is missing

const time = (s: Segment) => Date.parse(s.at)
const byTime = (segments: Segment[]) => [...segments].sort((a, b) => time(a) - time(b))

type Scheduled = { seg: Segment; src: AudioBufferSourceNode; gain: GainNode; end: number }

class Radio {
    private ctx: AudioContext | null = null
    private music: GainNode | null = null
    private out: AnalyserNode | null = null
    private noise: AudioBuffer | null = null

    private queue: Segment[] = [] // to play, not scheduled yet, oldest first
    private scheduled: Scheduled[] = [] // playing or lined up on the audio clock, in order
    private end = 0 // audio clock time when the last scheduled segment ends
    private last: Segment | null = null // the last segment scheduled, to tell whether the next one joins it
    private heard = -Infinity // time of the newest segment queued since tuning in, so nothing plays twice
    private loading = false
    private gen = 0 // bumped on every tune and stop, so async work for an old station gives up
    private snapshot: RadioSnapshot = { pianoId: null, status: 'off', segment: null }
    private listeners = new Set<() => void>()

    subscribe = (listener: () => void) => {
        this.listeners.add(listener)
        return () => {
            this.listeners.delete(listener)
        }
    }

    getSnapshot = () => this.snapshot

    // Everything after the music, static included, so visuals can react to it.
    get analyser() {
        return this.out
    }

    // Must be called from a click handler: browsers only allow audio to start from a user gesture.
    // segments are what may be played right now (see playable() in time.ts), possibly none.
    tune(pianoId: string, segments: Segment[]) {
        this.unlock()
        if (pianoId === this.snapshot.pianoId) return
        this.reset()
        this.set({ pianoId, status: 'waiting', segment: null })
        this.playStatic()
        this.fadeIn(STATIC_SECONDS * 0.6)
        this.enqueue(fromNewest(segments))
    }

    // New poll results for the tuned piano.
    update(pianoId: string, segments: Segment[]) {
        if (pianoId !== this.snapshot.pianoId) return
        // nothing heard since tuning in: start a few back from the newest, like tune(); otherwise only what's new
        const fresh =
            this.heard === -Infinity ? fromNewest(segments) : byTime(segments).filter((s) => time(s) > this.heard)
        if (!fresh.length) return
        const resuming = this.snapshot.status === 'waiting' && !this.scheduled.length && !this.loading
        if (resuming) this.fadeIn(0)
        this.enqueue(fresh)
    }

    // After 'blocked', from a click handler.
    resume() {
        this.unlock()
        if (this.scheduled.length) this.set({ status: 'playing' })
    }

    stop() {
        this.reset()
        this.set({ pianoId: null, status: 'off', segment: null })
    }

    private unlock() {
        if (!this.ctx) {
            // the rate the server stores segments at: decoding then doesn't resample each one on its own,
            // which would leave a small glitch at every join
            const ctx = new AudioContext({ sampleRate: 48000 })
            this.music = ctx.createGain()
            this.out = ctx.createAnalyser()
            this.out.fftSize = 2048
            this.out.smoothingTimeConstant = 0.8
            this.music.connect(this.out).connect(ctx.destination)
            this.noise = noiseBuffer(ctx)
            this.ctx = ctx
        }
        void this.ctx.resume()
    }

    private reset() {
        this.gen++
        for (const s of this.scheduled) s.src.stop()
        this.scheduled = []
        this.queue = []
        this.end = 0
        this.last = null
        this.heard = -Infinity
        this.loading = false
    }

    private enqueue(segments: Segment[]) {
        for (const s of segments) {
            if (time(s) <= this.heard) continue
            this.queue.push(s)
            this.heard = time(s)
        }
        void this.pump()
    }

    // Fetch, decode and schedule queued segments until AHEAD of them are lined up behind the one playing.
    // Runs whenever segments arrive and whenever one finishes, so the next is always ready before it's needed.
    private async pump() {
        if (this.loading || !this.ctx) return
        this.loading = true
        const gen = this.gen
        while (this.queue.length && this.scheduled.length <= AHEAD) {
            const seg = this.queue[0]
            const buf = await this.load(seg)
            if (gen !== this.gen) return
            this.queue.shift()
            if (buf) this.schedule(seg, buf) // a segment that won't load, e.g. already pruned, is skipped
        }
        this.loading = false
        if (!this.scheduled.length) this.wait()
    }

    private async load(seg: Segment): Promise<AudioBuffer | null> {
        try {
            const res = await fetch(seg.url)
            if (!res.ok) return null
            return await this.ctx!.decodeAudioData(await res.arrayBuffer())
        } catch {
            return null
        }
    }

    // Line the segment up right where the previous one ends, or a little after now if playback had stopped.
    // If a window is missing between the two, fade out, pause briefly and fade back in, rather than
    // gluing two unrelated moments together.
    private schedule(seg: Segment, buf: AudioBuffer) {
        const ctx = this.ctx!
        const gen = this.gen
        if (ctx.state === 'suspended') this.set({ status: 'blocked' })
        const now = ctx.currentTime
        // tuning in already has START_BACK segments lined up; after running dry there's only this one
        let start = Math.max(this.end, now + (this.last === null ? LEAD : CUSHION))
        const gap = this.last !== null && !continues(this.last, seg)
        const prev = this.scheduled.at(-1)
        if (gap && prev && prev.end > now) {
            prev.gain.gain.setValueAtTime(1, Math.max(now, prev.end - FADE))
            prev.gain.gain.linearRampToValueAtTime(0, prev.end)
            start += PAUSE
        }

        const gain = ctx.createGain() // per segment, so a fade can be added once the next one is known
        if (gap) {
            gain.gain.setValueAtTime(0, start)
            gain.gain.linearRampToValueAtTime(1, start + FADE)
        }
        const src = ctx.createBufferSource()
        src.buffer = buf
        src.connect(gain).connect(this.music!)
        src.start(start)
        const item: Scheduled = { seg, src, gain, end: start + buf.duration }
        this.end = item.end
        this.last = seg
        this.scheduled.push(item)

        // shown as the current segment once it actually starts
        window.setTimeout(
            () => {
                if (gen === this.gen && this.scheduled.includes(item) && this.snapshot.status !== 'blocked')
                    this.set({ status: 'playing', segment: seg })
            },
            (start - ctx.currentTime) * 1000,
        )
        src.onended = () => {
            if (gen !== this.gen) return // stopped by a station switch
            this.scheduled = this.scheduled.filter((s) => s !== item)
            if (this.scheduled.length || this.queue.length || this.loading) void this.pump()
            else this.wait()
        }
    }

    private wait() {
        this.set({ status: 'waiting', segment: null })
    }

    private fadeIn(delay: number) {
        if (!this.ctx || !this.music) return
        const gain = this.music.gain
        const t = this.ctx.currentTime
        gain.cancelScheduledValues(t)
        gain.setValueAtTime(0, t)
        gain.setValueAtTime(0, t + delay)
        gain.linearRampToValueAtTime(1, t + delay + 0.4)
    }

    // A burst of band-passed noise, like turning the dial between stations.
    private playStatic() {
        if (!this.ctx || !this.noise || !this.out) return
        const ctx = this.ctx
        const src = ctx.createBufferSource()
        const filter = ctx.createBiquadFilter()
        const gain = ctx.createGain()
        src.buffer = this.noise
        filter.type = 'bandpass'
        filter.frequency.value = 1800
        filter.Q.value = 0.6
        const t = ctx.currentTime
        gain.gain.setValueAtTime(0, t)
        gain.gain.linearRampToValueAtTime(0.25, t + 0.05)
        gain.gain.linearRampToValueAtTime(0.12, t + STATIC_SECONDS * 0.6)
        gain.gain.linearRampToValueAtTime(0, t + STATIC_SECONDS)
        src.connect(filter).connect(gain).connect(this.out)
        src.start(t, Math.random() * 0.5)
        src.stop(t + STATIC_SECONDS)
    }

    private set(patch: Partial<RadioSnapshot>) {
        this.snapshot = { ...this.snapshot, ...patch }
        this.listeners.forEach((l) => l())
    }
}

// the last few segments, oldest first: where listening starts when tuning in
function fromNewest(segments: Segment[]) {
    const sorted = byTime(segments)
    return sorted.slice(Math.max(0, sorted.length - START_BACK))
}

function noiseBuffer(ctx: AudioContext) {
    const buf = ctx.createBuffer(1, ctx.sampleRate * 1.5, ctx.sampleRate)
    const data = buf.getChannelData(0)
    for (let i = 0; i < data.length; i++) {
        const crackle = Math.random() < 0.001 ? 4 : 1
        data[i] = (Math.random() * 2 - 1) * crackle
    }
    return buf
}

export const radio = new Radio()

export function useRadio() {
    return useSyncExternalStore(radio.subscribe, radio.getSnapshot)
}
