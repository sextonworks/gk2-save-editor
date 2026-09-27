package gamedata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/gamedata/gamedatatest"
)

func TestReadBalance(t *testing.T) {
	b, err := readBalance(gamedatatest.SchemaBalance([]string{"candle_basic", "candle_basic"}, []string{"grave_ground"}))
	require.NoError(t, err)
	assert.Equal(t, []string{"candle_basic", "faith", "sword_0"}, b.items)
	assert.Equal(t, ItemDef{Icon: "i_candle_basic", Stack: 30}, b.defs["candle_basic"])
	assert.Equal(t, map[string]int{"chest_rough": 20, "chest_kitchen": 2}, b.storages)

	_, err = readBalance([]byte("not a balance"))
	require.Error(t, err)
}

func TestCatalogDefs(t *testing.T) {
	game := t.TempDir()
	require.NoError(t, gamedatatest.Write(game, []string{"cabbage", "1h_marble"}, nil, nil))
	c, err := Load(game, t.TempDir(), false)
	require.NoError(t, err)
	assert.Equal(t, "i_cabbage", c.Icon("cabbage"))
	assert.Equal(t, "i_cabbage", c.Icon("cabbage:2"))
	assert.Empty(t, c.Icon("unknown"))
	assert.True(t, c.Known("1h_marble"))
	assert.True(t, c.IsItem("1h_marble"))
	size, ok := c.StorageCapacity("chest_rough")
	assert.True(t, ok)
	assert.Equal(t, 20, size)
	_, ok = c.StorageCapacity("grave_ground")
	assert.False(t, ok)
}
