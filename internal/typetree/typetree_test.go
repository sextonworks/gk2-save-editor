package typetree

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func load(t *testing.T, name string) *Node {
	t.Helper()
	n, err := Load(name)
	require.NoError(t, err)
	return n
}

func TestRoundTripAllSchemas(t *testing.T) {
	for _, name := range []string{"GameBalance", "Texture2D", "Sprite", "SpriteAtlas"} {
		t.Run(name, func(t *testing.T) {
			root := load(t, name)
			raw, err := Write(root, map[string]any{"m_Name": "x"})
			require.NoError(t, err)
			v, err := Read(root, raw)
			require.NoError(t, err)
			assert.Equal(t, "x", Str(v, "m_Name"))
		})
	}
}

func TestReadGameBalanceValues(t *testing.T) {
	root := load(t, "GameBalance")
	raw, err := Write(root, map[string]any{
		"itemDefs": []any{
			map[string]any{"id": "candle_basic", "itemGroupIds": []string{"u_bag"}, "stackCount": 30, "iconId": "i_candle_1", "durDecreaseOnUse": 0.5, "isTool": true},
		},
		"wgoDefs": []any{map[string]any{"id": "chest_rough", "interactionType": 6, "inventorySize": 20}},
	})
	require.NoError(t, err)
	v, err := Read(root, raw)
	require.NoError(t, err)
	items := List(v, "itemDefs")
	require.Len(t, items, 1)
	assert.Equal(t, "candle_basic", Str(items[0], "id"))
	assert.Equal(t, []string{"u_bag"}, Strings(items[0], "itemGroupIds"))
	assert.Equal(t, int64(30), Int(items[0], "stackCount"))
	assert.InDelta(t, 0.5, Float(items[0], "durDecreaseOnUse"), 1e-9)
	assert.Equal(t, int64(1), Int(items[0], "isTool"))
	wgo := List(v, "wgoDefs")
	require.Len(t, wgo, 1)
	assert.Equal(t, int64(6), Int(wgo[0], "interactionType"))

	prefix, err := ReadPrefix(root, raw, "itemDefs")
	require.NoError(t, err)
	assert.Len(t, List(prefix, "itemDefs"), 1)
}

func TestTextureBytes(t *testing.T) {
	root := load(t, "Texture2D")
	pix := []byte{1, 2, 3, 4}
	raw, err := Write(root, map[string]any{
		"m_Name": "t", "m_Width": 1, "m_Height": 1, "m_TextureFormat": 4, "image data": pix,
		"m_StreamData": map[string]any{"offset": 16, "size": 4, "path": "a.resS"},
	})
	require.NoError(t, err)
	v, err := Read(root, raw)
	require.NoError(t, err)
	assert.Equal(t, pix, Bytes(v, "image data"))
	sd := Map(v, "m_StreamData")
	assert.Equal(t, "a.resS", Str(sd, "path"))
	assert.Equal(t, int64(16), Int(sd, "offset"))
}

func TestReadErrors(t *testing.T) {
	root := load(t, "SpriteAtlas")
	raw, err := Write(root, map[string]any{"m_Name": "Icons"})
	require.NoError(t, err)
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"truncated", raw[:len(raw)-6], ErrTruncated},
		{"trailing bytes", append(append([]byte{}, raw...), 0, 0, 0, 0), ErrLayout},
		{"negative length", []byte{0xff, 0xff, 0xff, 0xff}, ErrLayout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Read(root, tt.data)
			require.ErrorIs(t, err, tt.want)
		})
	}
	_, err = ReadPrefix(root, raw, "missing")
	require.ErrorIs(t, err, ErrSchema)
}

func TestSchemaErrors(t *testing.T) {
	_, err := Load("Missing")
	require.Error(t, err)
	_, err = build([][4]any{{0.0, "int", "a", 0.0}, {0.0, "int", "b", 0.0}})
	require.ErrorIs(t, err, ErrSchema)
	_, err = build([][4]any{{"x", "int", "a", 0.0}})
	require.ErrorIs(t, err, ErrSchema)
	_, err = build(nil)
	require.ErrorIs(t, err, ErrSchema)
	_, err = Read(&Node{Type: "Mystery", Name: "m"}, nil)
	require.ErrorIs(t, err, ErrSchema)
}

func TestGetters(t *testing.T) {
	m := map[string]any{"b": true, "f": 1.5, "i": int64(3), "s": "x", "l": []any{"a", 1}}
	assert.Equal(t, int64(1), Int(m, "b"))
	assert.Equal(t, int64(1), Int(m, "f"))
	assert.InDelta(t, 3.0, Float(m, "i"), 1e-9)
	assert.Equal(t, []string{"a"}, Strings(m, "l"))
	assert.Empty(t, List(m, "l"))
	assert.Empty(t, Str(m, "missing"))
	assert.Nil(t, Map(m, "s"))
	assert.Nil(t, Bytes(m, "s"))
}
