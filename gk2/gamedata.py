import re
import struct
from pathlib import Path

from .locate import CACHE_DIR, unity_version

_ID = re.compile(rb"([\x03-\x60])\x00\x00\x00([a-z][a-z0-9_:]+)")
_MARKERS = (b"\x05\x00\x00\x00u_bag", b"\x07\x00\x00\x00sword_0", b"\x0b\x00\x00\x00skull_0_0:1")


def _cache_file(resources: Path) -> Path:
    st = resources.stat()
    return CACHE_DIR / f"balance-{st.st_size}-{int(st.st_mtime)}.bin"


def balance(game_dir: Path, refresh: bool = False) -> bytes:
    resources = game_dir / "GraveyardKeeper2_Data/resources.assets"
    cache = _cache_file(resources)
    if cache.exists() and not refresh:
        return cache.read_bytes()
    import UnityPy
    import UnityPy.config

    vars(UnityPy.config)["FALLBACK_UNITY_VERSION"] = unity_version(game_dir)
    env = UnityPy.load(str(resources))
    best = b""
    for obj in env.objects:
        if obj.type.name != "MonoBehaviour":
            continue
        raw = obj.get_raw_data()
        if len(raw) > len(best) and all(m in raw for m in _MARKERS):
            best = raw
    if not best:
        raise RuntimeError(f"item definitions not found in {resources}")
    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    for old in CACHE_DIR.glob("balance-*.bin"):
        old.unlink()
    cache.write_bytes(best)
    return best


def ids(game_dir: Path, refresh: bool = False) -> dict[str, int]:
    counts: dict[str, int] = {}
    for m in _ID.finditer(balance(game_dir, refresh)):
        n, raw = m.group(1)[0], m.group(2)
        if len(raw) >= n:
            s = raw[:n].decode()
            counts[s] = counts.get(s, 0) + 1
    return counts


def is_bag_item(game_dir: Path, item_id: str) -> bool:
    data = balance(game_dir)
    b = item_id.encode()
    key = struct.pack("<i", len(b)) + b
    start = 0
    while (i := data.find(key, start)) >= 0:
        tail = data[i + 4 + len(b) : i + 4 + len(b) + 16]
        if b"u_bag" in tail:
            return True
        start = i + 1
    return False
