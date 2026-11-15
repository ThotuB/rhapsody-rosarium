"""everything the simulator starts from: command-line flags, a json scenario
(--config, see configs/), and the pianos to simulate.

load() is the only thing simulate.py needs: it merges the scenario with the
flags (flags win) and returns ready-to-use settings and pianos.
"""

import argparse
import json
from dataclasses import dataclass
from pathlib import Path

CONFIG_KEYS = {"server", "token", "segment_seconds", "fast", "pianos", "wav", "wav_piano"}
PIANO_KEYS = {"id", "mute", "stop_after", "token"}


@dataclass
class Settings:
    """shared by every simulated node"""

    server: str  # base url without a trailing /, e.g. http://localhost:8080/api
    token: str  # default node token, {id} is replaced with the piano id
    segment_seconds: float
    fast: bool  # don't wait out segment durations, upload immediately
    wav: Path | None = None
    wav_piano: str = "piano-2"


@dataclass
class Piano:
    """one piano as the scenario describes it"""

    id: str
    mute: bool = False  # heartbeats only, never plays
    stop_after: float | None = None  # seconds until the node stops and goes offline
    token: str | None = None  # instead of the default token


def load(description: str | None = None) -> tuple[Settings, list[Piano]]:
    """parse the command line, read --config if given, and resolve the pianos"""
    p = parser(description)

    # the config's settings become the defaults, so flags still win
    pre, _ = p.parse_known_args()
    scenario = read_scenario(pre.config) if pre.config else {}
    p.set_defaults(**{k: v for k, v in scenario.items() if k != "pianos"})
    args = p.parse_args()

    settings = Settings(
        server=args.server.rstrip("/"),
        token=args.token,
        segment_seconds=args.segment_seconds,
        fast=args.fast,
        wav=Path(args.wav).expanduser().resolve() if args.wav else None,
        wav_piano=args.wav_piano,
    )
    pianos = resolve_pianos(args, scenario.get("pianos", []))
    if not 0 < settings.segment_seconds <= 180:
        p.error("--segment-seconds must be between 0 and 180")
    if settings.wav:
        if not settings.wav.is_file() or settings.wav.suffix.lower() != ".wav":
            p.error(f"WAV file does not exist or is not a .wav: {settings.wav}")
        target = next((piano for piano in pianos if piano.id == settings.wav_piano), None)
        if target is None or target.mute:
            p.error("--wav-piano must name an active, unmuted piano in this simulator")
    return settings, pianos


def parser(description: str | None) -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        description=description, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    p.add_argument(
        "--server",
        default="http://localhost:8080/api",
        help="base url, segments go to <server>/segments",
    )
    p.add_argument(
        "--id",
        action="append",
        dest="ids",
        help="node id, can be repeated; overrides --nodes and --prefix",
    )
    p.add_argument("--nodes", type=int, default=10, help="number of fake pianos")
    p.add_argument("--prefix", default="piano-", help="node ids are <prefix>1..N")
    p.add_argument(
        "--token", default="dev", help="node token, {id} is replaced with the node id"
    )
    p.add_argument(
        "--segment-seconds",
        type=float,
        default=2,
        help="length of each segment; listeners hear a piano about 2-3 of these late",
    )
    p.add_argument(
        "--fast",
        action="store_true",
        help="don't wait out segment durations, upload immediately",
    )
    p.add_argument(
        "--mute",
        action="append",
        default=[],
        help="node id that only heartbeats and never plays, can be repeated",
    )
    p.add_argument(
        "--config",
        type=Path,
        help="json scenario: settings and which pianos to simulate, see configs/",
    )
    p.add_argument("--wav", type=Path, help="WAV recording to loop instead of generated music on one piano")
    p.add_argument("--wav-piano", default="piano-2", help="piano receiving --wav (default: piano-2)")
    return p


def read_scenario(path: Path) -> dict:
    """a --config file, checked, with its pianos as Piano objects"""
    try:
        scenario = json.loads(path.read_text())
    except (OSError, json.JSONDecodeError) as e:
        raise SystemExit(f"{path}: {e}")
    if unknown := set(scenario) - CONFIG_KEYS:
        raise SystemExit(
            f"{path}: unknown keys {sorted(unknown)}, expected some of {sorted(CONFIG_KEYS)}"
        )

    pianos = []
    for piano in scenario.get("pianos", []):
        if "id" not in piano or set(piano) - PIANO_KEYS:
            raise SystemExit(
                f"{path}: each piano needs an id and can have {sorted(PIANO_KEYS - {'id'})}, got {piano}"
            )
        stop_after = piano.get("stop_after")
        pianos.append(
            Piano(
                id=str(piano["id"]),
                mute=bool(piano.get("mute", False)),
                stop_after=None if stop_after is None else float(stop_after),
                token=piano.get("token"),
            )
        )
    scenario["pianos"] = pianos
    return scenario


def resolve_pianos(args: argparse.Namespace, configured: list[Piano]) -> list[Piano]:
    """pianos from the config and --id, or else --nodes of them; --mute applies to either"""
    pianos = {piano.id: piano for piano in configured}
    for node_id in args.ids or []:
        pianos.setdefault(node_id, Piano(node_id))
    if not pianos:
        ids = [f"{args.prefix}{i + 1}" for i in range(args.nodes)]
        pianos = {node_id: Piano(node_id) for node_id in ids}
    for node_id in args.mute:
        pianos.setdefault(node_id, Piano(node_id)).mute = True
    return list(pianos.values())
