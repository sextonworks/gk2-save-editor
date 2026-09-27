package main

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/sextonworks/gk2-save-editor/internal/save"
)

type jsonFlag struct {
	JSON bool `name:"json" help:"Print JSON."`
}

func emit(g *globals, j jsonFlag, v any, text func(io.Writer)) error {
	if !j.JSON {
		text(g.out)
		return nil
	}
	enc := json.NewEncoder(g.out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

func number(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

type infoCmd struct{ jsonFlag }

type infoView struct {
	Save     string     `json:"save"`
	Info     *save.Info `json:"info,omitempty"`
	Money    float64    `json:"money"`
	Backpack int        `json:"backpack"`
	Capacity int64      `json:"capacity"`
	Zombies  int        `json:"zombies"`
	Running  bool       `json:"gameRunning"`
}

func (c *infoCmd) Run(root *cli, g *globals) error {
	s, err := root.open(g)
	if err != nil {
		return fmt.Errorf("info: %w", err)
	}
	v := infoView{Save: s.Path()}
	if info, err := s.Info(); err == nil {
		v.Info = &info
	}
	if v.Money, err = s.Resource("money"); err != nil {
		return fmt.Errorf("info: %w", err)
	}
	bag, err := s.Items(save.Bag)
	if err != nil {
		return fmt.Errorf("info: %w", err)
	}
	v.Backpack = len(bag)
	if v.Capacity, err = s.Capacity(save.Bag); err != nil {
		return fmt.Errorf("info: %w", err)
	}
	zombies, err := s.Zombies()
	if err != nil {
		return fmt.Errorf("info: %w", err)
	}
	v.Zombies = len(zombies)
	if v.Running, err = g.running(g.ctx); err != nil {
		return fmt.Errorf("check game process: %w", err)
	}
	return emit(g, c.jsonFlag, v, func(w io.Writer) {
		fmt.Fprintf(w, "Save:     %s\n", v.Save)
		if v.Info != nil {
			fmt.Fprintf(w, "Day:      %d  saved %s  version %s\n", v.Info.Day, v.Info.SaveDateTime, v.Info.GameSaveVersion)
		}
		fmt.Fprintf(w, "Money:    %s\n", number(v.Money))
		fmt.Fprintf(w, "Backpack: %d/%d\n", v.Backpack, v.Capacity)
		fmt.Fprintf(w, "Zombies:  %d\n", v.Zombies)
		state := "closed"
		if v.Running {
			state = "RUNNING"
		}
		fmt.Fprintf(w, "Game:     %s\n", state)
	})
}

type bagCmd struct{ jsonFlag }

func (c *bagCmd) Run(root *cli, g *globals) error {
	return listContainer(root, g, c.jsonFlag, save.Bag)
}

type beltCmd struct{ jsonFlag }

func (c *beltCmd) Run(root *cli, g *globals) error {
	return listContainer(root, g, c.jsonFlag, save.Belt)
}

func listContainer(root *cli, g *globals, j jsonFlag, c save.Container) error {
	s, err := root.open(g)
	if err != nil {
		return fmt.Errorf("list %s: %w", c, err)
	}
	items, err := s.Items(c)
	if err != nil {
		return fmt.Errorf("list %s: %w", c, err)
	}
	cat, _ := root.catalog(g, false)
	return emit(g, j, items, func(w io.Writer) {
		for _, it := range items {
			name := ""
			if cat != nil {
				if n := cat.Name(it.ID, root.Lang); n != it.ID {
					name = "  " + n
				}
			}
			fmt.Fprintf(w, "%6d  %-28s%s\n", it.Count, it.ID, name)
		}
	})
}

type resCmd struct {
	jsonFlag
	Pattern string `arg:"" optional:"" default:"." help:"Regex filter."`
}

func (c *resCmd) Run(root *cli, g *globals) error {
	re, err := regexp.Compile(c.Pattern)
	if err != nil {
		return fmt.Errorf("pattern: %w", err)
	}
	s, err := root.open(g)
	if err != nil {
		return fmt.Errorf("res: %w", err)
	}
	all, err := s.Resources()
	if err != nil {
		return fmt.Errorf("res: %w", err)
	}
	res := make([]save.Resource, 0, len(all))
	for _, r := range all {
		if re.MatchString(r.Type) {
			res = append(res, r)
		}
	}
	return emit(g, c.jsonFlag, res, func(w io.Writer) {
		for _, r := range res {
			fmt.Fprintf(w, "%12s  %s\n", number(r.Value), r.Type)
		}
	})
}

type talentsCmd struct{ jsonFlag }

func (c *talentsCmd) Run(root *cli, g *globals) error {
	s, err := root.open(g)
	if err != nil {
		return fmt.Errorf("talents: %w", err)
	}
	talents, err := s.Talents()
	if err != nil {
		return fmt.Errorf("talents: %w", err)
	}
	return emit(g, c.jsonFlag, talents, func(w io.Writer) {
		for _, t := range talents {
			fmt.Fprintf(w, "%-15s level %3d  exp %3d  free %4d  value %d\n", t.ID, t.Level, t.Exp, t.FreePoints, t.Value)
		}
	})
}

type zombiesCmd struct{ jsonFlag }

func (c *zombiesCmd) Run(root *cli, g *globals) error {
	s, err := root.open(g)
	if err != nil {
		return fmt.Errorf("zombies: %w", err)
	}
	zombies, err := s.Zombies()
	if err != nil {
		return fmt.Errorf("zombies: %w", err)
	}
	return emit(g, c.jsonFlag, zombies, func(w io.Writer) {
		for _, z := range zombies {
			fmt.Fprintf(w, "%s  type %d  zone %s  tech R%d B%d G%d\n", z.Name, z.Type, z.Zone, z.TechRed, z.TechBlue, z.TechGreen)
			parts := make([]byte, 0, 128)
			for i, p := range z.BodyParts {
				if i > 0 {
					parts = append(parts, ", "...)
				}
				parts = append(parts, p.ID...)
			}
			fmt.Fprintf(w, "    %s\n", parts)
		}
	})
}

type pathsCmd struct{}

func (c *pathsCmd) Run(root *cli, g *globals) error {
	if p, err := root.savePath(g); err == nil {
		fmt.Fprintf(g.out, "%-8s %s\n", "save", p)
	} else {
		fmt.Fprintf(g.out, "%-8s not found (%v)\n", "save", err)
	}
	if p, err := g.env.GameDir(root.GameDir); err == nil {
		fmt.Fprintf(g.out, "%-8s %s\n", "game", p)
	} else {
		fmt.Fprintf(g.out, "%-8s not found (%v)\n", "game", err)
	}
	return nil
}
