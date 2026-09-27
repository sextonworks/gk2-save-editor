package session

import (
	"fmt"
	"strconv"

	"github.com/sextonworks/gk2-save-editor/internal/save"
)

type Change struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Value   string `json:"value"`
	Where   string `json:"where"`
}

func (s *Session) itemNames(lang string) (items, places map[string]string) {
	items, places = map[string]string{}, map[string]string{}
	collect := func(sv *save.Save) {
		for _, c := range []save.Container{save.Bag, save.Belt} {
			list, err := sv.Items(c)
			if err != nil {
				continue
			}
			for _, it := range list {
				items[it.UniqueID] = it.ID
			}
		}
		storages, err := sv.Storages()
		if err != nil {
			return
		}
		for _, st := range storages {
			places[string(save.WorldContainer(st.UniqueID))] = s.storageName(st.ID, lang)
			for _, it := range st.Items {
				items[it.UniqueID] = it.ID
			}
		}
	}
	if orig, err := s.editor.Original(); err == nil {
		collect(orig)
	}
	collect(s.editor.Save())
	return items, places
}

func number(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func (s *Session) changes() []Change {
	lang := s.lang
	if lang == "" {
		lang = "en"
	}
	names, places := s.itemNames(lang)
	item := func(uid string) string {
		if id, ok := names[uid]; ok {
			return s.name(id, lang)
		}
		return ""
	}
	where := func(c save.Container) string {
		if p, ok := places[string(c)]; ok {
			return p
		}
		return string(c)
	}
	ops := s.editor.Ops()
	out := make([]Change, 0, len(ops))
	for _, op := range ops {
		var c Change
		switch o := op.(type) {
		case save.SetResource:
			c = Change{Kind: "resource", Subject: s.name(o.Type, lang), Value: number(o.Value)}
			if o.Type == "money" {
				c.Kind = "money"
			}
		case save.SetItemCount:
			c = Change{Kind: "count", Subject: item(o.UniqueID), Value: strconv.FormatInt(o.Count, 10), Where: where(o.Container)}
		case save.AddItem:
			c = Change{Kind: "add", Subject: s.name(o.ItemID, lang), Value: strconv.FormatInt(o.Count, 10), Where: where(o.Container)}
		case save.RemoveItem:
			c = Change{Kind: "remove", Subject: item(o.UniqueID), Where: where(o.Container)}
		case save.SetItemID:
			c = Change{Kind: "replace", Subject: s.name(o.ItemID, lang), Where: where(o.Container)}
		case save.SetTalentPoints:
			c = Change{Kind: "talent", Subject: o.Talent, Value: strconv.FormatInt(o.Points, 10)}
		case save.MaxZombie:
			c = Change{Kind: "zombie", Subject: s.name(o.Name, lang)}
		case save.EquipBest:
			c = Change{Kind: "equip"}
		case save.SetInspirationProgress:
			c = Change{Kind: "inspire", Subject: s.baseName(o.ID, lang)}
		default:
			c = Change{Kind: "other", Subject: fmt.Sprint(op)}
		}
		out = append(out, c)
	}
	return out
}
