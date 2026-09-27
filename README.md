<p align="center">
  <img src="docs/assets/icon.png" width="160" alt="gk2 icon: a gravestone with a terminal prompt">
</p>

# gk2: save editor for Graveyard Keeper 2

A desktop editor and a command-line tool for Graveyard Keeper 2 save files: money, the backpack and tool belt,
every chest in the world, player stats and reputation, talent points, inspirations and zombies.
Item names and icons are read from your own game installation, in English, Russian or Chinese.
Every write makes a backup first, and nothing is written while the game is running.

This is an unofficial fan project. It is not affiliated with or endorsed by Lazy Bear Games or tinyBuild.
It ships no game files or game art.

## Desktop editor

`cmd/gk2-editor` is a native window (Wails) with pages for the character, inventory, storage (chests grouped by
zone), zombies, inspirations, technologies, a read-only inspector of every save field, and backups.
Changes pile up in a journal on the right: undo and redo them, then press Save to write the file and a backup.

Download `gk2-editor` for your platform from [Releases](https://github.com/sextonworks/gk2-save-editor/releases):
Windows x64, macOS (Apple Silicon and Intel) and Linux (x64 and arm64). To build it from source you need Go 1.27,
Node 24 with pnpm, and the Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
cd cmd/gk2-editor
wails build                      # Linux: wails build -tags webkit2_41
```

The app appears in `cmd/gk2-editor/build/bin`. On Linux it needs WebKitGTK 4.1
(`libwebkit2gtk-4.1-dev` and `libgtk-3-dev` on Debian and Ubuntu). The macOS build is not signed:
open it with right click, Open the first time.

## Command-line tool

Download `gk2` for your platform from [Releases](https://github.com/sextonworks/gk2-save-editor/releases),
or install it with Go:

```bash
go install github.com/sextonworks/gk2-save-editor/cmd/gk2@latest
gk2 --help
```

```bash
gk2 info                                  # day, money, backpack fill, game status
gk2 bag                                   # backpack contents
gk2 belt                                  # tool belt (tools, weapon, armor)
gk2 find '^candle_'                       # search ids in the game data
gk2 add candle_basic=5 heal_potion        # add stacks to the backpack
gk2 count heal_potion 20                  # set a stack size (--all for every stack)
gk2 remove inq_note_1                     # remove a stack
gk2 swap sword_3 sword_4 --where belt     # replace an item in place
gk2 money 50000                           # money in bronze: 100 bronze = 1 silver, 100 silver = 1 gold
gk2 res tech                              # player resources matching a regex
gk2 set-res tech_red 999                  # set a player resource
gk2 talents                               # talent branches and free points
gk2 set-talents 99                        # free talent points in every branch
gk2 inspirations                          # inspiration progress per branch
gk2 inspire                               # bring unfinished inspirations to their goal
gk2 techs                                 # unlocked, available and hidden technologies
gk2 zombies                               # the player's zombies
gk2 zombies-max                           # best body parts, max skill slots and tech points
gk2 equip-best                            # top tier on every tool belt slot
gk2 backups                               # list backups
gk2 restore 20260926_170215               # restore one (the current save is backed up first)
```

Global options: `--slot Steam_2` for another slot, `--save path/to/file.dat` for any file,
`-n` dry run, `-w` wait for the game to close, `--lang ru` or `--lang zh_cn` for item names.
Chests are edited in the desktop editor only.

## Verify a download

Every file in a release is built by this repository's GitHub Actions workflow and signed with
[Sigstore](https://www.sigstore.dev/): no private key is kept anywhere, the signature is tied to the workflow run.
Check a file with the GitHub CLI:

```bash
gh attestation verify gk2-editor-v0.2.0-windows-amd64.zip --repo sextonworks/gk2-save-editor
```

Or check the checksum list with [cosign](https://docs.sigstore.dev/cosign/system_config/installation/)
and then the file against it:

```bash
cosign verify-blob SHA256SUMS --bundle SHA256SUMS.sigstore.json \
  --certificate-identity-regexp '^https://github.com/sextonworks/gk2-save-editor/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --check --ignore-missing SHA256SUMS
```

The apps are not signed with an Apple or Microsoft certificate, so macOS and Windows warn on the first start.

## Where it looks for files

`gk2 paths` prints what it found. Save and game folders are detected automatically for:

| Platform | Save folder |
|---|---|
| Windows | `%USERPROFILE%\AppData\LocalLow\Lazy Bear Games\Graveyard Keeper 2` |
| Linux (Steam Proton) | `<steam library>/steamapps/compatdata/4358690/pfx/drive_c/users/steamuser/AppData/LocalLow/...` |
| macOS (CrossOver) | `~/Library/Application Support/CrossOver/Bottles/<bottle>/drive_c/users/<user>/AppData/LocalLow/...` |

If detection fails, pick the folders on the Saves page, pass `--save-dir` and `--game-dir`,
or set `GK2_SAVE_DIR` and `GK2_GAME_DIR`. Backups go to the user data folder (`GK2_DATA_DIR` overrides it),
game data and icons are cached in the user cache folder (`GK2_CACHE_DIR`).

## Before you edit

1. Save the game and quit to the desktop. Edits made while the game runs are overwritten by the next save.
2. If Steam asks about a cloud save conflict on the next start, pick the local copy.
3. In the command-line tool, try any command with `-n` first: it shows the changes without writing.

## Notes and limits

- Money and other resources are stored as 32-bit floats. Values above 16,777,216 (1,677 gold) lose precision.
- Added items are plain stacks without extra properties (quality, durability).
- Zone ratings are shown read-only: the game recalculates them from what is built.
- `zombies-max` gives each zombie 328 brains, which sets the skill slot limit to the game cap of 999.
  Disassembling such a zombie may return all of them.
- Tested with game version 1.006 on macOS (CrossOver). Windows and Linux paths follow the standard
  Steam layout but have not been tested on real machines yet; reports are welcome.

## How it works

Saves are [Odin Serializer](https://github.com/TeamSirenix/odin-serializer) binary streams.
`gk2` parses them into a tree, edits values in place, and can insert or remove whole nodes because the format
has no absolute offsets. Game data is read from Unity asset files and Addressables bundles with field schemas
taken from the game's own assemblies. See [docs/save-format.md](docs/save-format.md) for the fields it touches
and [docs/decision-log.md](docs/decision-log.md) for the reasons behind the design.

## Development

```bash
go test -race ./...
golangci-lint run ./...
cd cmd/gk2-editor && wails dev   # desktop editor with live reload
prek install                     # git hooks
```

## License

MIT, see [LICENSE](LICENSE).
