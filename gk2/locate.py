import os
import re
import subprocess
import sys
from collections.abc import Iterator
from pathlib import Path

from platformdirs import user_cache_dir, user_data_dir

APP_ID = "4358690"
GAME_FOLDER = "Graveyard Keeper 2"
EXE_NAME = "GraveyardKeeper2.exe"
SAVE_SUBPATH = Path("AppData/LocalLow/Lazy Bear Games/Graveyard Keeper 2")
DEFAULT_SLOT = "Steam_1"
FALLBACK_UNITY_VERSION = "6000.3.9f1"

DATA_DIR = Path(os.environ.get("GK2_DATA_DIR") or user_data_dir("gk2", appauthor=False))
BACKUP_DIR = DATA_DIR / "backups"
CACHE_DIR = Path(os.environ.get("GK2_CACHE_DIR") or user_cache_dir("gk2", appauthor=False))


class NotFoundError(Exception):
    pass


def _home() -> Path:
    return Path.home()


def _crossover_drives() -> list[Path]:
    bottles = _home() / "Library/Application Support/CrossOver/Bottles"
    return sorted(bottles.glob("*/drive_c")) if bottles.is_dir() else []


def _steam_roots() -> Iterator[Path]:
    if sys.platform == "win32":
        for var in ("PROGRAMFILES(X86)", "PROGRAMFILES"):
            base = os.environ.get(var)
            if base:
                yield Path(base) / "Steam"
    home = _home()
    yield home / ".steam/steam"
    yield home / ".local/share/Steam"
    yield home / ".var/app/com.valvesoftware.Steam/.local/share/Steam"
    yield home / "Library/Application Support/Steam"
    for drive in _crossover_drives():
        yield drive / "Program Files (x86)/Steam"


def _library_dirs() -> Iterator[Path]:
    seen: set[Path] = set()
    for root in _steam_roots():
        if not root.is_dir():
            continue
        candidates = [root]
        vdf = root / "steamapps/libraryfolders.vdf"
        if vdf.is_file():
            text = vdf.read_text(encoding="utf-8", errors="replace")
            candidates += [Path(p.replace("\\\\", "\\")) for p in re.findall(r'"path"\s+"([^"]+)"', text)]
        for lib in candidates:
            key = lib.resolve() if lib.exists() else lib
            if key not in seen:
                seen.add(key)
                yield lib


def save_dir_candidates() -> Iterator[Path]:
    if sys.platform == "win32":
        profile = os.environ.get("USERPROFILE")
        if profile:
            yield Path(profile) / SAVE_SUBPATH
    for lib in _library_dirs():
        yield lib / "steamapps/compatdata" / APP_ID / "pfx/drive_c/users/steamuser" / SAVE_SUBPATH
    for drive in _crossover_drives():
        yield from sorted((drive / "users").glob(f"*/{SAVE_SUBPATH.as_posix()}"))


def game_dir_candidates() -> Iterator[Path]:
    for lib in _library_dirs():
        yield lib / "steamapps/common" / GAME_FOLDER


def find_save_dir(explicit: Path | None = None) -> Path:
    override = explicit or (Path(os.environ["GK2_SAVE_DIR"]) if os.environ.get("GK2_SAVE_DIR") else None)
    if override:
        if not override.is_dir():
            raise NotFoundError(f"Save folder does not exist: {override}")
        return override
    for path in save_dir_candidates():
        if path.is_dir():
            return path
    raise NotFoundError("Save folder not found. Pass --save-dir or set GK2_SAVE_DIR.")


def find_game_dir(explicit: Path | None = None) -> Path:
    override = explicit or (Path(os.environ["GK2_GAME_DIR"]) if os.environ.get("GK2_GAME_DIR") else None)
    if override:
        if not (override / "GraveyardKeeper2_Data").is_dir():
            raise NotFoundError(f"Not a Graveyard Keeper 2 folder: {override}")
        return override
    for path in game_dir_candidates():
        if (path / "GraveyardKeeper2_Data").is_dir():
            return path
    raise NotFoundError("Game folder not found. Pass --game-dir or set GK2_GAME_DIR.")


def unity_version(game_dir: Path) -> str:
    ggm = game_dir / "GraveyardKeeper2_Data/globalgamemanagers"
    try:
        head = ggm.read_bytes()[:4096]
    except OSError:
        return FALLBACK_UNITY_VERSION
    m = re.search(rb"\d{4}\.\d+\.\d+[abfp]\d+", head)
    return m.group().decode() if m else FALLBACK_UNITY_VERSION


def game_running() -> bool:
    if sys.platform == "win32":
        out = subprocess.run(
            ["tasklist", "/FI", f"IMAGENAME eq {EXE_NAME}"], capture_output=True, text=True, check=False
        ).stdout
        return EXE_NAME.lower() in out.lower()
    try:
        return subprocess.run(["pgrep", "-qf", EXE_NAME], check=False).returncode == 0
    except FileNotFoundError:
        return False
