package gamedata

import (
	"slices"

	"github.com/sextonworks/gk2-save-editor/internal/typetree"
)

const (
	bagGroup         = "u_bag"
	interactionChest = 6
)

type ItemDef struct {
	Icon  string `json:"icon,omitempty"`
	Stack int    `json:"stack,omitempty"`
}

type balanceData struct {
	items    []string
	defs     map[string]ItemDef
	storages map[string]int
}

func readBalance(raw []byte) (*balanceData, error) {
	root, err := typetree.Load(balanceName)
	if err != nil {
		return nil, err
	}
	m, err := typetree.Read(root, raw)
	if err != nil {
		return nil, err
	}
	b := &balanceData{defs: map[string]ItemDef{}, storages: map[string]int{}}
	for _, it := range typetree.List(m, "itemDefs") {
		id := typetree.Str(it, "id")
		if id == "" {
			continue
		}
		icon := typetree.Str(it, "customIcon")
		if icon == "" {
			icon = typetree.Str(it, "iconId")
		}
		b.defs[id] = ItemDef{Icon: icon, Stack: int(typetree.Int(it, "stackCount"))}
		if slices.Contains(typetree.Strings(it, "itemGroupIds"), bagGroup) {
			b.items = append(b.items, id)
		}
	}
	for _, w := range typetree.List(m, "wgoDefs") {
		size := int(typetree.Int(w, "inventorySize"))
		if typetree.Int(w, "interactionType") == interactionChest && size > 0 {
			b.storages[typetree.Str(w, "id")] = size
		}
	}
	slices.Sort(b.items)
	b.items = slices.Compact(b.items)
	return b, nil
}
