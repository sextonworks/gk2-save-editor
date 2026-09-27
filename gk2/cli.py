import re
import shutil
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path
from typing import Annotated

import typer

from . import __version__, gamedata, locate
from .save import BEST_BELT, BEST_BODY, Save, SaveError, array_of, child, wait_for_game_exit

app = typer.Typer(
    no_args_is_help=True,
    add_completion=False,
    help="Save editor for Graveyard Keeper 2. Every write makes a backup first.",
)

Where = Annotated[str, typer.Option("--where", help="Container: bag or belt.")]
AllStacks = Annotated[bool, typer.Option("--all", help="Apply to every stack with this id.")]
Check = Annotated[bool, typer.Option(help="Refuse ids that are not in the game data (needs the game folder).")]


@dataclass
class Opts:
    slot: str
    save: Path | None
    save_dir: Path | None
    game_dir: Path | None
    dry_run: bool
    wait: bool

    def save_path(self) -> Path:
        if self.save:
            return self.save
        return locate.find_save_dir(self.save_dir) / f"{self.slot}.dat"


def version_callback(value: bool):
    if value:
        typer.echo(f"gk2 {__version__}")
        raise typer.Exit


@app.callback()
def main(
    ctx: typer.Context,
    slot: Annotated[str, typer.Option("--slot", "-s", help="Save slot name.")] = locate.DEFAULT_SLOT,
    save: Annotated[Path | None, typer.Option(help="Path to a .dat save file (overrides --slot).")] = None,
    save_dir: Annotated[Path | None, typer.Option(help="Save folder (default: auto, or GK2_SAVE_DIR).")] = None,
    game_dir: Annotated[Path | None, typer.Option(help="Game folder (default: auto, or GK2_GAME_DIR).")] = None,
    dry_run: Annotated[bool, typer.Option("--dry-run", "-n", help="Show changes without writing.")] = False,
    wait: Annotated[bool, typer.Option("--wait", "-w", help="Wait until the game is closed before editing.")] = False,
    version: Annotated[
        bool, typer.Option("--version", callback=version_callback, is_eager=True, help="Show version and exit.")
    ] = False,
):
    ctx.obj = Opts(slot, save, save_dir, game_dir, dry_run, wait)


def fail(msg: str):
    typer.secho(msg, fg="red", err=True)
    raise typer.Exit(1)


def guard[T](fn: Callable[..., T], *args) -> T:
    try:
        return fn(*args)
    except (SaveError, locate.NotFoundError) as e:
        fail(str(e))
        raise


def game_dir(ctx: typer.Context) -> Path:
    return guard(locate.find_game_dir, ctx.obj.game_dir)


def known_ids(ctx: typer.Context) -> dict[str, int]:
    return gamedata.ids(game_dir(ctx))


def load(ctx: typer.Context, write: bool = False) -> Save:
    o: Opts = ctx.obj
    path = guard(o.save_path)
    if write and not o.dry_run and o.save is None and locate.game_running():
        if not o.wait:
            fail("The game is running. Save, quit to desktop and try again, or pass --wait.")
        typer.echo("Waiting for the game to close...")
        wait_for_game_exit()
    if not path.exists():
        fail(f"Save not found: {path}")
    return guard(Save, path)


def commit(ctx: typer.Context, s: Save):
    for c in s.changes:
        typer.echo(f"  {c}")
    if not s.changes:
        typer.echo("Nothing to change.")
        return
    if ctx.obj.dry_run:
        typer.secho("Dry run, nothing written.", fg="yellow")
        return
    bk = guard(s.write, locate.BACKUP_DIR)
    typer.secho(f"Written. Backup: {bk}", fg="green")


def note_stacks(item_id: str, stacks: list, all_: bool):
    if len(stacks) > 1 and not all_:
        typer.secho(
            f"Note: {len(stacks)} stacks of {item_id}, changing the first one (--all for every stack).", fg="yellow"
        )


def require_known(ctx: typer.Context, item_id: str):
    if item_id not in known_ids(ctx):
        fail(f"'{item_id}' is not a known id. Try: gk2 find {item_id.split('_', maxsplit=1)[0]}")


@app.command(help="Save summary: day, money, backpack, zombies, game status.")
def info(ctx: typer.Context):
    s = load(ctx)
    i = s.info()
    holder, arr = s.container("bag")
    typer.echo(f"Save:     {s.path}")
    typer.echo(f"Day:      {i.get('day')}  saved {i.get('saveDateTime')}  version {i.get('gameSaveVersion')}")
    typer.echo(f"Money:    {s.res_node('money').value}")
    typer.echo(f"Backpack: {len(arr.children)}/{child(holder, 'inventorySize').value}")
    typer.echo(f"Zombies:  {len(s.zombies())}")
    typer.echo(f"Game:     {'RUNNING' if locate.game_running() else 'closed'}")


@app.command(help="List the backpack.")
def bag(ctx: typer.Context):
    for iid, cnt, _ in load(ctx).items("bag"):
        typer.echo(f"{cnt:>6}  {iid}")


@app.command(help="List the tool belt (tools, weapon, armor).")
def belt(ctx: typer.Context):
    for iid, _, _ in load(ctx).items("belt"):
        typer.echo(iid)


@app.command(help="Add new stacks: gk2 add candle_basic=5 heal_potion")
def add(
    ctx: typer.Context,
    items: Annotated[list[str], typer.Argument(help="Item ids, optionally id=count.")],
    where: Where = "bag",
    check: Check = True,
):
    parsed = []
    for spec in items:
        iid, _, n = spec.partition("=")
        if n and not n.isdigit():
            fail(f"Bad count in '{spec}'")
        if check:
            require_known(ctx, iid)
        parsed.append((iid, int(n) if n else 1))
    s = load(ctx, write=True)
    for iid, n in parsed:
        guard(s.add_item, where, iid, n)
    commit(ctx, s)


@app.command(help="Set the count of an existing stack.")
def count(ctx: typer.Context, item: str, value: int, where: Where = "bag", all_: AllStacks = False):
    s = load(ctx, write=True)
    stacks = guard(s.find_items, where, item)
    note_stacks(item, stacks, all_)
    for node in stacks if all_ else stacks[:1]:
        s.set_number(child(node, "count"), value)
    commit(ctx, s)


@app.command(help="Remove stacks.")
def remove(ctx: typer.Context, items: list[str], where: Where = "bag", all_: AllStacks = False):
    s = load(ctx, write=True)
    for iid in items:
        stacks = guard(s.find_items, where, iid)
        note_stacks(iid, stacks, all_)
        for _ in stacks if all_ else stacks[:1]:
            guard(s.remove_item, where, iid)
    commit(ctx, s)


@app.command(help="Replace an item id with another one in place, keeping the count.")
def swap(ctx: typer.Context, old: str, new: str, where: Where = "bag", all_: AllStacks = False, check: Check = True):
    if check:
        require_known(ctx, new)
    s = load(ctx, write=True)
    stacks = guard(s.find_items, where, old)
    note_stacks(old, stacks, all_)
    for _ in stacks if all_ else stacks[:1]:
        s.set_str(child(s.find_item(where, old), "id"), new)
    commit(ctx, s)


@app.command(help="Show player resources (money, energy, tech points, reputation...).")
def res(ctx: typer.Context, pattern: Annotated[str, typer.Argument(help="Regex filter.")] = "."):
    for t, node in load(ctx).res():
        if re.search(pattern, t):
            typer.echo(f"{node.value:>12}  {t}")


@app.command("set-res", help="Set a player resource: gk2 set-res tech_red 999")
def set_res(ctx: typer.Context, res_type: str, value: float):
    s = load(ctx, write=True)
    guard(s.set_number, guard(s.res_node, res_type), value)
    commit(ctx, s)


@app.command(help="Show or set money, in copper (100 copper = 1 silver). Stay below 16777216.")
def money(ctx: typer.Context, value: Annotated[float | None, typer.Argument()] = None):
    s = load(ctx, write=value is not None)
    node = guard(s.res_node, "money")
    if value is None:
        typer.echo(node.value)
        return
    guard(s.set_number, node, value)
    commit(ctx, s)


@app.command(help="Show talent branches: level, exp, free points.")
def talents(ctx: typer.Context):
    for t in load(ctx).talents():
        v = {c.name: c.value for c in t.children if c.kind == "val"}
        typer.echo(
            f"{v.get('id')!s:15} level {v.get('curTalentLevel'):>3}  exp {v.get('curExp'):>3}  "
            f"free {v.get('talentExpPoints'):>4}  value {v.get('curTalentValue')}"
        )


@app.command("set-talents", help="Set free talent points in every branch (or one with --branch).")
def set_talents(
    ctx: typer.Context,
    points: int,
    branch: Annotated[str | None, typer.Option(help="Only this branch, e.g. talent_red.")] = None,
):
    s = load(ctx, write=True)
    for t in s.talents():
        if branch is None or child(t, "id").value == branch:
            s.set_number(child(t, "talentExpPoints"), points)
    commit(ctx, s)


@app.command(help="List the player's zombies.")
def zombies(ctx: typer.Context):
    for z in load(ctx).zombies():
        v = {c.name: c.value for c in z.children if c.kind == "val"}
        parts = [child(i, "id").value for i in array_of(child(child(z, "zombieItem"), "inventory")).children]
        typer.echo(
            f"{v.get('name')}  type {v.get('zombieType')}  zone {v.get('worldZoneDataId')}  "
            f"tech R{v.get('techRed')} B{v.get('techBlue')} G{v.get('techGreen')}"
        )
        typer.echo(f"    {', '.join(parts)}")


@app.command("zombies-max", help="Best body parts, 999 red skulls and tech points for every zombie.")
def zombies_max(
    ctx: typer.Context,
    tech: Annotated[int, typer.Option(help="Tech points of each colour.")] = 999999,
    brains: Annotated[int, typer.Option(help="Brain count, 3 red skulls each.")] = 328,
):
    s = load(ctx, write=True)

    def parts(zi):
        return array_of(child(child(s.zombies()[zi], "zombieItem"), "inventory")).children

    for zi in range(len(s.zombies())):
        for pi in range(len(parts(zi))):
            node = child(parts(zi)[pi], "id")
            kind = node.value.split("_")[0]
            if kind in BEST_BODY and node.value != BEST_BODY[kind]:
                s.set_str(node, BEST_BODY[kind])
        for it in parts(zi):
            if child(it, "id").value.startswith("brain_") and child(it, "count").value != brains:
                s.set_number(child(it, "count"), brains)
        z = s.zombies()[zi]
        for c in ("techRed", "techBlue", "techGreen"):
            if child(z, c).value != tech:
                s.set_number(child(z, c), tech)
    commit(ctx, s)


@app.command("equip-best", help="Top tier on every tool belt slot that already holds an item.")
def equip_best(ctx: typer.Context):
    s = load(ctx, write=True)
    for idx in range(len(s.items("belt"))):
        iid, _, node = s.items("belt")[idx]
        fam = iid.rsplit("_", 1)[0]
        if fam in BEST_BELT and BEST_BELT[fam] != iid:
            s.set_str(child(node, "id"), BEST_BELT[fam])
    commit(ctx, s)


@app.command(help="Search ids in the game data (items, techs, buildings, crafts).")
def find(
    ctx: typer.Context,
    pattern: Annotated[str, typer.Argument(help="Regex.")],
    refresh: Annotated[bool, typer.Option(help="Re-read the game data.")] = False,
    limit: int = 200,
):
    gd = game_dir(ctx)
    rx = re.compile(pattern)
    hits = sorted(k for k in gamedata.ids(gd, refresh) if rx.search(k))
    for k in hits[:limit]:
        typer.echo(k + ("   [item]" if gamedata.is_bag_item(gd, k) else ""))
    if len(hits) > limit:
        typer.echo(f"... {len(hits) - limit} more")


@app.command(help="List backups made by gk2.")
def backups():
    typer.echo(f"# {locate.BACKUP_DIR}")
    if locate.BACKUP_DIR.is_dir():
        for d in sorted(locate.BACKUP_DIR.iterdir()):
            typer.echo(d.name)


@app.command(help="Restore a backup. The current save is backed up first.")
def restore(ctx: typer.Context, name: str):
    src = locate.BACKUP_DIR / name
    if not src.is_dir():
        fail(f"No backup {name} in {locate.BACKUP_DIR}")
    current = load(ctx, write=True)
    target = current.path.parent
    if ctx.obj.dry_run:
        typer.echo(f"Would restore {name} into {target}")
        return
    current.changes.append(f"restore {name}")
    typer.echo(f"Backup of the current save: {current.write(locate.BACKUP_DIR)}")
    for f in src.iterdir():
        shutil.copy2(f, target / f.name)
    typer.secho(f"Restored {name}.", fg="green")


@app.command(help="Show the folders gk2 uses.")
def paths(ctx: typer.Context):
    o: Opts = ctx.obj
    rows: list[tuple[str, Callable[[], Path]]] = [
        ("save", o.save_path),
        ("game", lambda: locate.find_game_dir(o.game_dir)),
    ]
    for label, fn in rows:
        try:
            typer.echo(f"{label:8} {fn()}")
        except locate.NotFoundError as e:
            typer.echo(f"{label:8} not found ({e})")
    typer.echo(f"{'backups':8} {locate.BACKUP_DIR}")
    typer.echo(f"{'cache':8} {locate.CACHE_DIR}")
