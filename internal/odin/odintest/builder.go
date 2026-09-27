package odintest

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

const (
	ItemType  = "Item, Assembly-CSharp"
	ItemList  = "System.Collections.Generic.List`1[[Item, Assembly-CSharp]], mscorlib"
	PropsType = "System.Collections.Generic.Dictionary`2[[System.Type, mscorlib],[SerializedItemProperty, Assembly-CSharp]]"
)

type Writer struct {
	buf   []byte
	types map[string]int32
	refs  int32
}

func NewWriter() *Writer {
	return &Writer{types: map[string]int32{}}
}

func (w *Writer) Bytes() []byte {
	return w.buf
}

func (w *Writer) i32(v int32) {
	w.buf = binary.LittleEndian.AppendUint32(w.buf, uint32(v))
}

func (w *Writer) String(v string) {
	u := utf16.Encode([]rune(v))
	w.buf = append(w.buf, 1)
	w.i32(int32(len(u)))
	for _, c := range u {
		w.buf = binary.LittleEndian.AppendUint16(w.buf, c)
	}
}

func (w *Writer) typ(t string) {
	if id, ok := w.types[t]; ok {
		w.buf = append(w.buf, 0x30)
		w.i32(id)
		return
	}
	id := int32(len(w.types))
	w.types[t] = id
	w.buf = append(w.buf, 0x2f)
	w.i32(id)
	w.String(t)
}

func (w *Writer) Ref(name, t string) *Writer {
	if name == "" {
		w.buf = append(w.buf, 0x02)
	} else {
		w.buf = append(w.buf, 0x01)
		w.String(name)
	}
	w.typ(t)
	w.refs++
	w.i32(w.refs)
	return w
}

func (w *Writer) End() *Writer {
	w.buf = append(w.buf, 0x05)
	return w
}

func (w *Writer) Array(n int) *Writer {
	w.buf = append(w.buf, 0x06)
	w.buf = binary.LittleEndian.AppendUint64(w.buf, uint64(n))
	return w
}

func (w *Writer) EndArray() *Writer {
	w.buf = append(w.buf, 0x07)
	return w
}

func (w *Writer) Int(name string, v int32) *Writer {
	w.buf = append(w.buf, 0x17)
	w.String(name)
	w.i32(v)
	return w
}

func (w *Writer) Float(name string, v float32) *Writer {
	w.buf = append(w.buf, 0x1f)
	w.String(name)
	w.buf = binary.LittleEndian.AppendUint32(w.buf, math.Float32bits(v))
	return w
}

func (w *Writer) Str(name, v string) *Writer {
	if name == "" {
		w.buf = append(w.buf, 0x28)
	} else {
		w.buf = append(w.buf, 0x27)
		w.String(name)
	}
	w.String(v)
	return w
}

type Stack struct {
	ID    string
	Count int32
}

func (w *Writer) Item(id string, count int32) *Writer {
	w.Ref("", ItemType).Str("id", id).Int("count", count)
	w.Ref("uniqueId", "SGuid, Assembly-CSharp").Str("id", fmt.Sprintf("00000000-0000-0000-0000-%012d", w.refs)).End()
	w.Ref("inventory", ItemList).Array(0).EndArray().End()
	w.Int("inventorySize", 0).Int("inventoryFillSize", -1)
	w.Ref("properties", PropsType).Array(0).EndArray().End()
	return w.End()
}

func (w *Writer) Container(field string, items []Stack, size int32) *Writer {
	w.Ref(field, "Inventory, Assembly-CSharp")
	w.Ref("inventoryItem", ItemType).Str("id", field).Int("count", 1)
	w.Ref("inventory", ItemList).Array(len(items))
	for _, it := range items {
		w.Item(it.ID, it.Count)
	}
	w.EndArray().End()
	w.Int("inventorySize", size).Int("inventoryFillSize", int32(len(items)))
	return w.End().End()
}

type Chest struct {
	ID       string
	UniqueID string
	Zone     string
	Size     int32
	Items    []Stack
}

type SaveSpec struct {
	Bag     []Stack
	Belt    []Stack
	Money   float32
	BagSize int32
	Chests  []Chest
}

func DefaultSpec() SaveSpec {
	return SaveSpec{
		Bag:     []Stack{{"faith", 99}, {"salt", 1}, {"heal_potion", 3}, {"heal_potion", 2}},
		Belt:    []Stack{{"hand_tool", 1}, {"axe_1", 1}, {"sword_0", 1}},
		Money:   861,
		BagSize: 25,
		Chests: []Chest{
			{ID: "chest_rough", UniqueID: "chest-yard", Zone: "yard", Size: 20, Items: []Stack{{"wood", 12}, {"stone", 5}}},
			{ID: "chest_kitchen", UniqueID: "chest-home", Zone: "home", Size: 2},
		},
	}
}

func BuildSave(spec SaveSpec) []byte {
	w := NewWriter()
	w.Ref("", "GameSave, Assembly-CSharp").Str("gameSaveVersion", "1.006")
	w.Ref("worldData", "WorldData, Assembly-CSharp").Ref("gameSceneDataList", "List").Array(1)
	w.Ref("", "GameSceneData, Assembly-CSharp").Str("id", "MainScene")
	w.Ref("wgoDataList", "List").Array(len(spec.Chests) + 1)
	for _, c := range spec.Chests {
		w.Ref("", "WgoData, Assembly-CSharp").Str("id", c.ID)
		w.Ref("uniqueId", "SGuid, Assembly-CSharp").Str("id", c.UniqueID).End()
		w.Str("worldZoneDataId", c.Zone)
		w.Container("inventory", c.Items, c.Size)
		w.End()
	}
	w.Ref("", "ZombieWgoData, Assembly-CSharp").Str("name", "zombie_name_1").Int("zombieType", 1)
	w.Str("worldZoneDataId", "yard")
	w.Ref("zombieItem", ItemType).Str("id", "body_zombie").Int("count", 1)
	w.Ref("inventory", ItemList).Array(3)
	for _, p := range []string{"skin_0_0:1", "brain_1_0:1", "guts_0_0:1"} {
		w.Item(p, 1)
	}
	w.EndArray().End().Int("inventorySize", 0).Int("inventoryFillSize", -1)
	w.Ref("properties", PropsType).Array(0).EndArray().End().End()
	w.Int("techRed", 10).Int("techBlue", 0).Int("techGreen", 5).End()
	w.EndArray().End().End()
	w.EndArray().End().End()
	w.Ref("playerData", "PlayerData, Assembly-CSharp")
	w.Container("inventory", spec.Bag, spec.BagSize)
	w.Container("toolBeltInventory", spec.Belt, 14)
	w.Ref("res", "LazyBearTechnology.GameRes, LazyBearTechnology").Ref("resValues", "List").Array(3)
	res := []struct {
		t string
		v float32
	}{{"money", spec.Money}, {"tech_red", 100}, {"energy", 95.5}}
	for _, r := range res {
		w.Ref("", "GameResAtom").Str("type", r.t).Float("value", r.v).End()
	}
	w.EndArray().End().End()
	w.End()
	w.Ref("talentSystemData", "TalentSystemData, Assembly-CSharp").Ref("talentData", "List").Array(2)
	for i, id := range []string{"talent_orange", "talent_red"} {
		w.Ref("", "TalentData, Assembly-CSharp").Str("id", id).Int("curExp", 1).Int("curTalentLevel", 4)
		w.Int("talentExpPoints", int32(2+i)).Int("curTalentValue", 1)
		insp := [][3]int32{{3, 6}, {13, 11}}
		w.Ref("inspirationsProgression", "List").Array(len(insp))
		for j, p := range insp {
			w.Ref("", "InspirationProgressData, Assembly-CSharp").Str("id", fmt.Sprintf("insp_%s_%d", id[7:], j))
			w.Int("currentValue", p[0]).Int("completionGoalValue", p[1]).End()
		}
		w.EndArray().End().End()
	}
	w.EndArray().End().End()
	w.Ref("knowledgeSystem", "KnowledgeSystem, Assembly-CSharp")
	techs := []struct {
		field string
		list  []string
	}{{"unlockedTechs", []string{"garden_improve_1", "candles_1"}}, {"hiddenTechs", []string{"zombie_wood"}}, {"revealedTechs", []string{"garden_honey"}}}
	for _, tl := range techs {
		field, list := tl.field, tl.list
		w.Ref(field, "System.Collections.Generic.List`1[[System.String, mscorlib]], mscorlib").Array(len(list))
		for _, v := range list {
			w.Str("", v)
		}
		w.EndArray().End()
	}
	w.End()
	w.End()
	return w.Bytes()
}
