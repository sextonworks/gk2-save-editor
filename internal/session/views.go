package session

import (
	"fmt"
	"strings"

	"github.com/sextonworks/gk2-save-editor/internal/gamedata"
	"github.com/sextonworks/gk2-save-editor/internal/odin"
	"github.com/sextonworks/gk2-save-editor/internal/save"
)

type ItemView struct {
	UniqueID string `json:"uniqueId"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Count    int64  `json:"count"`
}

type ContainerView struct {
	Items    []ItemView `json:"items"`
	Capacity int64      `json:"capacity"`
}

type ResourceView struct {
	Type  string  `json:"type"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type TalentView struct {
	save.Talent
	Name string `json:"name"`
}

type Player struct {
	Money     float64        `json:"money"`
	Resources []ResourceView `json:"resources"`
	Talents   []TalentView   `json:"talents"`
}

type ZombieView struct {
	save.Zombie
	DisplayName string     `json:"displayName"`
	Parts       []ItemView `json:"parts"`
	PerkNames   []string   `json:"perkNames"`
}

type InspirationView struct {
	save.Inspiration
	Name        string `json:"name"`
	Description string `json:"description"`
	Ready       bool   `json:"ready"`
}

type TechView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

func (s *Session) name(id, lang string) string {
	if s.catalog == nil {
		return id
	}
	return s.catalog.Name(id, lang)
}

func (s *Session) baseName(id, lang string) string {
	if s.catalog == nil {
		return id
	}
	return s.catalog.BaseName(id, lang)
}

func (s *Session) read(fn func(sv *save.Save) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.editor == nil {
		return ErrNoSave
	}
	return fn(s.editor.Save())
}

func (s *Session) Container(container, lang string) (ContainerView, error) {
	view := ContainerView{Items: []ItemView{}}
	err := s.read(func(sv *save.Save) error {
		items, err := sv.Items(save.Container(container))
		if err != nil {
			return err
		}
		for _, it := range items {
			view.Items = append(view.Items, ItemView{UniqueID: it.UniqueID, ID: it.ID, Name: s.name(it.ID, lang), Count: it.Count})
		}
		view.Capacity, err = sv.Capacity(save.Container(container))
		return err
	})
	return view, err
}

func (s *Session) Player(lang string) (Player, error) {
	p := Player{Resources: []ResourceView{}, Talents: []TalentView{}}
	err := s.read(func(sv *save.Save) error {
		res, err := sv.Resources()
		if err != nil {
			return err
		}
		for _, r := range res {
			if r.Type == "money" {
				p.Money = r.Value
			}
			p.Resources = append(p.Resources, ResourceView{Type: r.Type, Name: s.name(r.Type, lang), Value: r.Value})
		}
		talents, err := sv.Talents()
		if err != nil {
			return err
		}
		for _, t := range talents {
			p.Talents = append(p.Talents, TalentView{Talent: t, Name: s.name(t.ID, lang)})
		}
		return nil
	})
	return p, err
}

func (s *Session) Zombies(lang string) ([]ZombieView, error) {
	out := []ZombieView{}
	err := s.read(func(sv *save.Save) error {
		zombies, err := sv.Zombies()
		if err != nil {
			return err
		}
		for _, z := range zombies {
			v := ZombieView{Zombie: z, DisplayName: s.name(z.Name, lang), Parts: []ItemView{}, PerkNames: []string{}}
			for _, p := range z.BodyParts {
				v.Parts = append(v.Parts, ItemView{UniqueID: p.UniqueID, ID: p.ID, Name: s.name(p.ID, lang), Count: p.Count})
			}
			for _, perk := range z.Perks {
				v.PerkNames = append(v.PerkNames, s.name(perk, lang))
			}
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

func (s *Session) Inspirations(lang string) ([]InspirationView, error) {
	out := []InspirationView{}
	err := s.read(func(sv *save.Save) error {
		list, err := sv.Inspirations()
		if err != nil {
			return err
		}
		for _, i := range list {
			v := InspirationView{Inspiration: i, Name: s.baseName(i.ID, lang), Ready: i.Ready()}
			if s.catalog != nil {
				v.Description = s.catalog.Description(i.ID, lang)
			}
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

func (s *Session) Techs(lang string) ([]TechView, error) {
	out := []TechView{}
	err := s.read(func(sv *save.Save) error {
		t := sv.Techs()
		for _, group := range []struct {
			state string
			ids   []string
		}{{"unlocked", t.Unlocked}, {"available", t.Revealed}, {"hidden", t.Hidden}} {
			for _, id := range group.ids {
				out = append(out, TechView{ID: id, Name: s.name(id, lang), State: group.state})
			}
		}
		return nil
	})
	return out, err
}

func (s *Session) SearchItems(query, lang string, all bool, limit int) ([]gamedata.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.catalog == nil {
		return []gamedata.Entry{}, ErrNoCatalog
	}
	return s.catalog.Search(query, lang, !all, limit), nil
}

func (s *Session) SetMoney(v float64) (State, error) {
	return s.edit(save.SetResource{Type: "money", Value: v})
}

func (s *Session) SetResource(resType string, v float64) (State, error) {
	return s.edit(save.SetResource{Type: resType, Value: v})
}

func (s *Session) SetTalentPoints(talent string, points int64) (State, error) {
	return s.edit(save.SetTalentPoints{Talent: talent, Points: points})
}

func (s *Session) SetItemCount(container, uniqueID string, count int64) (State, error) {
	return s.edit(save.SetItemCount{Container: save.Container(container), UniqueID: uniqueID, Count: count})
}

func (s *Session) RemoveItem(container, uniqueID string) (State, error) {
	return s.edit(save.RemoveItem{Container: save.Container(container), UniqueID: uniqueID})
}

func (s *Session) checkID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.catalog != nil && !s.catalog.Known(id) {
		return fmt.Errorf("%q is not a known id", id)
	}
	return nil
}

func (s *Session) AddItem(container, id string, count int64) (State, error) {
	if err := s.checkID(id); err != nil {
		return s.State(), err
	}
	return s.edit(save.NewAddItem(save.Container(container), id, count))
}

func (s *Session) SwapItem(container, uniqueID, newID string) (State, error) {
	if err := s.checkID(newID); err != nil {
		return s.State(), err
	}
	return s.edit(save.SetItemID{Container: save.Container(container), UniqueID: uniqueID, ItemID: newID})
}

func (s *Session) EquipBest() (State, error) {
	return s.edit(save.EquipBest{})
}

func (s *Session) MaxZombie(name string, tech, brains int64) (State, error) {
	return s.edit(save.MaxZombie{Name: name, Tech: tech, Brains: brains})
}

func (s *Session) Inspire(talent, id string) (State, error) {
	var ops []save.Op
	err := s.read(func(sv *save.Save) error {
		list, err := sv.Inspirations()
		if err != nil {
			return err
		}
		for _, i := range list {
			if (talent == "" || i.Talent == talent) && (id == "" || i.ID == id) && !i.Ready() {
				ops = append(ops, save.SetInspirationProgress{Talent: i.Talent, ID: i.ID, Value: i.Goal})
			}
		}
		return nil
	})
	if err != nil {
		return s.State(), err
	}
	return s.edit(ops...)
}

type NodeView struct {
	Offset   int    `json:"offset"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Children int    `json:"children"`
}

var kindNames = map[odin.Kind]string{
	odin.KindRoot: "root", odin.KindRef: "object", odin.KindStruct: "struct",
	odin.KindArray: "array", odin.KindPrimitiveArray: "bytes", odin.KindValue: "value",
}

func nodeView(n *odin.Node) NodeView {
	v := NodeView{Offset: n.Off, Name: n.Name, Path: n.Path(), Kind: kindNames[n.Kind], Type: n.TypeName, Children: len(n.Children)}
	if !n.Named {
		v.Name = "#"
	}
	switch n.Kind {
	case odin.KindArray, odin.KindPrimitiveArray:
		v.Value = fmt.Sprintf("%d", n.ArrayLen)
	case odin.KindValue:
		v.Value = valueString(n)
	}
	return v
}

func valueString(n *odin.Node) string {
	switch n.Wire {
	case odin.WireString, odin.WireChar:
		return n.Str
	case odin.WireBool:
		return fmt.Sprintf("%t", n.Bool)
	case odin.WireNull:
		return "null"
	case odin.WireFloat32, odin.WireFloat64:
		return fmt.Sprintf("%g", n.Float)
	case odin.WireInternalRef, odin.WireExternalRef:
		return fmt.Sprintf("ref %d", n.Int)
	case odin.WireGUID, odin.WireDecimal:
		return fmt.Sprintf("%x", n.Raw)
	}
	if v, ok := n.Integer(); ok {
		return fmt.Sprintf("%d", v)
	}
	return ""
}

func (s *Session) InspectChildren(offset, from, limit int) ([]NodeView, error) {
	out := []NodeView{}
	err := s.read(func(sv *save.Save) error {
		parent := sv.Root()
		if offset >= 0 {
			odin.Walk(sv.Root(), func(n *odin.Node) bool {
				if n.Kind != odin.KindRoot && n.Off == offset && len(n.Children) > 0 {
					parent = n
					return false
				}
				return true
			})
		}
		end := min(len(parent.Children), from+limit)
		for i := max(from, 0); i < end; i++ {
			out = append(out, nodeView(parent.Children[i]))
		}
		return nil
	})
	return out, err
}

func (s *Session) InspectSearch(query string, limit int) ([]NodeView, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	out := []NodeView{}
	if q == "" {
		return out, nil
	}
	err := s.read(func(sv *save.Save) error {
		odin.Walk(sv.Root(), func(n *odin.Node) bool {
			if n.Kind == odin.KindRoot {
				return true
			}
			if strings.Contains(strings.ToLower(n.Name), q) || (n.Kind == odin.KindValue && strings.Contains(strings.ToLower(valueString(n)), q)) {
				out = append(out, nodeView(n))
			}
			return len(out) < limit
		})
		return nil
	})
	return out, err
}
