# Decision log

## 2026-09-27: ruff rules that are switched off

- `PLR2004` (magic values): the Odin parser compares entry codes and sizes of a binary format;
  naming every byte value would not make it clearer than the table in `docs/save-format.md`.
- `PLR0911`, `PLR0912`, `PLR0913`, `PLR0915`, `PLR0917`: typer commands take their options as function
  parameters and the parser is one loop over entry codes; splitting them only to satisfy the counter hurts reading.
- `PLW2901`: loop variables are reassigned on purpose after a node is re-parsed.
- `PLC0415` in `gk2/gamedata.py`: UnityPy is imported lazily because it takes about a second to load
  and is only needed when the game data cache is (re)built.
- `tools/` is excluded from ruff: one-off reverse engineering scripts, not part of the package.

## 2026-09-27: edits re-parse the whole save

After every structural edit (string length change, insert, remove) the save is parsed again from scratch.
It costs well under a second on a 12 MB save and removes a whole class of stale offset bugs.
Commands that loop over nodes re-fetch them by index after each edit for the same reason.

## 2026-09-27: new items copy an existing plain item

`add` copies the bytes of an item already in the same container that has no nested inventory and
no properties, then rewrites the id, count, unique id and all reference ids. Building the stream from scratch
would need the exact type ids the save already uses, and copying keeps them right by construction.
