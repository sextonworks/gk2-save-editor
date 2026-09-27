<p align="center">
  <img src="docs/assets/icon.png" width="160" alt="gk2 icon: a gravestone with a terminal prompt">
</p>

# gk2: save editor for Graveyard Keeper 2

A small command-line tool that reads and edits Graveyard Keeper 2 save files:
items in the backpack and on the tool belt, money, player resources, talent points and zombies.
Every write makes a backup first, and the tool refuses to touch the save while the game is running.

This is an unofficial fan project. It is not affiliated with or endorsed by Lazy Bear Games or tinyBuild.
It ships no game files; the item list used for id checks is read from your own installation.

## Install

Requires Python 3.12+ and [uv](https://docs.astral.sh/uv/) (or pipx).

Latest code from GitHub:

```bash
uv tool install git+https://github.com/sextonworks/gk2-save-editor
gk2 --help
```

A tagged version: download the `.whl` file from [Releases](https://github.com/sextonworks/gk2-save-editor/releases)
and install it with `uv tool install ./gk2_save_editor-<version>-py3-none-any.whl`.

From a local checkout: `uv tool install -e .`

## Where it looks for files

`gk2 paths` prints what it found. Save and game folders are detected automatically for:

| Platform | Save folder |
|---|---|
| Windows | `%USERPROFILE%\AppData\LocalLow\Lazy Bear Games\Graveyard Keeper 2` |
| Linux (Steam Proton) | `<steam library>/steamapps/compatdata/4358690/pfx/drive_c/users/steamuser/AppData/LocalLow/...` |
| macOS (CrossOver) | `~/Library/Application Support/CrossOver/Bottles/<bottle>/drive_c/users/<user>/AppData/LocalLow/...` |

If detection fails, pass `--save-dir` and `--game-dir` or set `GK2_SAVE_DIR` and `GK2_GAME_DIR`.
Backups go to the user data folder (`gk2 paths` shows it), override with `GK2_DATA_DIR`.

## Before you edit

1. Save the game and quit to the desktop. Edits made while the game runs are overwritten by the next save.
   `gk2 -w <command>` waits for the game to close and then applies the change.
2. If Steam asks about a cloud save conflict on the next start, pick the local copy.
3. Try any command with `-n` first: it shows the changes without writing.

## Commands

```bash
gk2 info                                  # day, money, backpack fill, game status
gk2 bag                                   # backpack contents
gk2 belt                                  # tool belt (tools, weapon, armor)
gk2 find '^candle_'                       # search ids in the game data
gk2 add candle_basic=5 heal_potion        # add stacks to the backpack
gk2 count heal_potion 20                  # set a stack size (--all for every stack)
gk2 remove inq_note_1                     # remove a stack
gk2 swap sword_3 sword_4 --where belt     # replace an item in place
gk2 money 5000                            # money in copper (100 copper = 1 silver)
gk2 res tech                              # player resources matching a regex
gk2 set-res tech_red 999                  # set a player resource
gk2 talents                               # talent branches and free points
gk2 set-talents 99                        # free talent points in every branch
gk2 zombies                               # the player's zombies
gk2 zombies-max                           # best body parts, max skill slots and tech points
gk2 equip-best                            # top tier on every tool belt slot
gk2 backups                               # list backups
gk2 restore 20260926_170215               # restore one (the current save is backed up first)
```

Global options: `--slot Steam_2` for another slot, `--save path/to/file.dat` for any file,
`-n` dry run, `-w` wait for the game to close.

## Notes and limits

- Money and other resources are stored as 32-bit floats. Values above 16,777,216 lose precision.
- Items added with `add` are plain stacks without extra properties (quality, durability).
- `zombies-max` gives each zombie 328 brains, which sets the skill slot limit to the game cap of 999.
  Disassembling such a zombie may return all of them.
- Tested with game version 1.006 on macOS (CrossOver). Windows and Linux paths follow the standard
  Steam layout but have not been tested on real machines yet; reports are welcome.

## How it works

Saves are [Odin Serializer](https://github.com/TeamSirenix/odin-serializer) binary streams.
`gk2` parses them into a tree, edits values in place, and can insert or remove whole nodes because the format
has no absolute offsets. See [docs/save-format.md](docs/save-format.md) for the layout of the fields it touches.

## Development

```bash
uv sync
uv run pytest
uv run ruff check . && uv run ruff format --check .
uv run ty check
prek install        # git hooks
```

The `tools/` folder has small scripts for reading the game's `Assembly-CSharp.dll`
(CIL disassembly, field and string usage). They are handy when a game update moves things around.

## License

MIT, see [LICENSE](LICENSE).
