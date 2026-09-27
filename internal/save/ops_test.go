package save

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/odin"
	"github.com/sextonworks/gk2-save-editor/internal/odin/odintest"
)

func editor(t *testing.T, spec odintest.SaveSpec) *Editor {
	t.Helper()
	e, err := OpenEditor(writeSave(t, spec))
	require.NoError(t, err)
	return e
}

func stacks(t *testing.T, s *Save, c Container) []Stack {
	t.Helper()
	items, err := s.Items(c)
	require.NoError(t, err)
	return items
}

func ids(items []Stack) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func TestSetResourceOp(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	require.NoError(t, e.Apply(SetResource{Type: "money", Value: 9999999}))
	v, err := e.Save().Resource("money")
	require.NoError(t, err)
	assert.InDelta(t, 9999999, v, 1e-6)
	require.ErrorIs(t, e.Apply(SetResource{Type: "nope", Value: 1}), ErrField)
}

func TestItemOps(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	bag := stacks(t, e.Save(), Bag)

	require.NoError(t, e.Apply(SetItemCount{Container: Bag, UniqueID: bag[0].UniqueID, Count: 7}))
	require.NoError(t, e.Apply(SetItemID{Container: Bag, UniqueID: bag[1].UniqueID, ItemID: "candle_master"}))
	add := NewAddItem(Bag, "свеча", 5)
	require.NoError(t, e.Apply(add))
	require.NoError(t, e.Apply(RemoveItem{Container: Bag, UniqueID: bag[2].UniqueID}))

	got := stacks(t, e.Save(), Bag)
	assert.Equal(t, []string{"faith", "candle_master", "heal_potion", "свеча"}, ids(got))
	assert.Equal(t, int64(7), got[0].Count)
	assert.Equal(t, int64(5), got[3].Count)
	assert.Equal(t, add.UniqueID, got[3].UniqueID)
	assert.Equal(t, int64(2), got[2].Count)

	holder, arr, err := e.Save().containerArray(Bag)
	require.NoError(t, err)
	assert.Equal(t, int64(4), arr.ArrayLen)
	assert.Equal(t, int64(4), integer(holder, "inventoryFillSize"))

	money, err := e.Save().Resource("money")
	require.NoError(t, err)
	assert.InDelta(t, 861, money, 1e-6)
}

func TestItemOpErrors(t *testing.T) {
	spec := odintest.DefaultSpec()
	spec.BagSize = 4
	e := editor(t, spec)
	require.ErrorIs(t, e.Apply(NewAddItem(Bag, "x", 1)), ErrFull)
	require.ErrorIs(t, e.Apply(SetItemCount{Container: Bag, UniqueID: "missing", Count: 1}), ErrNoItem)
	require.ErrorIs(t, e.Apply(RemoveItem{Container: "chest", UniqueID: "x"}), ErrContainer)
	bag := stacks(t, e.Save(), Bag)
	require.ErrorIs(t, e.Apply(SetItemCount{Container: Bag, UniqueID: bag[0].UniqueID, Count: 1 << 40}), odin.ErrOutOfRange)
	assert.False(t, e.Dirty())
	assert.Empty(t, e.Changes())
}

func TestAddItemNeedsTemplate(t *testing.T) {
	spec := odintest.DefaultSpec()
	spec.Bag = nil
	e := editor(t, spec)
	require.ErrorIs(t, e.Apply(NewAddItem(Bag, "x", 1)), ErrTemplate)
}

func TestTalentAndZombieOps(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	require.NoError(t, e.Apply(SetTalentPoints{Talent: "talent_red", Points: 99}))
	require.ErrorIs(t, e.Apply(SetTalentPoints{Talent: "talent_x", Points: 1}), ErrField)
	require.NoError(t, e.Apply(MaxZombie{Name: "zombie_name_1", Tech: 999999, Brains: 328}))
	require.ErrorIs(t, e.Apply(MaxZombie{Name: "nobody"}), ErrNoZombie)

	talents, err := e.Save().Talents()
	require.NoError(t, err)
	assert.Equal(t, int64(99), talents[1].FreePoints)
	assert.Equal(t, int64(2), talents[0].FreePoints)

	zombies, err := e.Save().Zombies()
	require.NoError(t, err)
	z := zombies[0]
	assert.Equal(t, []int64{999999, 999999, 999999}, []int64{z.TechRed, z.TechBlue, z.TechGreen})
	assert.Equal(t, []string{"skin_3_0:3", "brain_3_1:3", "guts_3_0:3"}, ids(z.BodyParts))
	assert.Equal(t, []int64{1, 328, 1}, []int64{z.BodyParts[0].Count, z.BodyParts[1].Count, z.BodyParts[2].Count})
}

func TestEquipBest(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	require.NoError(t, e.Apply(EquipBest{}))
	assert.Equal(t, []string{"hand_tool", "axe_3", "sword_4"}, ids(stacks(t, e.Save(), Belt)))
	assert.Equal(t, "belt: top tier on every slot", EquipBest{}.String())
}

func TestOpStrings(t *testing.T) {
	ops := []Op{
		SetResource{Type: "money", Value: 5},
		SetItemCount{Container: Bag, UniqueID: "u", Count: 2},
		SetItemID{Container: Belt, UniqueID: "u", ItemID: "axe_3"},
		AddItem{Container: Bag, ItemID: "salt", Count: 1},
		RemoveItem{Container: Bag, UniqueID: "u"},
		SetTalentPoints{Talent: "talent_red", Points: 9},
		MaxZombie{Name: "z", Tech: 1, Brains: 2},
	}
	for _, op := range ops {
		assert.NotEmpty(t, op.String())
	}
}
