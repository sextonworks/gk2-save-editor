import json
import sys
from pathlib import Path

from UnityPy.helpers.Tpk import get_typetree_node
from UnityPy.helpers.TypeTreeGenerator import TypeTreeGenerator

UNITY = "6000.3.9f1"
BUILTIN = {"Texture2D": 28, "Sprite": 213, "SpriteAtlas": 687078895}


def rows_of(node, out):
    out.append([node.m_Level, node.m_Type, node.m_Name, node.m_MetaFlag or 0])
    for child in node.m_Children:
        rows_of(child, out)
    return out


def main(game_data, out_dir):
    out = Path(out_dir)
    gen = TypeTreeGenerator(UNITY)
    gen.load_local_dll_folder(str(Path(game_data) / "Managed"))
    nodes = gen.get_nodes("Assembly-CSharp.dll", "GameBalance")
    rows = [[n.m_Level, n.m_Type, n.m_Name, n.m_MetaFlag] for n in nodes]
    (out / "GameBalance.json").write_text(json.dumps(rows, separators=(",", ":")) + "\n")
    version = tuple(int(p) for p in UNITY.replace("f", ".").split(".")[:3]) + (0,)
    for name, class_id in BUILTIN.items():
        rows = rows_of(get_typetree_node(class_id, version), [])
        (out / f"{name}.json").write_text(json.dumps(rows, separators=(",", ":")) + "\n")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
