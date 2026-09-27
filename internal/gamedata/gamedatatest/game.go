package gamedatatest

import (
	"encoding/binary"
	"os"
	"path/filepath"

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

func Write(dir string, items, others []string, names map[string]map[string]string) error {
	objects := []unitytest.Object{{PathID: 1, Class: classMonoBehaviour, Data: unitytest.Mono("GameBalance", Balance(items, others))}}
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
	return os.WriteFile(filepath.Join(data, "resources.assets"), raw, 0o600)
}
