import pytest

from gk2 import locate


def test_env_override_for_save_dir(tmp_path, monkeypatch):
    monkeypatch.setenv("GK2_SAVE_DIR", str(tmp_path))
    assert locate.find_save_dir() == tmp_path


def test_explicit_missing_save_dir(tmp_path):
    with pytest.raises(locate.NotFoundError):
        locate.find_save_dir(tmp_path / "missing")


def test_game_dir_must_look_like_the_game(tmp_path):
    with pytest.raises(locate.NotFoundError):
        locate.find_game_dir(tmp_path)
    (tmp_path / "GraveyardKeeper2_Data").mkdir()
    assert locate.find_game_dir(tmp_path) == tmp_path


def test_proton_and_crossover_layouts(tmp_path, monkeypatch):
    monkeypatch.delenv("GK2_SAVE_DIR", raising=False)
    monkeypatch.setattr(locate, "_home", lambda: tmp_path)
    monkeypatch.setattr(locate.sys, "platform", "linux")
    proton = (
        tmp_path
        / ".steam/steam/steamapps/compatdata"
        / locate.APP_ID
        / "pfx/drive_c/users/steamuser"
        / locate.SAVE_SUBPATH
    )
    proton.mkdir(parents=True)
    assert locate.find_save_dir() == proton

    proton.rename(tmp_path / "moved")
    bottle = (
        tmp_path / "Library/Application Support/CrossOver/Bottles/Steam/drive_c/users/crossover" / locate.SAVE_SUBPATH
    )
    bottle.mkdir(parents=True)
    assert locate.find_save_dir() == bottle


def test_unity_version_fallback(tmp_path):
    assert locate.unity_version(tmp_path) == locate.FALLBACK_UNITY_VERSION
