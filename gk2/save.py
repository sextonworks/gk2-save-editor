import json
import shutil
import struct
import time
import uuid
from pathlib import Path

from . import odin
from .locate import BACKUP_DIR, game_running

BEST_BODY = {
    "skin": "skin_3_0:3",
    "bones": "bones_3_1:3",
    "skull": "skull_3_2:3",
    "heart": "heart_3_1:3",
    "brain": "brain_3_1:3",
    "guts": "guts_3_0:3",
}
BEST_BELT = {
    "axe": "axe_3",
    "pickaxe": "pickaxe_3",
    "hammer": "hammer_3",
    "shovel": "shovel_3",
    "small_instruments": "small_instruments_3",
    "sword": "sword_4",
    "armor": "armor_4",
    "bow": "bow_3",
    "surgical_kit": "surgical_kit_3",
    "alchemy_kit": "alchemy_kit_3",
    "tool_book": "tool_book_3",
    "tool_talisman": "tool_talisman_3",
}
CONTAINERS = {"bag": "inventory", "belt": "toolBeltInventory"}
NUMBER_FORMATS = {"i": "<i", "f": "<f", "d": "<d"}


class SaveError(Exception):
    pass


def wait_for_game_exit(poll: float = 3.0, settle: float = 5.0) -> None:
    while game_running():
        time.sleep(poll)
    time.sleep(settle)


def child(node, name):
    for c in node.children:
        if c.name == name:
            return c
    raise SaveError(f"{node.path() or node.name}: no field '{name}'")


def array_of(node):
    for c in node.children:
        if c.kind == "array":
            return c
    raise SaveError(f"{node.path()}: no array")


class Save:
    def __init__(self, path: Path):
        self.path = Path(path)
        self.info_path = self.path.with_suffix(".info")
        self.data = bytearray(self.path.read_bytes())
        self.changes: list[str] = []
        self.reparse()

    def reparse(self):
        self.root = odin.Reader(bytes(self.data)).parse()
        if not self.root.children:
            raise SaveError(f"{self.path}: empty save")
        self.game = self.root.children[0]

    def info(self) -> dict:
        try:
            return json.loads(self.info_path.read_text(encoding="utf-8-sig"))
        except (OSError, ValueError):
            return {}

    def section(self, name):
        return child(self.game, name)

    def _str_len(self, node) -> int:
        flag = self.data[node.voff]
        n = struct.unpack_from("<i", self.data, node.voff + 1)[0]
        return 5 + n * (1 if flag == 0 else 2)

    @staticmethod
    def _str_bytes(flag: int, value: str) -> bytes:
        enc = "latin-1" if flag == 0 else "utf-16-le"
        return bytes([flag]) + struct.pack("<i", len(value)) + value.encode(enc)

    def set_str(self, node, value: str):
        if node.type != "str":
            raise SaveError(f"{node.path()} is not a string")
        old = node.value
        start = node.voff
        self.data[start : start + self._str_len(node)] = self._str_bytes(self.data[start], value)
        self.changes.append(f"{node.path()}: {old!r} -> {value!r}")
        self.reparse()

    def set_number(self, node, value):
        fmt = NUMBER_FORMATS.get(node.type)
        if fmt is None:
            raise SaveError(f"{node.path()} is not a number")
        value = int(value) if node.type == "i" else float(value)
        struct.pack_into(fmt, self.data, node.voff, value)
        self.changes.append(f"{node.path()}: {node.value} -> {value}")
        node.value = value

    def player(self):
        return self.section("playerData")

    def container(self, which: str):
        if which not in CONTAINERS:
            raise SaveError(f"unknown container '{which}', use: {', '.join(CONTAINERS)}")
        holder = child(child(self.player(), CONTAINERS[which]), "inventoryItem")
        return holder, array_of(child(holder, "inventory"))

    def items(self, which: str):
        _, arr = self.container(which)
        return [(child(i, "id").value, child(i, "count").value, i) for i in arr.children]

    def find_items(self, which: str, item_id: str):
        found = [i for iid, _, i in self.items(which) if iid == item_id]
        if not found:
            raise SaveError(f"'{item_id}' is not in the {which}")
        return found

    def find_item(self, which: str, item_id: str):
        return self.find_items(which, item_id)[0]

    def _template(self, arr):
        for i in arr.children:
            if not array_of(child(i, "inventory")).children and not array_of(child(i, "properties")).children:
                return i
        raise SaveError("no plain item to copy the layout from, the container must hold at least one simple item")

    def add_item(self, which: str, item_id: str, count: int = 1):
        holder, arr = self.container(which)
        size = child(holder, "inventorySize").value
        if len(arr.children) >= size:
            raise SaveError(f"{which} is full ({len(arr.children)}/{size})")
        tpl = self._template(arr)
        buf = bytearray(self.data[tpl.off : tpl.end])
        next_ref = max(n.refid for n in odin.walk(self.root) if n.refid is not None)
        for r in (n for n in odin.walk(tpl) if n.kind == "ref"):
            next_ref += 1
            struct.pack_into("<i", buf, r.refoff - tpl.off, next_ref)
        struct.pack_into("<i", buf, child(tpl, "count").voff - tpl.off, count)
        edits = [(child(child(tpl, "uniqueId"), "id"), str(uuid.uuid4())), (child(tpl, "id"), item_id)]
        for node, value in edits:
            s = node.voff - tpl.off
            buf[s : s + self._str_len(node)] = self._str_bytes(self.data[node.voff], value)
        fill = child(holder, "inventoryFillSize")
        struct.pack_into("<q", self.data, arr.off + 1, len(arr.children) + 1)
        if fill.type == "i" and fill.value >= 0:
            struct.pack_into("<i", self.data, fill.voff, fill.value + 1)
        insert_at = arr.children[-1].end
        self.data[insert_at:insert_at] = bytes(buf)
        self.changes.append(f"{which}: + {item_id} x{count}")
        self.reparse()

    def remove_item(self, which: str, item_id: str):
        holder, arr = self.container(which)
        node = self.find_item(which, item_id)
        fill = child(holder, "inventoryFillSize")
        struct.pack_into("<q", self.data, arr.off + 1, len(arr.children) - 1)
        if fill.type == "i" and fill.value > 0:
            struct.pack_into("<i", self.data, fill.voff, fill.value - 1)
        del self.data[node.off : node.end]
        self.changes.append(f"{which}: - {item_id}")
        self.reparse()

    def res(self) -> list:
        values = child(child(self.player(), "res"), "resValues")
        return [(child(a, "type").value, child(a, "value")) for a in array_of(values).children]

    def res_node(self, res_type: str):
        for t, node in self.res():
            if t == res_type:
                return node
        raise SaveError(f"player resource '{res_type}' not found")

    def talents(self):
        return array_of(child(self.section("talentSystemData"), "talentData")).children

    def zombies(self):
        return [n.parent for n in odin.walk(self.section("worldData")) if n.name == "zombieType"]

    def write(self, backup_dir: Path = BACKUP_DIR) -> Path | None:
        if not self.changes:
            return None
        check = odin.Reader(bytes(self.data)).parse()
        ids = [n.refid for n in odin.walk(check) if n.refid is not None]
        if len(ids) != len(set(ids)):
            raise SaveError("duplicate reference ids after the edit, nothing written")
        bk = backup_dir / time.strftime("%Y%m%d_%H%M%S")
        suffix = 1
        while bk.exists():
            bk = backup_dir / f"{time.strftime('%Y%m%d_%H%M%S')}_{suffix}"
            suffix += 1
        bk.mkdir(parents=True)
        shutil.copy2(self.path, bk / self.path.name)
        if self.info_path.exists():
            shutil.copy2(self.info_path, bk / self.info_path.name)
        self.path.write_bytes(self.data)
        return bk
