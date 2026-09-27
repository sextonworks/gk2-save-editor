package session

import (
	"cmp"
	"slices"
	"strings"

	"github.com/sextonworks/gk2-save-editor/internal/save"
)

const (
	GroupMain       = "main"
	GroupReputation = "reputation"
	GroupZones      = "zones"
	GroupPerks      = "perks"
	GroupOther      = "other"

	storageFallback = "chest"
	zonePrefix      = "wz_"
	perkPrefix      = "perk_"
	repSuffix       = "_REP"
	npcPrefix       = "npc_"
	placeSuffix     = "_place"
	storageNameKey  = "ui_storage"
)

var mainResources = []string{"energy", "stamina", "insanity", "happiness", "tech_red", "tech_green", "tech_blue"}

func (s *Session) lookupName(lang string, keys ...string) string {
	for _, k := range keys {
		if n := s.name(k, lang); n != k {
			return n
		}
	}
	return ""
}

func (s *Session) resource(r save.Resource, lang string) ResourceView {
	v := ResourceView{Type: r.Type, Value: r.Value, Group: GroupOther, Editable: true}
	switch {
	case slices.Contains(mainResources, r.Type):
		v.Group, v.Name, v.Icon = GroupMain, s.lookupName(lang, r.Type), r.Type
	case strings.HasSuffix(r.Type, repSuffix):
		who := strings.TrimSuffix(r.Type, repSuffix)
		v.Group, v.Name = GroupReputation, s.lookupName(lang, who, npcPrefix+who)
	case strings.HasPrefix(r.Type, zonePrefix):
		v.Group, v.Name, v.Editable = GroupZones, s.lookupName(lang, r.Type), false
	case strings.HasPrefix(r.Type, perkPrefix):
		v.Group, v.Name, v.Editable = GroupPerks, s.lookupName(lang, r.Type), false
	}
	return v
}

func (s *Session) isStorage(id string) bool {
	if s.catalog != nil && len(s.catalog.Storages) > 0 {
		_, ok := s.catalog.StorageCapacity(id)
		return ok
	}
	return strings.Contains(id, storageFallback)
}

func (s *Session) storageName(id, lang string) string {
	return cmp.Or(s.lookupName(lang, id, id+placeSuffix), s.lookupName(lang, storageNameKey), id)
}

func (s *Session) Storages(lang string) ([]StorageView, error) {
	out := []StorageView{}
	err := s.read(func(sv *save.Save) error {
		list, err := sv.Storages()
		if err != nil {
			return err
		}
		for _, st := range list {
			if !s.isStorage(st.ID) {
				continue
			}
			v := StorageView{
				UniqueID: string(save.WorldContainer(st.UniqueID)), ID: st.ID, Zone: st.Zone, Capacity: st.Capacity,
				Name:     s.storageName(st.ID, lang),
				ZoneName: s.lookupName(lang, zonePrefix+st.Zone),
				Items:    make([]ItemView, 0, len(st.Items)),
			}
			for _, it := range st.Items {
				v.Items = append(v.Items, s.item(it, lang))
			}
			out = append(out, v)
		}
		return nil
	})
	slices.SortStableFunc(out, func(a, b StorageView) int {
		return cmp.Or(cmp.Compare(unzoned(a), unzoned(b)), cmp.Compare(a.ZoneName, b.ZoneName), cmp.Compare(a.Name, b.Name))
	})
	return out, err
}

func unzoned(v StorageView) int {
	if v.ZoneName == "" {
		return 1
	}
	return 0
}
