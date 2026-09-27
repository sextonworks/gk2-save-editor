package save

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sextonworks/gk2-save-editor/internal/odin"
)

var (
	ErrField     = errors.New("field not found")
	ErrContainer = errors.New("unknown container")
	ErrNoItem    = errors.New("item not in container")
)

type Container string

const (
	Bag  Container = "bag"
	Belt Container = "belt"
)

var containerField = map[Container]string{Bag: "inventory", Belt: "toolBeltInventory"}

type Info struct {
	Day              int    `json:"day"`
	SaveDateTime     string `json:"saveDateTime"`
	GameSaveVersion  string `json:"gameSaveVersion"`
	GraveyardQuality int    `json:"graveyardQuality"`
	ChurchQuality    int    `json:"churchQuality"`
}

type Stack struct {
	ID       string `json:"id"`
	Count    int64  `json:"count"`
	UniqueID string `json:"uniqueId"`
}

type Resource struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

type Talent struct {
	ID         string `json:"id"`
	Level      int64  `json:"level"`
	Exp        int64  `json:"exp"`
	FreePoints int64  `json:"freePoints"`
	Value      int64  `json:"value"`
}

type Zombie struct {
	Name      string   `json:"name"`
	Type      int64    `json:"type"`
	Zone      string   `json:"zone"`
	TechRed   int64    `json:"techRed"`
	TechBlue  int64    `json:"techBlue"`
	TechGreen int64    `json:"techGreen"`
	BodyParts []Stack  `json:"bodyParts"`
	Perks     []string `json:"perks"`
}

type Save struct {
	path string
	data []byte
	root *odin.Node
	game *odin.Node
}

func Open(path string) (*Save, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read save: %w", err)
	}
	return Load(path, data)
}

func Load(path string, data []byte) (*Save, error) {
	root, err := odin.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse save: %w", err)
	}
	if len(root.Children) == 0 || !strings.HasPrefix(root.Children[0].TypeName, "GameSave") {
		return nil, fmt.Errorf("parse save: root is not GameSave: %w", odin.ErrMalformed)
	}
	return &Save{path: path, data: data, root: root, game: root.Children[0]}, nil
}

func (s *Save) Path() string {
	return s.path
}

func (s *Save) InfoPath() string {
	return strings.TrimSuffix(s.path, ".dat") + ".info"
}

func (s *Save) Info() (Info, error) {
	var info Info
	data, err := os.ReadFile(s.InfoPath())
	if err != nil {
		return info, fmt.Errorf("read info: %w", err)
	}
	data = []byte(strings.TrimPrefix(string(data), "\uFEFF"))
	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("decode info: %w", err)
	}
	return info, nil
}

func child(n *odin.Node, names ...string) (*odin.Node, error) {
	cur := n
	for _, name := range names {
		next, ok := cur.Child(name)
		if !ok {
			return nil, fmt.Errorf("%s/%s: %w", cur.Path(), name, ErrField)
		}
		cur = next
	}
	return cur, nil
}

func array(n *odin.Node, names ...string) (*odin.Node, error) {
	holder, err := child(n, names...)
	if err != nil {
		return nil, err
	}
	arr, ok := holder.Array()
	if !ok {
		return nil, fmt.Errorf("%s: array: %w", holder.Path(), ErrField)
	}
	return arr, nil
}

func str(n *odin.Node, name string) string {
	if c, ok := n.Child(name); ok {
		return c.Str
	}
	return ""
}

func integer(n *odin.Node, name string) int64 {
	if c, ok := n.Child(name); ok {
		if v, ok := c.Integer(); ok {
			return v
		}
	}
	return 0
}

func stackOf(item *odin.Node) Stack {
	st := Stack{ID: str(item, "id"), Count: integer(item, "count")}
	if uid, ok := item.Child("uniqueId"); ok {
		st.UniqueID = str(uid, "id")
	}
	return st
}

func (s *Save) containerArray(c Container) (holder, arr *odin.Node, err error) {
	field, ok := containerField[c]
	if !ok {
		return nil, nil, fmt.Errorf("%q: %w", c, ErrContainer)
	}
	holder, err = child(s.game, "playerData", field, "inventoryItem")
	if err != nil {
		return nil, nil, err
	}
	arr, err = array(holder, "inventory")
	return holder, arr, err
}

func (s *Save) Items(c Container) ([]Stack, error) {
	_, arr, err := s.containerArray(c)
	if err != nil {
		return nil, fmt.Errorf("items %s: %w", c, err)
	}
	out := make([]Stack, 0, len(arr.Children))
	for _, it := range arr.Children {
		out = append(out, stackOf(it))
	}
	return out, nil
}

func (s *Save) Capacity(c Container) (int64, error) {
	holder, _, err := s.containerArray(c)
	if err != nil {
		return 0, fmt.Errorf("capacity %s: %w", c, err)
	}
	return integer(holder, "inventorySize"), nil
}

func (s *Save) Resources() ([]Resource, error) {
	arr, err := array(s.game, "playerData", "res", "resValues")
	if err != nil {
		return nil, fmt.Errorf("resources: %w", err)
	}
	out := make([]Resource, 0, len(arr.Children))
	for _, a := range arr.Children {
		r := Resource{Type: str(a, "type")}
		if v, ok := a.Child("value"); ok {
			r.Value = v.Float
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Save) Resource(resType string) (float64, error) {
	res, err := s.Resources()
	if err != nil {
		return 0, fmt.Errorf("resource: %w", err)
	}
	for _, r := range res {
		if r.Type == resType {
			return r.Value, nil
		}
	}
	return 0, fmt.Errorf("resource %q: %w", resType, ErrField)
}

func (s *Save) Talents() ([]Talent, error) {
	arr, err := array(s.game, "talentSystemData", "talentData")
	if err != nil {
		return nil, fmt.Errorf("talents: %w", err)
	}
	out := make([]Talent, 0, len(arr.Children))
	for _, t := range arr.Children {
		out = append(out, Talent{
			ID:         str(t, "id"),
			Level:      integer(t, "curTalentLevel"),
			Exp:        integer(t, "curExp"),
			FreePoints: integer(t, "talentExpPoints"),
			Value:      integer(t, "curTalentValue"),
		})
	}
	return out, nil
}

func (s *Save) zombieNodes() ([]*odin.Node, error) {
	world, err := child(s.game, "worldData")
	if err != nil {
		return nil, fmt.Errorf("zombies: %w", err)
	}
	nodes := make([]*odin.Node, 0, 4)
	odin.Walk(world, func(n *odin.Node) bool {
		if n.Named && n.Name == "zombieType" {
			nodes = append(nodes, n.Parent)
		}
		return true
	})
	return nodes, nil
}

func (s *Save) Zombies() ([]Zombie, error) {
	nodes, err := s.zombieNodes()
	if err != nil {
		return nil, fmt.Errorf("zombies: %w", err)
	}
	out := make([]Zombie, 0, len(nodes))
	for _, z := range nodes {
		zb := Zombie{
			Name:      str(z, "name"),
			Type:      integer(z, "zombieType"),
			Zone:      str(z, "worldZoneDataId"),
			TechRed:   integer(z, "techRed"),
			TechBlue:  integer(z, "techBlue"),
			TechGreen: integer(z, "techGreen"),
			BodyParts: make([]Stack, 0, 8),
			Perks:     make([]string, 0, 4),
		}
		if parts, err := array(z, "zombieItem", "inventory"); err == nil {
			for _, p := range parts.Children {
				zb.BodyParts = append(zb.BodyParts, stackOf(p))
			}
		}
		if perks, err := array(z, "activePerks"); err == nil {
			for _, p := range perks.Children {
				zb.Perks = append(zb.Perks, str(p, "id"))
			}
		}
		out = append(out, zb)
	}
	return out, nil
}

func (s *Save) Root() *odin.Node {
	return s.root
}
