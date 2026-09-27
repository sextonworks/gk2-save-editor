package save

import (
	"fmt"
	"slices"
)

type Inspiration struct {
	Talent  string `json:"talent"`
	ID      string `json:"id"`
	Current int64  `json:"current"`
	Goal    int64  `json:"goal"`
}

func (i Inspiration) Ready() bool {
	return i.Goal > 0 && i.Current >= i.Goal
}

func (i Inspiration) CanBringToGoal() bool {
	return i.Goal > i.Current
}

type Techs struct {
	Unlocked []string `json:"unlocked"`
	Revealed []string `json:"revealed"`
	Hidden   []string `json:"hidden"`
}

func (s *Save) Inspirations() ([]Inspiration, error) {
	arr, err := array(s.game, "talentSystemData", "talentData")
	if err != nil {
		return nil, fmt.Errorf("inspirations: %w", err)
	}
	out := make([]Inspiration, 0, 32)
	for _, t := range arr.Children {
		list, err := array(t, "inspirationsProgression")
		if err != nil {
			continue
		}
		for _, p := range list.Children {
			out = append(out, Inspiration{
				Talent:  str(t, "id"),
				ID:      str(p, "id"),
				Current: integer(p, "currentValue"),
				Goal:    integer(p, "completionGoalValue"),
			})
		}
	}
	return out, nil
}

func (s *Save) stringList(names ...string) []string {
	arr, err := array(s.game, names...)
	if err != nil {
		return []string{}
	}
	out := make([]string, 0, len(arr.Children))
	for _, n := range arr.Children {
		out = append(out, n.Str)
	}
	slices.Sort(out)
	return out
}

func (s *Save) Techs() Techs {
	return Techs{
		Unlocked: s.stringList("knowledgeSystem", "unlockedTechs"),
		Revealed: s.stringList("knowledgeSystem", "revealedTechs"),
		Hidden:   s.stringList("knowledgeSystem", "hiddenTechs"),
	}
}

type SetInspirationProgress struct {
	Talent string
	ID     string
	Value  int64
}

func (o SetInspirationProgress) Apply(s *Save) error {
	t, err := s.talentNode(o.Talent)
	if err != nil {
		return err
	}
	list, err := array(t, "inspirationsProgression")
	if err != nil {
		return err
	}
	for _, p := range list.Children {
		if str(p, "id") != o.ID {
			continue
		}
		if o.Value < integer(p, "currentValue") {
			return fmt.Errorf("inspiration %s/%s: progress can only go up: %w", o.Talent, o.ID, ErrField)
		}
		return s.setField(p, "currentValue", float64(o.Value))
	}
	return fmt.Errorf("inspiration %s/%s: %w", o.Talent, o.ID, ErrField)
}

func (o SetInspirationProgress) String() string {
	return fmt.Sprintf("%s: inspiration %s progress = %d", o.Talent, o.ID, o.Value)
}
