"""simulate several piano-nodes uploading segments to the server.

each playing node plays nonstop, uploading one segment after another of music
it generates itself (see music.py); muted nodes stay online without playing.

every node also sends a heartbeat every few seconds, so the server knows it is
online even while nobody plays. stop the simulator and its pianos go offline.
the simulator exits by itself once every node has stopped.

what exactly is sent to the server is in api.py.

    uv run simulate.py --nodes 10
    uv run simulate.py --nodes 6 --fast
    uv run simulate.py --mute piano-3    # piano-3 stays online but never plays
    uv run simulate.py --nodes 10 --wav "recording.wav" --wav-piano piano-2

one process per piano, id set by arg (run in several terminals):
    uv run simulate.py --id piano-1
    uv run simulate.py --id piano-2

a scenario from a json file (see configs/), flags given on top override its settings:
    uv run simulate.py --config configs/mixed.json
    uv run simulate.py --config configs/mixed.json --fast

    {
      "server": "http://localhost:8080/api",    any of: server, token, segment_seconds, fast
      "segment_seconds": 2,
      "pianos": [
        { "id": "piano-2" },                     plays
        { "id": "piano-5", "mute": true },       online, never plays
        { "id": "piano-7", "stop_after": 30 }    stops after 30 s, so it goes offline
      ]
    }
"""

import itertools
import math
import queue
import random
import threading
import time
from collections import deque
from dataclasses import dataclass
from typing import Self

import requests

import api
import config
from config import Piano, Settings
from music import Voice
from wav_source import WavVoice

MAX_QUEUED = 20  # segments kept for retrying while the server is unreachable
HEARTBEAT_SECONDS = 2  # also how soon the node hears that someone tuned in
RETRY_SECONDS = 2  # how often failed uploads are retried when nothing new arrives


@dataclass
class Window:
    """one segment of a node's audio, waiting to be uploaded"""

    seq: int  # counts up from 0 since the node started, so listeners can spot a missing one
    name: str
    audio: bytes


@dataclass
class Node:
    """a fake piano node, run like a real one would be: its own thread generates a
    window of music every segment (a real node would record it), an upload thread
    sends the windows, and a heartbeat thread tells the server the node is online.
    a slow upload never delays the next window."""

    id: str
    token: str
    server: str
    mute: bool = False  # heartbeats only, never plays
    stop_at: float | None = None  # time.monotonic() when it stops, None to run forever
    wav_voice: WavVoice | None = None

    @classmethod
    def from_piano(cls, piano: Piano, server: str, default_token: str) -> Self:
        """the node for a piano from the scenario, with its token resolved and its stop time set"""
        return cls(
            id=piano.id,
            token=(piano.token or default_token).format(id=piano.id),
            server=server,
            mute=piano.mute,
            stop_at=(
                None
                if piano.stop_after is None
                else time.monotonic() + piano.stop_after
            ),
        )

    def alive(self) -> bool:
        return self.stop_at is None or time.monotonic() < self.stop_at

    def start(self, settings: Settings) -> threading.Thread:
        """launch the node in its own thread and return right away; the thread ends when the node stops"""
        what = "muted" if self.mute else "playing"
        if self.wav_voice is not None:
            what = "playing WAV recording on repeat"
        if self.stop_at is not None:
            what += f", stops after {self.stop_at - time.monotonic():.0f}s"
        log(self.id, what)

        thread = threading.Thread(
            target=self.run, args=(settings,), name=self.id, daemon=True
        )
        thread.start()
        return thread

    def run(self, settings: Settings):
        """the node's body: blocks until the node stops"""
        windows: queue.Queue[Window] = queue.Queue()
        workers = [threading.Thread(target=self.heartbeat, daemon=True)]
        if not self.mute:
            workers.append(
                threading.Thread(target=self.upload, args=(windows,), daemon=True)
            )
        for worker in workers:
            worker.start()

        if self.mute:
            nap(math.inf, self.alive)
        else:
            self.play(settings, windows)

        for worker in workers:
            worker.join()
        log(self.id, "stopped, the server will show it offline soon")

    def play(self, settings: Settings, windows: queue.Queue[Window]):
        """generate a window of music every segment_seconds and hand it to the uploader"""
        voice = self.wav_voice or Voice.for_node(self.id)

        # pause returns false once the node is stopped, so a half-recorded window is never sent
        def pause(seconds):
            nap(0.2 if settings.fast else seconds, self.alive)
            return self.alive()

        # a random start, so the nodes don't all upload at the same moment
        start = random.Random(f"{self.id}/start").uniform(0, settings.segment_seconds)
        if not pause(start):
            return

        # a window is due every segment_seconds on a fixed clock, so generating it comes out
        # of the wait instead of adding to it, and the node keeps up with real time
        due = time.monotonic()
        for seq in itertools.count():
            name = f"{self.id}-{int(time.time() * 1000)}.wav"
            audio = voice.segment(settings.segment_seconds)

            # "recording" the window takes as long as the window itself
            due += settings.segment_seconds
            if not pause(due - time.monotonic()):
                return
            windows.put(Window(seq, name, audio))

    def upload(self, windows: queue.Queue[Window]):
        """send windows in order as they come, retrying the ones the server didn't take"""
        session = requests.Session()
        # when this overflows, the oldest windows are dropped; their numbers leave a gap
        pending: deque[Window] = deque(maxlen=MAX_QUEUED)
        while self.alive():
            try:
                pending.append(windows.get(timeout=RETRY_SECONDS))
            except queue.Empty:
                pass
            self.flush(session, pending)

    def flush(self, session: requests.Session, pending: deque[Window]):
        """upload pending windows oldest first, stopping at the first one to retry later"""
        while pending:
            window = pending[0]
            name, data = window.name, window.audio
            try:
                r = api.post_segment(
                    session, self.server, self.id, self.token, name, data, window.seq
                )
            except requests.RequestException as e:
                log(
                    self.id,
                    f"upload failed ({type(e).__name__}), {len(pending)} queued",
                )
                return

            if r.ok:
                log(self.id, f"sent {name} ({len(data) // 1024} KB) -> {r.status_code}")
            elif r.status_code < 500:
                log(
                    self.id,
                    f"rejected {name} -> {r.status_code} {r.text[:80]!r}, dropped",
                )
            else:
                log(self.id, f"server error {r.status_code}, {len(pending)} queued")
                return

            pending.popleft()

    def heartbeat(self):
        """tell the server the node is online, every HEARTBEAT_SECONDS until it stops,
        and log who is listening whenever that changes"""
        session = requests.Session()
        failing = False
        heard = None  # the audience from the last heartbeat
        while self.alive():
            try:
                r = api.post_heartbeat(session, self.server, self.id, self.token)
                ok, why = r.ok, str(r.status_code)
                audience = r.json() if ok else None
            except requests.RequestException as e:  # json errors too
                ok, why, audience = False, type(e).__name__, None

            if not ok and not failing:
                log(self.id, f"heartbeat failed ({why})")
            elif ok and failing:
                log(self.id, "heartbeat ok again")
            if audience is not None and audience != heard:
                log(self.id, describe(audience))
                heard = audience

            failing = not ok
            nap(HEARTBEAT_SECONDS, self.alive)


def describe(audience):
    """a heartbeat's answer, {"listeners": 2, "museum": true, "led": "#ff00ff"}, in words"""
    n = audience.get("listeners", 0)
    led = f", led {audience['led']}" if audience.get("led") else ""
    if not n:
        return f"nobody listening{led}"
    museum = ", the museum among them" if audience.get("museum") else ""
    return f"{n} listening{museum}{led}"


def log(node_id, message):
    print(f"{time.strftime('%H:%M:%S')} {node_id:>10}  {message}", flush=True)


def nap(seconds, alive):
    """sleep, but wake up early once the node is told to stop"""
    end = time.monotonic() + seconds
    while alive() and time.monotonic() < end:
        time.sleep(min(1, end - time.monotonic()))


def main():
    settings, pianos = config.load(description=__doc__)
    try:
        wav_voice = WavVoice.from_file(settings.wav) if settings.wav else None
    except ValueError as error:
        raise SystemExit(str(error)) from error
    threads = []
    for piano in pianos:
        node = Node.from_piano(piano, settings.server, settings.token)
        if piano.id == settings.wav_piano:
            node.wav_voice = wav_voice
        threads.append(node.start(settings))
    # each node's thread ends when it stops; Ctrl+C interrupts the wait, and the daemon
    # threads end with the program
    try:
        for thread in threads:
            thread.join()
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
