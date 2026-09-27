package main

import (
	"fmt"
	"io"

	"github.com/sextonworks/gk2-save-editor/internal/save"
)

func (c *cli) nameOf(g *globals, id string) string {
	cat, err := c.catalog(g, false)
	if err != nil {
		return ""
	}
	if n := cat.BaseName(id, c.Lang); n != id {
		return n
	}
	return ""
}

type inspirationsCmd struct {
	jsonFlag
}

func (a *inspirationsCmd) Run(c *cli, g *globals) error {
	s, err := c.open(g)
	if err != nil {
		return err
	}
	list, err := s.Inspirations()
	if err != nil {
		return err
	}
	return emit(g, a.jsonFlag, list, func(w io.Writer) {
		for _, i := range list {
			mark := " "
			if i.Ready() {
				mark = "*"
			}
			fmt.Fprintf(w, "%s %-14s %-28s %5d / %-5d %s\n", mark, i.Talent, i.ID, i.Current, i.Goal, c.nameOf(g, i.ID))
		}
		fmt.Fprintln(w, "* = ready to buy in the game")
	})
}

type techsCmd struct {
	jsonFlag
}

func (a *techsCmd) Run(c *cli, g *globals) error {
	s, err := c.open(g)
	if err != nil {
		return err
	}
	techs := s.Techs()
	return emit(g, a.jsonFlag, techs, func(w io.Writer) {
		for _, group := range []struct {
			title string
			ids   []string
		}{{"Unlocked", techs.Unlocked}, {"Available", techs.Revealed}, {"Hidden", techs.Hidden}} {
			fmt.Fprintf(w, "%s (%d)\n", group.title, len(group.ids))
			for _, id := range group.ids {
				fmt.Fprintf(w, "  %-36s %s\n", id, c.nameOf(g, id))
			}
		}
	})
}

type inspireCmd struct {
	Talent string   `help:"Only this talent branch, e.g. talent_red."`
	IDs    []string `arg:"" optional:"" name:"id" help:"Inspiration ids (default: every unfinished one)."`
}

func (a *inspireCmd) Run(c *cli, g *globals) error {
	want := map[string]bool{}
	for _, id := range a.IDs {
		want[id] = true
	}
	return c.edit(g, func(e *save.Editor) error {
		list, err := e.Save().Inspirations()
		if err != nil {
			return fmt.Errorf("inspire: %w", err)
		}
		matched := 0
		for _, i := range list {
			if (a.Talent != "" && i.Talent != a.Talent) || (len(want) > 0 && !want[i.ID]) {
				continue
			}
			matched++
			if !i.CanBringToGoal() {
				continue
			}
			if err := e.Apply(save.SetInspirationProgress{Talent: i.Talent, ID: i.ID, Value: i.Goal}); err != nil {
				return fmt.Errorf("inspire: %w", err)
			}
		}
		if len(want) > 0 && matched < len(want) {
			return fmt.Errorf("inspire: some ids were not found: %w", save.ErrField)
		}
		return nil
	})
}
