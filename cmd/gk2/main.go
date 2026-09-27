package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/alecthomas/kong"

	"github.com/sextonworks/gk2-save-editor/internal/gamedata"
	"github.com/sextonworks/gk2-save-editor/internal/locate"
	"github.com/sextonworks/gk2-save-editor/internal/save"
)

var revision = ""

type globals struct {
	ctx     context.Context
	out     io.Writer
	env     locate.Env
	running func(context.Context) (bool, error)
	sleep   func(context.Context, time.Duration) error
	catalog *gamedata.Catalog
}

type cli struct {
	Slot      string           `short:"s" default:"Steam_1" help:"Save slot name."`
	Save      string           `help:"Path to a .dat save file (overrides --slot)." type:"path"`
	SaveDir   string           `help:"Save folder (default: auto, or GK2_SAVE_DIR)." type:"path"`
	GameDir   string           `help:"Game folder (default: auto, or GK2_GAME_DIR)." type:"path"`
	DryRun    bool             `short:"n" help:"Show changes without writing."`
	Wait      bool             `short:"w" help:"Wait until the game is closed before editing."`
	BackupDir string           `help:"Backup folder (default: user data folder, or GK2_DATA_DIR/backups)." type:"path"`
	CacheDir  string           `help:"Cache folder for game data (default: user cache folder, or GK2_CACHE_DIR)." type:"path"`
	Lang      string           `default:"en" enum:"en,ru,zh_cn" help:"Language for item names: en, ru, zh_cn."`
	Version   kong.VersionFlag `help:"Show version and exit."`

	Info    infoCmd    `cmd:"" help:"Save summary: day, money, backpack, zombies, game status."`
	Bag     bagCmd     `cmd:"" help:"List the backpack."`
	Belt    beltCmd    `cmd:"" help:"List the tool belt (tools, weapon, armor)."`
	Res     resCmd     `cmd:"" help:"Show player resources (money, energy, tech points, reputation...)."`
	Talents talentsCmd `cmd:"" help:"Show talent branches: level, exp, free points."`
	Zombies zombiesCmd `cmd:"" help:"List the player's zombies."`
	Paths   pathsCmd   `cmd:"" help:"Show the folders gk2 uses."`
	Find    findCmd    `cmd:"" help:"Search items (or every id with --all) by id or name."`

	Inspirations inspirationsCmd `cmd:"" help:"Show inspiration progress per talent branch."`
	Techs        techsCmd        `cmd:"" help:"Show technologies: unlocked, available, hidden."`
	Inspire      inspireCmd      `cmd:"" help:"Bring inspirations to their goal so they can be bought in the game."`

	Add        addCmd        `cmd:"" help:"Add new stacks: gk2 add candle_basic=5 heal_potion"`
	Count      countCmd      `cmd:"" help:"Set the count of an existing stack."`
	Remove     removeCmd     `cmd:"" help:"Remove stacks."`
	Swap       swapCmd       `cmd:"" help:"Replace an item id with another one in place, keeping the count."`
	Money      moneyCmd      `cmd:"" help:"Show or set money in copper (100 copper = 1 silver)."`
	SetRes     setResCmd     `cmd:"" name:"set-res" help:"Set a player resource: gk2 set-res tech_red 999"`
	SetTalents setTalentsCmd `cmd:"" name:"set-talents" help:"Set free talent points in every branch (or one with --branch)."`
	ZombiesMax zombiesMaxCmd `cmd:"" name:"zombies-max" help:"Best body parts, max skill slots and tech points for every zombie."`
	EquipBest  equipBestCmd  `cmd:"" name:"equip-best" help:"Top tier on every tool belt slot that already holds an item."`
	Backups    backupsCmd    `cmd:"" help:"List backups."`
	Restore    restoreCmd    `cmd:"" help:"Restore a backup. The current save is backed up first."`
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
	err := run(ctx, os.Args[1:], os.Stdout, globals{env: locate.System(), running: locate.GameRunning, sleep: sleep})
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
