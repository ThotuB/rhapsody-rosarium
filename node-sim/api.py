"""what a piano node sends to the server, one function per endpoint.

both need the piano's id and its node token from the server's pianos.json;
the server answers 401 when they don't match.
"""

import mimetypes

import requests


def post_heartbeat(
    session: requests.Session, server: str, node_id: str, token: str, timeout: float = 5
) -> requests.Response:
    """POST <server>/heartbeat: the node is online, even while nobody plays.

    200 when accepted, with who is listening to the piano right now and the colour
    the museum set for its light: {"listeners": 2, "museum": true, "led": "#ff00ff"},
    listeners counts the museum too.
    """
    return session.post(
        f"{server}/heartbeat",
        headers=_auth(token),
        data={"node": node_id},
        timeout=timeout,
    )


def post_segment(
    session: requests.Session,
    server: str,
    node_id: str,
    token: str,
    name: str,
    audio: bytes,
    seq: int | None = None,
    timeout: float = 15,
) -> requests.Response:
    """POST <server>/segments: one recorded segment, in any format ffmpeg reads.

    seq is the window number, counting up from 0 since the node started. listeners
    play consecutive numbers back to back and leave a short pause where one is
    missing, instead of gluing across the gap.

    200 with the stored segment as json, 422 if the audio can't be decoded,
    400 if seq isn't a whole number from 0 up.
    """
    content_type = mimetypes.guess_type(name)[0] or "application/octet-stream"
    data = {"node": node_id}
    if seq is not None:
        data["seq"] = str(seq)
    return session.post(
        f"{server}/segments",
        headers=_auth(token),
        data=data,
        files={"audio": (name, audio, content_type)},
        timeout=timeout,
    )


def _auth(token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}
