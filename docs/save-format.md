# Save format

Checked against game version 1.006 (save version `1.006`).

## Files

| File | Content |
|---|---|
| `Steam_N.dat` | the save of slot N, Odin Serializer binary stream, no compression, no checksum |
| `Steam_N.info` | JSON summary shown in the load menu (day, date, quality values) |
| `Steam_N_backup_{1,2,3}.*` | the game's own rotating backups, newest is `_1` |
| `steam_autocloud.vdf` | marks the folder for Steam Auto-Cloud |

## Odin binary in short

Each entry starts with one byte. The ones that matter here:

| Byte | Entry | Payload |
|---|---|---|
| 1 / 2 | start of reference node (named / unnamed) | name, type entry, int32 reference id |
| 3 / 4 | start of struct node | name, type entry |
| 5 | end of node | |
| 6 / 7 | start / end of array | int64 length |
| 8 | primitive array | int32 count, int32 element size, raw data |
| 23 / 24 | int32 | name (if named), value |
| 31 / 32 | float32 | |
| 33 / 34 | float64 | |
| 39 / 40 | string | name, string |
| 43 / 44 | bool | |
| 45 / 46 | null | |
| 47 / 48 | type name / type id | int32 id (+ string for 47) |

Named entries have odd codes and are followed by the field name. Strings are a flag byte
(0 = 8-bit, 1 = UTF-16), an int32 character count and the characters.

There are no absolute offsets, so inserting or removing a node only requires two things:
the parent array length has to match, and reference ids (entries 1 and 2) have to stay unique.

## Fields gk2 edits

Paths are from the root `GameSave` object.

| What | Path | Type |
|---|---|---|
| money | `playerData/res/resValues[]` where `type == "money"`, field `value` | float32, copper |
| tech points | same list, `tech_red`, `tech_green`, `tech_blue` | float32 |
| backpack | `playerData/inventory/inventoryItem/inventory[]` | `Item {id, count, uniqueId, ...}` |
| tool belt | `playerData/toolBeltInventory/inventoryItem/inventory[]` | same |
| hotbar pins | `playerData/pinnedItems` | string array of item ids |
| talent branches | `talentSystemData/talentData[]` | see below |
| zombies | `worldData/.../wgoDataList[]` objects of type `ZombieWgoData` | see below |

Backpack and belt holders also have `inventorySize` (slots) and `inventoryFillSize` (used slots).

### Talents

`talent_orange`, `talent_red`, `talent_green`, `talent_yellow`, `talent_blue`, each with:

- `curExp`: experience towards the next level;
- `curTalentLevel`: level; reaching the threshold raises it and adds one `talentExpPoints`;
- `talentExpPoints`: free points spent on level-ups;
- `curTalentValue`: bonus from bought level-ups; `studiedLevelUps`: their ids;
- `inspirationsProgression[]`: `{id, currentValue, completionGoalValue}`. Buying an inspiration adds the next
  level's goal to `completionGoalValue`, so edit `currentValue` only.

### Zombies

- `zombieType`: 0 Free, 1 Crafter, 2 Caretaker, 3 ConveyorCrafter, 4 Worker, 5 Porter, 6 Gardener,
  7 ConveyorTransporter, 8 Fighter.
- `techRed`, `techBlue`, `techGreen`: the zombie's own tech points, spent on its level-ups.
- `zombieItem/inventory[]`: body parts named `part_R_W:tier`, where R is red skulls and W white skulls.
- The skill slot limit is the sum of red skulls times stack count over the body parts, capped at 999.
  Learned skills are counted over `talentData[].studiedLevelUps`.

## Game data

Item, craft and building ids come from one large `MonoBehaviour` in
`GraveyardKeeper2_Data/resources.assets`. `gk2` finds it by content, not by object id, and caches it
in the user cache folder. `gk2 find --refresh` rebuilds the cache after a game update.
