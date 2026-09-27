package gamedata

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/unityasset"
	"github.com/sextonworks/gk2-save-editor/internal/unityasset/unitytest"
)

type payload struct{ b []byte }

func (p *payload) align() {
	for len(p.b)%4 != 0 {
		p.b = append(p.b, 0)
	}
}

func (p *payload) i32(v int) *payload {
	p.align()
	p.b = binary.LittleEndian.AppendUint32(p.b, uint32(v))
	return p
}

func (p *payload) str(s string) *payload {
	p.i32(len(s))
	p.b = append(p.b, s...)
	p.align()
	return p
}

func (p *payload) strs(list ...string) *payload {
	p.i32(len(list))
	for _, s := range list {
		p.str(s)
	}
	return p
}

func balance() []byte {
	p := &payload{}
	p.str("faith").strs("u_bag")
	p.str("heal_potion").strs("battle_potion", "pot_bag", "u_bag").str("i_heal_potion_1")
	p.str("sword_0").strs("u_bag", "weapon", "melee")
	p.str("tech_zombie_digging").i32(7)
	p.str("fertilizer").strs("u_bag")
	return p.b
}

func language(code string, keys, values []string) []byte {
	p := &payload{}
	p.strs("candle_place").strs("candle")
	p.str(code)
	p.strs(keys...)
	p.strs(values...)
	return p.b
}

func gameDir(t *testing.T, objects ...unitytest.Object) string {
	t.Helper()
	dir := t.TempDir()
	data := filepath.Join(dir, "GraveyardKeeper2_Data")
	require.NoError(t, os.MkdirAll(data, 0o700))
	raw := unitytest.Builder{Version: unityasset.SupportedVersion, Objects: objects}.Build()
	require.NoError(t, os.WriteFile(filepath.Join(data, "resources.assets"), raw, 0o600))
	return dir
}

func fullGame(t *testing.T) string {
	t.Helper()
	return gameDir(t,
		unitytest.Object{PathID: 1, Class: unityasset.ClassMonoBehavour, Data: unitytest.Mono("GameBalance", balance())},
		unitytest.Object{PathID: 2, Class: unityasset.ClassMonoBehavour, Data: unitytest.Mono("lng_en",
			language("en", []string{"faith", "candle", "heal_potion", "fertilizer:add_cells_2", "lumberjack_1", "lumberjack_1_d"}, []string{"Faith", "Candle", "Healing Potion", "Fertilizer", "Lumberjack I", "Chop trees."}))},
		unitytest.Object{PathID: 3, Class: unityasset.ClassMonoBehavour, Data: unitytest.Mono("lng_ru",
			language("ru", []string{"faith", "candle"}, []string{"<nobr>Вера</nobr>", "Свеча\u200b"}))},
	)
}

func TestExtractCatalog(t *testing.T) {
	c, err := Load(fullGame(t), t.TempDir(), false)
	require.NoError(t, err)
	assert.Equal(t, []string{"faith", "fertilizer", "heal_potion", "sword_0"}, c.Items)
	assert.True(t, c.Known("tech_zombie_digging"))
	assert.False(t, c.Known("nope"))

	tests := []struct {
		id, lang, want string
		item           bool
	}{
		{"faith", "ru", "Вера", true},
		{"faith", "zh_cn", "Faith", true},
		{"heal_potion", "ru", "Healing Potion", true},
		{"candle_place", "ru", "Свеча", false},
		{"faith:2", "en", "Faith", true},
		{"fertilizer:add_cells_2", "en", "Fertilizer", true},
		{"tech_zombie_digging", "en", "tech_zombie_digging", false},
	}
	for _, tt := range tests {
		t.Run(tt.id+"/"+tt.lang, func(t *testing.T) {
			assert.Equal(t, tt.want, c.Name(tt.id, tt.lang))
			assert.Equal(t, tt.item, c.IsItem(tt.id))
		})
	}
}

func TestBaseNameAndDescription(t *testing.T) {
	c, err := Load(fullGame(t), t.TempDir(), false)
	require.NoError(t, err)
	assert.Equal(t, "Lumberjack", c.BaseName("lumberjack", "en"))
	c.Names["zh_cn"] = map[string]string{"lumberjack_1": "伐木工I"}
	assert.Equal(t, "伐木工", c.BaseName("lumberjack", "zh_cn"))
	assert.Equal(t, "Faith", c.BaseName("faith", "en"))
	assert.Equal(t, "nothing", c.BaseName("nothing", "en"))
	assert.Equal(t, "Chop trees.", c.Description("lumberjack", "en"))
	assert.Empty(t, c.Description("faith", "en"))
}

func TestSearch(t *testing.T) {
	c, err := Load(fullGame(t), t.TempDir(), false)
	require.NoError(t, err)
	got := c.Search("вера", "ru", true, 10)
	require.Len(t, got, 1)
	assert.Equal(t, Entry{ID: "faith", Name: "Вера", IsItem: true}, got[0])
	assert.Len(t, c.Search("", "en", true, 2), 2)
	assert.Len(t, c.Search("zombie", "en", false, 0), 1)
}

func TestCache(t *testing.T) {
	game, cache := fullGame(t), t.TempDir()
	first, err := Load(game, cache, false)
	require.NoError(t, err)
	cached, err := Load(game, cache, false)
	require.NoError(t, err)
	assert.Equal(t, first.Source, cached.Source)
	assert.Equal(t, "Вера", cached.Name("faith", "ru"))
	assert.True(t, cached.IsItem("sword_0"))

	other := gameDir(t, unitytest.Object{PathID: 1, Class: unityasset.ClassMonoBehavour, Data: unitytest.Mono("GameBalance", balance())})
	require.NoError(t, os.Rename(ResourcesPath(other), ResourcesPath(game)))
	updated, err := Load(game, cache, false)
	require.NoError(t, err)
	assert.NotEqual(t, first.Source, updated.Source)
	assert.Equal(t, "faith", updated.Name("faith", "ru"))

	require.NoError(t, os.WriteFile(filepath.Join(cache, "catalog-v3.json"), []byte("{bad"), 0o600))
	_, err = Load(game, cache, false)
	require.NoError(t, err)
}

func TestExtractErrors(t *testing.T) {
	_, err := Load(t.TempDir(), t.TempDir(), false)
	require.Error(t, err)

	noBalance := gameDir(t, unitytest.Object{PathID: 1, Class: unityasset.ClassMonoBehavour, Data: unitytest.Mono("GameBalance", []byte("nothing"))})
	_, err = Load(noBalance, t.TempDir(), false)
	require.ErrorIs(t, err, ErrNoBalance)

	bad := &payload{}
	bad.strs("a", "b").strs("c")
	broken := gameDir(t,
		unitytest.Object{PathID: 1, Class: unityasset.ClassMonoBehavour, Data: unitytest.Mono("GameBalance", balance())},
		unitytest.Object{PathID: 2, Class: unityasset.ClassMonoBehavour, Data: unitytest.Mono("lng_en", bad.b)},
	)
	_, err = Load(broken, t.TempDir(), false)
	require.ErrorIs(t, err, ErrBadStrings)
}

func TestClean(t *testing.T) {
	assert.Equal(t, "简易蜡烛", Clean("<nobr>简易</nobr>\u200b<nobr>蜡烛</nobr>"))
	assert.Equal(t, "a b", Clean(" a\u00a0b "))
}

func TestRealGame(t *testing.T) {
	game := os.Getenv("GK2_REAL_GAME")
	if game == "" {
		t.Skip("set GK2_REAL_GAME to the game folder")
	}
	c, err := Load(game, t.TempDir(), true)
	require.NoError(t, err)
	assert.Greater(t, len(c.Items), 500)
	assert.Equal(t, "Простая свеча", c.Name("candle_basic", "ru"))
	assert.Equal(t, "Simple Candle", c.Name("candle_basic", "en"))
	assert.Equal(t, "简易蜡烛", c.Name("candle_basic", "zh_cn"))
	for _, id := range []string{"faith", "heal_potion", "sword_4", "alchemy_kit_3", "pro_rod", "skin_3_0:3"} {
		assert.True(t, c.IsItem(id), id)
	}
}
