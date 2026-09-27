package save

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/odin/odintest"
)

func TestStorages(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	st, err := e.Save().Storages()
	require.NoError(t, err)
	require.Len(t, st, 2)
	assert.Equal(t, Storage{
		UniqueID: "chest-yard", ID: "chest_rough", Zone: "yard", Scene: "MainScene", Capacity: 20,
		Items: stacks(t, e.Save(), WorldContainer("chest-yard")),
	}, st[0])
	assert.Equal(t, "wood", st[0].Items[0].ID)
	assert.Empty(t, st[1].Items)
	assert.Equal(t, int64(2), st[1].Capacity)
}

func TestWorldContainerOps(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	yard, home := WorldContainer("chest-yard"), WorldContainer("chest-home")
	wood := stacks(t, e.Save(), yard)[0]

	require.NoError(t, e.Apply(SetItemCount{Container: yard, UniqueID: wood.UniqueID, Count: 99}))
	require.NoError(t, e.Apply(NewAddItem(home, "candle_basic", 3)))
	require.NoError(t, e.Apply(NewAddItem(home, "wax", 1)))
	require.ErrorIs(t, e.Apply(NewAddItem(home, "honey", 1)), ErrFull)
	require.NoError(t, e.Apply(RemoveItem{Container: yard, UniqueID: wood.UniqueID}))

	yardItems := stacks(t, e.Save(), yard)
	require.Len(t, yardItems, 1)
	assert.Equal(t, "stone", yardItems[0].ID)
	homeItems := stacks(t, e.Save(), home)
	require.Len(t, homeItems, 2)
	assert.Equal(t, []string{"candle_basic", "wax"}, []string{homeItems[0].ID, homeItems[1].ID})

	holder, _, err := e.Save().containerArray(home)
	require.NoError(t, err)
	assert.Equal(t, int64(2), integer(holder, "inventoryFillSize"))
	require.ErrorIs(t, e.Apply(RemoveItem{Container: WorldContainer("missing"), UniqueID: "x"}), ErrContainer)
}
