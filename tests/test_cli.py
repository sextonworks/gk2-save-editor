import pytest
from typer.testing import CliRunner

from gk2 import cli, locate
from gk2.save import Save, child

from .builder import write_save

runner = CliRunner()


@pytest.fixture
def env(tmp_path, monkeypatch):
    saves = tmp_path / "saves"
    path = write_save(saves)
    monkeypatch.setattr(locate, "BACKUP_DIR", tmp_path / "backups")
    monkeypatch.setattr(locate, "game_running", lambda: False)
    return saves, path


def run(saves, *args):
    return runner.invoke(cli.app, ["--save-dir", str(saves), *args])


def test_info_and_bag(env):
    saves, _ = env
    r = run(saves, "info")
    assert r.exit_code == 0, r.output
    assert "Day:      7" in r.output
    r = run(saves, "bag")
    assert "99  faith" in r.output


def test_add_writes_with_backup(env, tmp_path):
    saves, path = env
    r = run(saves, "add", "--no-check", "candle_basic=5", "heal_potion")
    assert r.exit_code == 0, r.output
    assert "Written" in r.output
    items = [(i, c) for i, c, _ in Save(path).items("bag")]
    assert items[-2:] == [("candle_basic", 5), ("heal_potion", 1)]
    assert len(list((tmp_path / "backups").iterdir())) == 1


def test_dry_run_does_not_write(env):
    saves, path = env
    before = path.read_bytes()
    r = run(saves, "-n", "money", "5000")
    assert r.exit_code == 0, r.output
    assert "Dry run" in r.output
    assert path.read_bytes() == before


def test_refuses_while_game_runs(env, monkeypatch):
    saves, path = env
    monkeypatch.setattr(locate, "game_running", lambda: True)
    before = path.read_bytes()
    r = run(saves, "money", "5000")
    assert r.exit_code == 1
    assert "running" in r.output
    assert path.read_bytes() == before


def test_count_warns_about_several_stacks(env):
    saves, path = env
    r = run(saves, "count", "heal_potion", "9")
    assert "2 stacks" in r.output
    assert [c for i, c, _ in Save(path).items("bag") if i == "heal_potion"] == [9, 2]
    run(saves, "count", "heal_potion", "4", "--all")
    assert [c for i, c, _ in Save(path).items("bag") if i == "heal_potion"] == [4, 4]


def test_set_talents_and_zombies_max(env):
    saves, path = env
    assert run(saves, "set-talents", "99").exit_code == 0
    assert run(saves, "zombies-max").exit_code == 0
    s = Save(path)
    assert [child(t, "talentExpPoints").value for t in s.talents()] == [99, 99]
    z = s.zombies()[0]
    assert child(z, "techRed").value == 999999
    parts = {
        child(i, "id").value: child(i, "count").value
        for i in child(child(z, "zombieItem"), "inventory").children[0].children
    }
    assert parts == {"skin_3_0:3": 1, "brain_3_1:3": 328, "guts_3_0:3": 1}


def test_equip_best_upgrades_belt(env):
    saves, path = env
    assert run(saves, "equip-best").exit_code == 0
    assert [i for i, _, _ in Save(path).items("belt")] == ["hand_tool", "axe_3", "sword_4"]


def test_missing_item_is_an_error(env):
    saves, _ = env
    r = run(saves, "remove", "nothing")
    assert r.exit_code == 1
    assert "not in the bag" in r.output
