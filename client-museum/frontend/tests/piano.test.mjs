// Node 26 can import this module's erasable TypeScript directly.
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { notesAt, pianoKeys, readTranscription, WHITE_KEYS } from '../src/piano.ts'

test('keyboard spans A0 to C8 with 52 white keys and 36 black keys', () => {
    assert.equal(pianoKeys.length, 88)
    assert.equal(WHITE_KEYS, 52)
    assert.equal(pianoKeys.filter((key) => key.black).length, 36)
    assert.equal(pianoKeys[0].name, 'A0')
    assert.equal(pianoKeys.at(-1).name, 'C8')
})

test('simultaneous notes, release boundaries, silence and seeking use audio time', () => {
    const { notes } = readTranscription({ duration: 3, analysisSeconds: 1, notes: [
        { pitch: 64, start: 1, duration: 0.5, velocity: 90 },
        { pitch: 60, start: 0.5, duration: 1, velocity: 70 },
        { pitch: 60, start: 1, duration: 1, velocity: 100 },
    ] })
    assert.deepEqual([...notesAt(notes, 0)], [])
    assert.deepEqual([...notesAt(notes, 1)], [[60, 100], [64, 90]])
    assert.deepEqual([...notesAt(notes, 1.5)], [[60, 100]])
    assert.deepEqual([...notesAt(notes, 2)], [])
    assert.deepEqual([...notesAt(notes, 0.5)], [[60, 70]])
})

test('silence is a valid result; malformed notes are rejected', () => {
    assert.deepEqual(readTranscription({ duration: 1, analysisSeconds: 0, notes: [] }).notes, [])
    for (const patch of [{ pitch: 200 }, { start: -1 }, { duration: 2 }, { velocity: NaN }]) {
        assert.throws(() => readTranscription({ duration: 1, analysisSeconds: 0, notes: [
            { pitch: 60, start: 0, duration: 0.5, velocity: 80, ...patch },
        ] }))
    }
})
