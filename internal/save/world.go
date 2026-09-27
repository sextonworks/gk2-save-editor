package save

import (
	"fmt"
	"strings"

	"github.com/sextonworks/gk2-save-editor/internal/odin"
)

const worldPrefix = "wgo:"

type Storage struct {
	UniqueID string  `json:"uniqueId"`
	ID       string  `json:"id"`
	Zone     string  `json:"zone"`
	Scene    string  `json:"scene"`
	Capacity int64   `json:"capacity"`
	Items    []Stack `json:"items"`
}

func WorldContainer(uniqueID string) Container {
	return Container(worldPrefix + uniqueID)
}

func (c Container) world() (string, bool) {
	return strings.CutPrefix(string(c), worldPrefix)
}

type worldObject struct {
	node  *odin.Node
	scene string
}

func (s *Save) worldObjects() ([]worldObject, error) {
	scenes, err := array(s.game, "worldData", "gameSceneDataList")
	if err != nil {
		return nil, err
	}
	var out []worldObject
	for _, sc := range scenes.Children {
		list, err := array(sc, "wgoDataList")
		if err != nil {
			continue
		}
		for _, w := range list.Children {
			out = append(out, worldObject{node: w, scene: str(sc, "id")})
		}
	}
	return out, nil
}

func worldID(w *odin.Node) string {
	if uid, ok := w.Child("uniqueId"); ok {
		return str(uid, "id")
	}
	return ""
}

func (s *Save) worldInventory(uniqueID string) (*odin.Node, error) {
	objs, err := s.worldObjects()
	if err != nil {
		return nil, err
	}
	for _, o := range objs {
		if uniqueID != "" && worldID(o.node) == uniqueID {
			return child(o.node, "inventory", "inventoryItem")
		}
	}
	return nil, fmt.Errorf("world object %s: %w", uniqueID, ErrContainer)
}

func (s *Save) Storages() ([]Storage, error) {
	objs, err := s.worldObjects()
	if err != nil {
		return nil, fmt.Errorf("storages: %w", err)
	}
	out := make([]Storage, 0, 32)
	for _, o := range objs {
		holder, err := child(o.node, "inventory", "inventoryItem")
		if err != nil {
			continue
		}
		size := integer(holder, "inventorySize")
		arr, err := array(holder, "inventory")
		if err != nil || size <= 0 {
			continue
		}
		st := Storage{
			UniqueID: worldID(o.node),
			ID:       str(o.node, "id"),
			Zone:     str(o.node, "worldZoneDataId"),
			Scene:    o.scene,
			Capacity: size,
			Items:    make([]Stack, 0, len(arr.Children)),
		}
		for _, it := range arr.Children {
			st.Items = append(st.Items, stackOf(it))
		}
		out = append(out, st)
	}
	return out, nil
}
