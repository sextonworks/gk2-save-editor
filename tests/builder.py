import struct
from pathlib import Path

ITEM_T = "Item, Assembly-CSharp"
LIST_ITEM_T = "System.Collections.Generic.List`1[[Item, Assembly-CSharp]], mscorlib"
PROPS_T = "System.Collections.Generic.Dictionary`2[[System.Type, mscorlib],[SerializedItemProperty, Assembly-CSharp]]"


class OdinWriter:
    def __init__(self):
        self.b = bytearray()
        self.types: dict[str, int] = {}
        self.refs = 0

    def _s(self, v: str):
        self.b += bytes([1]) + struct.pack("<i", len(v)) + v.encode("utf-16-le")

    def _type(self, t: str):
        if t in self.types:
            self.b += bytes([48]) + struct.pack("<i", self.types[t])
        else:
            self.types[t] = len(self.types)
            self.b += bytes([47]) + struct.pack("<i", self.types[t])
            self._s(t)

    def ref(self, name: str | None, t: str):
        if name is None:
            self.b.append(2)
        else:
            self.b.append(1)
            self._s(name)
        self._type(t)
        self.refs += 1
        self.b += struct.pack("<i", self.refs)
        return self

    def end(self):
        self.b.append(5)
        return self

    def array(self, n: int):
        self.b.append(6)
        self.b += struct.pack("<q", n)
        return self

    def end_array(self):
        self.b.append(7)
        return self

    def i32(self, name: str, v: int):
        self.b.append(23)
        self._s(name)
        self.b += struct.pack("<i", v)
        return self

    def f32(self, name: str, v: float):
        self.b.append(31)
        self._s(name)
        self.b += struct.pack("<f", v)
        return self

    def string(self, name: str | None, v: str):
        if name is None:
            self.b.append(40)
        else:
            self.b.append(39)
            self._s(name)
        self._s(v)
        return self

    def item(self, item_id: str, count: int, parts: list[tuple[str, int]] | None = None):
        self.ref(None, ITEM_T).string("id", item_id).i32("count", count)
        self.ref("uniqueId", "SGuid, Assembly-CSharp").string(
            "id", "00000000-0000-0000-0000-00000000000" + str(self.refs % 10)
        ).end()
        self.ref("inventory", LIST_ITEM_T).array(len(parts or []))
        for pid, pc in parts or []:
            self.item(pid, pc)
        self.end_array().end()
        self.i32("inventorySize", 0).i32("inventoryFillSize", -1)
        self.ref("properties", PROPS_T).array(0).end_array().end()
        return self.end()

    def container(self, field: str, items: list[tuple[str, int]], size: int):
        self.ref(field, "Inventory, Assembly-CSharp")
        self.ref("inventoryItem", ITEM_T).string("id", field).i32("count", 1)
        self.ref("inventory", LIST_ITEM_T).array(len(items))
        for iid, c in items:
            self.item(iid, c)
        self.end_array().end()
        self.i32("inventorySize", size).i32("inventoryFillSize", len(items))
        return self.end().end()


def build_save(bag=None, belt=None, money=861.0, bag_size=25) -> bytes:
    bag = [("faith", 99), ("salt", 1), ("heal_potion", 3), ("heal_potion", 2)] if bag is None else bag
    belt = [("hand_tool", 1), ("axe_1", 1), ("sword_0", 1)] if belt is None else belt
    w = OdinWriter()
    w.ref(None, "GameSave, Assembly-CSharp").string("gameSaveVersion", "1.006")
    w.ref("worldData", "WorldData, Assembly-CSharp").ref("wgoDataList", "List").array(1)
    w.ref(None, "ZombieWgoData, Assembly-CSharp").string("name", "zombie_name_1").i32("zombieType", 1)
    w.string("worldZoneDataId", "yard")
    w.ref("zombieItem", ITEM_T).string("id", "body_zombie").i32("count", 1)
    w.ref("inventory", LIST_ITEM_T).array(3)
    for pid in ("skin_0_0:1", "brain_1_0:1", "guts_0_0:1"):
        w.item(pid, 1)
    w.end_array().end().i32("inventorySize", 0).i32("inventoryFillSize", -1)
    w.ref("properties", PROPS_T).array(0).end_array().end().end()
    w.i32("techRed", 10).i32("techBlue", 0).i32("techGreen", 5).end()
    w.end_array().end().end()
    w.ref("playerData", "PlayerData, Assembly-CSharp")
    w.container("inventory", bag, bag_size)
    w.container("toolBeltInventory", belt, 14)
    w.ref("res", "LazyBearTechnology.GameRes, LazyBearTechnology").ref("resValues", "List").array(3)
    for t, v in (("money", money), ("tech_red", 100.0), ("energy", 95.5)):
        w.ref(None, "GameResAtom").string("type", t).f32("value", v).end()
    w.end_array().end().end()
    w.end()
    w.ref("talentSystemData", "TalentSystemData, Assembly-CSharp").ref("talentData", "List").array(2)
    for tid, pts in (("talent_orange", 2), ("talent_red", 3)):
        w.ref(None, "TalentData, Assembly-CSharp").string("id", tid).i32("curExp", 1).i32("curTalentLevel", 4)
        w.i32("talentExpPoints", pts).i32("curTalentValue", 1).end()
    w.end_array().end().end()
    w.end()
    return bytes(w.b)


def write_save(folder: Path, name: str = "Steam_1", **kw) -> Path:
    folder.mkdir(parents=True, exist_ok=True)
    path = folder / f"{name}.dat"
    path.write_bytes(build_save(**kw))
    (folder / f"{name}.info").write_text('{"day": 7, "gameSaveVersion": "1.006"}', encoding="utf-8")
    return path
