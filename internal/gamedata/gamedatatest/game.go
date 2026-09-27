package gamedatatest

import (
	"encoding/binary"
	"os"
	"path/filepath"

	"github.com/sextonworks/gk2-save-editor/internal/typetree"
	"github.com/sextonworks/gk2-save-editor/internal/unityasset/unitytest"
)

const classMonoBehaviour = 114

type payload struct{ b []byte }

func (p *payload) align() {
	for len(p.b)%4 != 0 {
		p.b = append(p.b, 0)
	}
}

func (p *payload) i32(v int) {
	p.align()
	p.b = binary.LittleEndian.AppendUint32(p.b, uint32(v))
}

func (p *payload) str(s string) {
	p.i32(len(s))
	p.b = append(p.b, s...)
	p.align()
}

func (p *payload) strs(list ...string) {
	p.i32(len(list))
	for _, s := range list {
		p.str(s)
	}
}

func Balance(items, others []string) []byte {
	p := &payload{}
	p.str("sword_0")
	p.strs("u_bag", "weapon", "melee")
	p.str("faith")
	p.strs("u_bag")
	for _, id := range items {
		p.str(id)
		p.strs("u_bag")
	}
	for _, id := range others {
		p.str(id)
		p.i32(0)
	}
	return p.b
}

func Language(code string, names map[string]string) []byte {
	p := &payload{}
	p.strs()
	p.strs()
	p.str(code)
	keys := make([]string, 0, len(names))
	values := make([]string, 0, len(names))
	for k, v := range names {
		keys = append(keys, k)
		values = append(values, v)
	}
	p.strs(keys...)
	p.strs(values...)
	return p.b
}

const (
	classTexture2D   = 28
	classSprite      = 213
	classSpriteAtlas = 687078895
	chestInteraction = 6
)

var Storages = map[string]int{"chest_rough": 20, "chest_kitchen": 2}

func must(root *typetree.Node, err error) *typetree.Node {
	if err != nil {
		panic(err)
	}
	return root
}

func encode(schema string, v map[string]any) []byte {
	raw, err := typetree.Write(must(typetree.Load(schema)), v)
	if err != nil {
		panic(err)
	}
	return raw
}

func SchemaBalance(items, others []string) []byte {
	defs := []any{
		map[string]any{"id": "sword_0", "itemGroupIds": []string{"u_bag", "weapon"}, "stackCount": 1, "iconId": "i_sword_0"},
		map[string]any{"id": "faith", "itemGroupIds": []string{"u_bag"}, "stackCount": 999, "iconId": "i_faith"},
	}
	for _, id := range items {
		defs = append(defs, map[string]any{"id": id, "itemGroupIds": []string{"u_bag"}, "stackCount": 30, "iconId": "i_" + id})
	}
	wgo := []any{}
	for _, id := range others {
		wgo = append(wgo, map[string]any{"id": id})
	}
	for id, size := range Storages {
		wgo = append(wgo, map[string]any{"id": id, "interactionType": chestInteraction, "inventorySize": size})
	}
	return encode("GameBalance", map[string]any{"m_Name": "GameBalance", "itemDefs": defs, "wgoDefs": wgo})
}

func Write(dir string, items, others []string, names map[string]map[string]string) error {
	objects := []unitytest.Object{{PathID: 1, Class: classMonoBehaviour, Data: SchemaBalance(items, others)}}
	id := int64(2)
	for lang, table := range names {
		objects = append(objects, unitytest.Object{PathID: id, Class: classMonoBehaviour, Data: unitytest.Mono("lng_"+lang, Language(lang, table))})
		id++
	}
	data := filepath.Join(dir, "GraveyardKeeper2_Data")
	if err := os.MkdirAll(data, 0o700); err != nil {
		return err
	}
	raw := unitytest.Builder{Version: 22, Objects: objects}.Build()
	if err := os.WriteFile(filepath.Join(data, "resources.assets"), raw, 0o600); err != nil {
		return err
	}
	return writeIcons(data)
}

func rect(x, y, w, h float64) map[string]any {
	return map[string]any{"x": x, "y": y, "width": w, "height": h}
}

func key(n int) map[string]any {
	return map[string]any{"first": map[string]any{"data[0]": n, "data[1]": 7, "data[2]": 7, "data[3]": 7}, "second": 21300000}
}

var IconPixels = [4][4]byte{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 0}}

type sprite struct {
	name string
	row  int
}

func atlasObjects(atlas string, sprites []sprite) []unitytest.Object {
	pix := make([]byte, 0, 4*4*4)
	for row := range 4 {
		for range 4 {
			pix = append(pix, IconPixels[row][:]...)
		}
	}
	tex := encode("Texture2D", map[string]any{"m_Name": atlas, "m_Width": 4, "m_Height": 4, "m_TextureFormat": 4, "image data": pix})
	objects := []unitytest.Object{{PathID: 10, Class: classTexture2D, Data: tex}}
	render := []any{}
	names := []string{}
	for i, sp := range sprites {
		objects = append(objects, unitytest.Object{PathID: int64(20 + i), Class: classSprite, Data: encode("Sprite", map[string]any{
			"m_Name": sp.name, "m_Rect": rect(0, 0, 6, 3), "m_RenderDataKey": key(i),
		})})
		render = append(render, map[string]any{"first": key(i), "second": map[string]any{
			"texture": map[string]any{"m_PathID": 10}, "textureRect": rect(0, float64(sp.row), 4, 1),
			"textureRectOffset": map[string]any{"x": 1, "y": 1}, "settingsRaw": 67,
		}})
		names = append(names, sp.name)
	}
	return append(objects, unitytest.Object{PathID: 30, Class: classSpriteAtlas, Data: encode("SpriteAtlas", map[string]any{
		"m_Name": atlas, "m_PackedSpriteNamesToIndex": names, "m_RenderDataMap": render,
	})})
}

func writeBundle(dir, name, atlas string, sprites []sprite) error {
	assets := unitytest.Builder{Version: 22, Objects: atlasObjects(atlas, sprites)}.Build()
	raw := unitytest.BundleSpec{Files: []unitytest.BundleFile{{Name: "CAB-" + name, Data: assets}}}.Build()
	return os.WriteFile(filepath.Join(dir, name+".bundle"), raw, 0o600)
}

func writeIcons(data string) error {
	raw := unitytest.Builder{Version: 22, Objects: atlasObjects("Icons", []sprite{{"i_candle_basic", 0}, {"i_faith", 2}})}.Build()
	if err := os.WriteFile(filepath.Join(data, "sharedassets0.assets"), raw, 0o600); err != nil {
		return err
	}
	bundles := filepath.Join(data, "StreamingAssets", "aa", "StandaloneWindows64")
	if err := os.MkdirAll(bundles, 0o700); err != nil {
		return err
	}
	if err := writeBundle(bundles, "aaa", "IconsCompressed", []sprite{{"i_bundle_icon", 1}}); err != nil {
		return err
	}
	return writeBundle(bundles, "bbb", "Characters", []sprite{{"hero_walk", 1}})
}
