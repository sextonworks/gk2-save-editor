package save

import (
	"fmt"
	"strings"
)

var (
	BestBody = map[string]string{
		"skin": "skin_3_0:3", "bones": "bones_3_1:3", "skull": "skull_3_2:3",
		"heart": "heart_3_1:3", "brain": "brain_3_1:3", "guts": "guts_3_0:3",
	}
	BestBelt = map[string]string{
		"axe": "axe_3", "pickaxe": "pickaxe_3", "hammer": "hammer_3", "shovel": "shovel_3",
		"small_instruments": "small_instruments_3", "sword": "sword_4", "armor": "armor_4",
		"bow": "bow_3", "surgical_kit": "surgical_kit_3", "alchemy_kit": "alchemy_kit_3",
		"tool_book": "tool_book_3", "tool_talisman": "tool_talisman_3",
	}
)

type Op interface {
	Apply(s *Save) error
	String() string
}

type SetResource struct {
	Type  string
	Value float64
}

func (o SetResource) Apply(s *Save) error {
	n, err := s.resourceNode(o.Type)
	if err != nil {
		return err
	}
	return s.setNumber(n, o.Value)
}

func (o SetResource) String() string {
	return fmt.Sprintf("set %s = %g", o.Type, o.Value)
}

type SetItemCount struct {
	Container Container
	UniqueID  string
	Count     int64
}

func (o SetItemCount) Apply(s *Save) error {
	it, err := s.itemNode(o.Container, o.UniqueID)
	if err != nil {
		return err
	}
	return s.setField(it, "count", float64(o.Count))
}

func (o SetItemCount) String() string {
	return fmt.Sprintf("%s: stack %s count = %d", o.Container, o.UniqueID, o.Count)
}

type SetItemID struct {
	Container Container
	UniqueID  string
	ItemID    string
}

func (o SetItemID) Apply(s *Save) error {
	it, err := s.itemNode(o.Container, o.UniqueID)
	if err != nil {
		return err
	}
	n, err := child(it, "id")
	if err != nil {
		return err
	}
	return s.setString(n, o.ItemID)
}

func (o SetItemID) String() string {
	return fmt.Sprintf("%s: stack %s id = %s", o.Container, o.UniqueID, o.ItemID)
}

type AddItem struct {
	Container Container
	ItemID    string
	Count     int64
	UniqueID  string
}

func NewAddItem(c Container, itemID string, count int64) AddItem {
	return AddItem{Container: c, ItemID: itemID, Count: count, UniqueID: NewUUID()}
}

func (o AddItem) Apply(s *Save) error {
	return s.insertItem(o.Container, o.ItemID, o.Count, o.UniqueID)
}

func (o AddItem) String() string {
	return fmt.Sprintf("%s: + %s x%d", o.Container, o.ItemID, o.Count)
}

type RemoveItem struct {
	Container Container
	UniqueID  string
}

func (o RemoveItem) Apply(s *Save) error {
	return s.deleteItem(o.Container, o.UniqueID)
}

func (o RemoveItem) String() string {
	return fmt.Sprintf("%s: - stack %s", o.Container, o.UniqueID)
}

type SetTalentPoints struct {
	Talent string
	Points int64
}

func (o SetTalentPoints) Apply(s *Save) error {
	t, err := s.talentNode(o.Talent)
	if err != nil {
		return err
	}
	return s.setField(t, "talentExpPoints", float64(o.Points))
}

func (o SetTalentPoints) String() string {
	return fmt.Sprintf("%s free points = %d", o.Talent, o.Points)
}

type MaxZombie struct {
	Name   string
	Tech   int64
	Brains int64
}

func (o MaxZombie) Apply(s *Save) error {
	z, err := s.zombieNode(o.Name)
	if err != nil {
		return err
	}
	parts, err := array(z, "zombieItem", "inventory")
	if err != nil {
		return err
	}
	for i := range len(parts.Children) {
		if z, err = s.zombieNode(o.Name); err != nil {
			return err
		}
		if parts, err = array(z, "zombieItem", "inventory"); err != nil {
			return err
		}
		idNode, err := child(parts.Children[i], "id")
		if err != nil {
			return err
		}
		kind, _, _ := strings.Cut(idNode.Str, "_")
		best, ok := BestBody[kind]
		if ok && idNode.Str != best {
			if err := s.setString(idNode, best); err != nil {
				return err
			}
		}
	}
	if z, err = s.zombieNode(o.Name); err != nil {
		return err
	}
	if parts, err = array(z, "zombieItem", "inventory"); err != nil {
		return err
	}
	for _, p := range parts.Children {
		if strings.HasPrefix(str(p, "id"), "brain_") {
			if err := s.setField(p, "count", float64(o.Brains)); err != nil {
				return err
			}
		}
	}
	for _, f := range []string{"techRed", "techBlue", "techGreen"} {
		if err := s.setField(z, f, float64(o.Tech)); err != nil {
			return err
		}
	}
	return nil
}

func (o MaxZombie) String() string {
	return fmt.Sprintf("zombie %s: best body parts, %d brains, tech %d", o.Name, o.Brains, o.Tech)
}

type EquipBest struct{}

func (EquipBest) Apply(s *Save) error {
	items, err := s.Items(Belt)
	if err != nil {
		return err
	}
	for _, it := range items {
		fam := it.ID
		if i := strings.LastIndexByte(fam, '_'); i > 0 {
			fam = fam[:i]
		}
		best, ok := BestBelt[fam]
		if !ok || best == it.ID {
			continue
		}
		if err := (SetItemID{Container: Belt, UniqueID: it.UniqueID, ItemID: best}).Apply(s); err != nil {
			return err
		}
	}
	return nil
}

func (EquipBest) String() string {
	return "belt: top tier on every slot"
}
