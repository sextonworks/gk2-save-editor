import pytest

from gk2 import odin
from gk2.save import Save, SaveError, child

from .builder import write_save


@pytest.fixture
def save(tmp_path):
    return Save(write_save(tmp_path / "saves"))


def bag_ids(s):
    return [(i, c) for i, c, _ in s.items("bag")]


def refids(s):
    return [n.refid for n in odin.walk(s.root) if n.refid is not None]


def test_reads_containers_and_values(save):
    assert bag_ids(save) == [("faith", 99), ("salt", 1), ("heal_potion", 3), ("heal_potion", 2)]
    assert [i for i, _, _ in save.items("belt")] == ["hand_tool", "axe_1", "sword_0"]
    assert save.res_node("money").value == pytest.approx(861.0)
    assert [child(t, "talentExpPoints").value for t in save.talents()] == [2, 3]
    assert len(save.zombies()) == 1
    assert save.info()["day"] == 7


def test_add_item_appends_stack_with_unique_refs(save):
    before = len(refids(save))
    save.add_item("bag", "candle_master", 5)
    assert bag_ids(save)[-1] == ("candle_master", 5)
    ids = refids(save)
    assert len(ids) == len(set(ids))
    assert len(ids) > before
    holder, arr = save.container("bag")
    assert arr.value == 5
    assert child(holder, "inventoryFillSize").value == 5


def test_add_item_refuses_when_full(tmp_path):
    s = Save(write_save(tmp_path, bag=[("salt", 1)], bag_size=1))
    with pytest.raises(SaveError, match="full"):
        s.add_item("bag", "faith", 1)


def test_remove_item_keeps_stream_valid(save):
    save.remove_item("bag", "salt")
    assert [i for i, _ in bag_ids(save)] == ["faith", "heal_potion", "heal_potion"]
    assert save.container("bag")[1].value == 3
    assert save.res_node("money").value == pytest.approx(861.0)


def test_set_str_with_other_length_shifts_following_data(save):
    save.set_str(child(save.find_item("belt", "axe_1"), "id"), "small_instruments_3")
    assert [i for i, _, _ in save.items("belt")] == ["hand_tool", "small_instruments_3", "sword_0"]
    assert save.res_node("energy").value == pytest.approx(95.5)


def test_set_number_uses_field_type(save):
    save.set_number(save.res_node("money"), 5000)
    save.set_number(child(save.find_item("bag", "faith"), "count"), 7.9)
    assert save.res_node("money").value == pytest.approx(5000.0)
    assert child(save.find_item("bag", "faith"), "count").value == 7
    with pytest.raises(SaveError, match="not a number"):
        save.set_number(child(save.find_item("bag", "faith"), "id"), 1)


def test_find_items_lists_every_stack(save):
    assert len(save.find_items("bag", "heal_potion")) == 2
    with pytest.raises(SaveError, match="not in the bag"):
        save.find_items("bag", "nothing")


def test_write_makes_backup_and_round_trips(save, tmp_path):
    original = save.path.read_bytes()
    save.add_item("bag", "candle_basic", 1)
    bk = save.write(tmp_path / "bk")
    assert bk is not None
    assert (bk / "Steam_1.dat").read_bytes() == original
    assert (bk / "Steam_1.info").exists()
    again = Save(save.path)
    assert bag_ids(again)[-1] == ("candle_basic", 1)


def test_write_without_changes_does_nothing(save, tmp_path):
    assert save.write(tmp_path / "bk") is None
    assert not (tmp_path / "bk").exists()
