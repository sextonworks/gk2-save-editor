package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/sextonworks/gk2-save-editor/internal/locate"
	"github.com/sextonworks/gk2-save-editor/internal/save"
)

var revision = ""

type globals struct {
	ctx     context.Context
	out     io.Writer
	env     locate.Env
	running func(context.Context) (bool, error)
}

type cli struct {
	Slot    string           `short:"s" default:"Steam_1" help:"Save slot name."`
	Save    string           `help:"Path to a .dat save file (overrides --slot)." type:"path"`
	SaveDir string           `help:"Save folder (default: auto, or GK2_SAVE_DIR)." type:"path"`
	GameDir string           `help:"Game folder (default: auto, or GK2_GAME_DIR)." type:"path"`
	Version kong.VersionFlag `help:"Show version and exit."`

	Info    infoCmd    `cmd:"" help:"Save summary: day, money, backpack, zombies, game status."`
	Bag     bagCmd     `cmd:"" help:"List the backpack."`
	Belt    beltCmd    `cmd:"" help:"List the tool belt (tools, weapon, armor)."`
	Res     resCmd     `cmd:"" help:"Show player resources (money, energy, tech points, reputation...)."`
	Talents talentsCmd `cmd:"" help:"Show talent branches: level, exp, free points."`
	Zombies zombiesCmd `cmd:"" help:"List the player's zombies."`
	Paths   pathsCmd   `cmd:"" help:"Show the folders gk2 uses."`
}

type jsonFlag struct {
	JSON bool `name:"json" help:"Print JSON."`
}

func (c *cli) savePath(g *globals) (string, error) {
	if c.Save != "" {
		return c.Save, nil
	}
	dir, err := g.env.SaveDir(c.SaveDir)
	if err != nil {
		return "", fmt.Errorf("find save: %w", err)
	}
	return filepath.Join(dir, c.Slot+".dat"), nil
}

func (c *cli) open(g *globals) (*save.Save, error) {
	path, err := c.savePath(g)
	if err != nil {
		return nil, err
	}
	s, err := save.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return s, nil
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
	return emit(g, j, items, func(w io.Writer) {
		for _, it := range items {
			fmt.Fprintf(w, "%6d  %s\n", it.Count, it.ID)
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

func version() string {
	if revision != "" {
		return revision
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			return bi.Main.Version
		}
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				return s.Value[:7]
			}
		}
	}
	return "unknown"
}

func run(ctx context.Context, args []string, out io.Writer, g globals) error {
	var c cli
	parser, err := kong.New(&c,
		kong.Name("gk2"),
		kong.Description("Save editor for Graveyard Keeper 2. Every write makes a backup first."),
		kong.Vars{"version": "gk2 " + version()},
		kong.Writers(out, out),
		kong.Exit(func(int) { panic(exitSignal{}) }),
		kong.UsageOnError(),
	)
	if err != nil {
		return fmt.Errorf("build cli: %w", err)
	}
	kctx, err := parseArgs(parser, args)
	if err != nil {
		return err
	}
	if kctx == nil {
		return nil
	}
	g.ctx, g.out = ctx, out
	return kctx.Run(&c, &g)
}

type exitSignal struct{}

func parseArgs(parser *kong.Kong, args []string) (kctx *kong.Context, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(exitSignal); !ok {
				panic(r)
			}
			kctx, err = nil, nil
		}
	}()
	if len(args) == 0 {
		args = []string{"--help"}
	}
	kctx, err = parser.Parse(args)
	if err != nil {
		return nil, fmt.Errorf("parse arguments: %w", err)
	}
	return kctx, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout, globals{env: locate.System(), running: locate.GameRunning})
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
