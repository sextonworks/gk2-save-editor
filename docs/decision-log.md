# Decision log

## 2026-09-27: edits re-parse the whole save

After every structural edit (string length change, insert, remove) the save is parsed again from scratch.
It costs well under a second on a 12 MB save and removes a whole class of stale offset bugs.
Commands that loop over nodes re-fetch them by index after each edit for the same reason.

## 2026-09-27: new items copy an existing plain item

`add` copies the bytes of an item already in the same container (or, for an empty chest, of any plain item earlier in the stream) that has no nested inventory and
no properties, then rewrites the id, count, unique id and all reference ids. Building the stream from scratch
would need the exact type ids the save already uses, and copying keeps them right by construction.

## 2026-09-28: game data is read by type tree schemas

GameBalance and the sprite, atlas and texture objects are read with field schemas embedded in the binary
(`internal/typetree/schema`), not by scanning bytes. A read must consume the object exactly; if a game update
changes the layout the editor falls back to the old id scan and shows no icons instead of reading garbage.
The GameBalance schema comes from the game's own assemblies (UnityPy `TypeTreeGenerator`), the built-in class
schemas from UnityPy's type tree database for Unity 6000.3.9f1. The generator script left with the Python code;
it is in git history as `tools/typetree_schema.py` at commit e05d29d.

## 2026-09-28: icons are cut from the installed game

Item icons live in two sprite atlases: `Icons` in `sharedassets0.assets` and `IconsCompressed` in one of the
Addressables bundles. The bundle is found by scanning bundle headers for a SpriteAtlas (about half a second for
25k bundles), then the icons are written as PNG files to the cache, keyed by the game data hash.
No game art is shipped with the editor.

## 2026-09-28: Python version removed

The Go CLI and desktop editor replaced the Python package after a byte-for-byte parity check on a real save
(parser dump of 409,850 nodes and the output of eight write commands).
