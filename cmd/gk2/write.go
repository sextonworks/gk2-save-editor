package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sextonworks/gk2-save-editor/internal/save"
	"github.com/sextonworks/gk2-save-editor/internal/store"
)

var (
	errGameRunning = errors.New("the game is running: save, quit to desktop and try again, or pass --wait")
	errBadSpec     = errors.New("bad item spec")
)

const (
	pollInterval = 3 * time.Second
	settleDelay  = 5 * time.Second
)

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *cli) waitForGame(g *globals) error {
	waited := false
	for {
		if err := g.ctx.Err(); err != nil {
			return err
		}
		running, err := g.running(g.ctx)
		if err != nil {
			return fmt.Errorf("check game process: %w", err)
		}
		if !running {
			break
		}
		if !c.Wait {
			return errGameRunning
		}
		if !waited {
			fmt.Fprintln(g.out, "Waiting for the game to close...")
			waited = true
		}
		if err := g.sleep(g.ctx, pollInterval); err != nil {
			return err
		}
	}
	if waited {
		return g.sleep(g.ctx, settleDelay)
	}
	return nil
}

func (c *cli) edit(g *globals, fn func(e *save.Editor) error) error {
	path, err := c.savePath(g)
	if err != nil {
		return err
	}
	if !c.DryRun && c.Save == "" {
		if err := c.waitForGame(g); err != nil {
			return err
		}
	}
	e, err := save.OpenEditor(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	if err := fn(e); err != nil {
		return err
	}
	changes := e.Changes()
	for _, ch := range changes {
		fmt.Fprintf(g.out, "  %s\n", ch)
	}
	if len(changes) == 0 {
		fmt.Fprintln(g.out, "Nothing to change.")
		return nil
	}
	if c.DryRun {
		fmt.Fprintln(g.out, "Dry run, nothing written.")
		return nil
	}
	st := store.New(c.backupDir())
	st.Warn = func(err error) { fmt.Fprintln(g.out, "warning:", err) }
	b, err := st.Create(path)
	if err != nil {
		return fmt.Errorf("backup before writing: %w", err)
	}
	if err := store.WriteAtomic(path, e.Save().Bytes(), e.OriginalHash()); err != nil {
		return fmt.Errorf("write save: %w", err)
	}
	fmt.Fprintf(g.out, "Written. Backup: %s\n", b.Path)
	return nil
}

func (c *cli) backupDir() string {
	if c.BackupDir != "" {
		return c.BackupDir
	}
	return store.DefaultBackupDir()
}

func (c *cli) editStacks(g *globals, cmd string, where save.Container, ids []string, all bool, op func(save.Stack) save.Op) error {
	return c.edit(g, func(e *save.Editor) error {
		for _, id := range ids {
			found, err := stacksOf(e, where, id, all, g)
			if err != nil {
				return fmt.Errorf("%s: %w", cmd, err)
			}
			for _, st := range found {
				if err := e.Apply(op(st)); err != nil {
					return fmt.Errorf("%s: %w", cmd, err)
				}
			}
		}
		return nil
	})
}

type whereFlag struct {
	Where string `default:"bag" enum:"bag,belt" help:"Container: bag or belt."`
}

type allFlag struct {
	All bool `help:"Apply to every stack with this id."`
}

func stacksOf(e *save.Editor, c save.Container, id string, all bool, g *globals) ([]save.Stack, error) {
	items, err := e.Save().Items(c)
	if err != nil {
		return nil, err
	}
	found := make([]save.Stack, 0, 2)
	for _, it := range items {
		if it.ID == id {
			found = append(found, it)
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("%q in %s: %w", id, c, save.ErrNoItem)
	}
	if len(found) > 1 && !all {
		fmt.Fprintf(g.out, "Note: %d stacks of %s, changing the first one (--all for every stack).\n", len(found), id)
		return found[:1], nil
	}
	return found, nil
}

type checkFlag struct {
	Check bool `default:"true" negatable:"" help:"Refuse ids that are not in the game data (needs the game folder)."`
}

type addCmd struct {
	whereFlag
	checkFlag
	Items []string `arg:"" help:"Item ids, optionally id=count (candle_basic=5)."`
}

func parseSpec(spec string) (string, int64, error) {
	id, n, found := strings.Cut(spec, "=")
	if id == "" {
		return "", 0, fmt.Errorf("%q: %w", spec, errBadSpec)
	}
	if !found {
		return id, 1, nil
	}
	count, err := strconv.ParseInt(n, 10, 64)
	if err != nil || count < 1 {
		return "", 0, fmt.Errorf("%q: count must be a positive number: %w", spec, errBadSpec)
	}
	return id, count, nil
}

func (a *addCmd) Run(c *cli, g *globals) error {
	ops := make([]save.Op, 0, len(a.Items))
	ids := make([]string, 0, len(a.Items))
	for _, spec := range a.Items {
		id, n, err := parseSpec(spec)
		if err != nil {
			return err
		}
		ids = append(ids, id)
		ops = append(ops, save.NewAddItem(save.Container(a.Where), id, n))
	}
	if a.Check {
		if err := c.requireKnown(g, ids...); err != nil {
			return fmt.Errorf("add: %w", err)
		}
	}
	return c.edit(g, func(e *save.Editor) error {
		for _, op := range ops {
			if err := e.Apply(op); err != nil {
				return fmt.Errorf("add: %w", err)
			}
		}
		return nil
	})
}

type countCmd struct {
	whereFlag
	allFlag
	Item  string `arg:"" help:"Item id."`
	Value int64  `arg:"" help:"New stack size."`
}

func (a *countCmd) Run(c *cli, g *globals) error {
	return c.editStacks(g, "count", save.Container(a.Where), []string{a.Item}, a.All, func(st save.Stack) save.Op {
		return save.SetItemCount{Container: save.Container(a.Where), UniqueID: st.UniqueID, Count: a.Value}
	})
}

type removeCmd struct {
	whereFlag
	allFlag
	Items []string `arg:"" help:"Item ids."`
}

func (a *removeCmd) Run(c *cli, g *globals) error {
	return c.editStacks(g, "remove", save.Container(a.Where), a.Items, a.All, func(st save.Stack) save.Op {
		return save.RemoveItem{Container: save.Container(a.Where), UniqueID: st.UniqueID}
	})
}

type swapCmd struct {
	whereFlag
	allFlag
	checkFlag
	Old string `arg:"" help:"Current item id."`
	New string `arg:"" help:"New item id."`
}

func (a *swapCmd) Run(c *cli, g *globals) error {
	if a.Check {
		if err := c.requireKnown(g, a.New); err != nil {
			return fmt.Errorf("swap: %w", err)
		}
	}
	return c.editStacks(g, "swap", save.Container(a.Where), []string{a.Old}, a.All, func(st save.Stack) save.Op {
		return save.SetItemID{Container: save.Container(a.Where), UniqueID: st.UniqueID, ItemID: a.New}
	})
}

type moneyCmd struct {
	Value *float64 `arg:"" optional:"" help:"New amount in copper (100 copper = 1 silver). Stay below 16777216."`
}

func (a *moneyCmd) Run(c *cli, g *globals) error {
	if a.Value == nil {
		s, err := c.open(g)
		if err != nil {
			return err
		}
		v, err := s.Resource("money")
		if err != nil {
			return fmt.Errorf("money: %w", err)
		}
		fmt.Fprintln(g.out, number(v))
		return nil
	}
	return c.edit(g, func(e *save.Editor) error {
		if err := e.Apply(save.SetResource{Type: "money", Value: *a.Value}); err != nil {
			return fmt.Errorf("money: %w", err)
		}
		return nil
	})
}

type setResCmd struct {
	Type  string  `arg:"" help:"Resource type, e.g. tech_red."`
	Value float64 `arg:"" help:"New value."`
}

func (a *setResCmd) Run(c *cli, g *globals) error {
	return c.edit(g, func(e *save.Editor) error {
		if err := e.Apply(save.SetResource{Type: a.Type, Value: a.Value}); err != nil {
			return fmt.Errorf("set-res: %w", err)
		}
		return nil
	})
}

type setTalentsCmd struct {
	Points int64  `arg:"" help:"Free talent points."`
	Branch string `help:"Only this branch, e.g. talent_red."`
}

func (a *setTalentsCmd) Run(c *cli, g *globals) error {
	return c.edit(g, func(e *save.Editor) error {
		talents, err := e.Save().Talents()
		if err != nil {
			return fmt.Errorf("set-talents: %w", err)
		}
		for _, t := range talents {
			if a.Branch != "" && t.ID != a.Branch {
				continue
			}
			if err := e.Apply(save.SetTalentPoints{Talent: t.ID, Points: a.Points}); err != nil {
				return fmt.Errorf("set-talents: %w", err)
			}
		}
		return nil
	})
}

type zombiesMaxCmd struct {
	Tech   int64 `default:"999999" help:"Tech points of each colour."`
	Brains int64 `default:"328" help:"Brain count, 3 red skulls each."`
}

func (a *zombiesMaxCmd) Run(c *cli, g *globals) error {
	return c.edit(g, func(e *save.Editor) error {
		zombies, err := e.Save().Zombies()
		if err != nil {
			return fmt.Errorf("zombies-max: %w", err)
		}
		for _, z := range zombies {
			if err := e.Apply(save.MaxZombie{Name: z.Name, Tech: a.Tech, Brains: a.Brains}); err != nil {
				return fmt.Errorf("zombies-max: %w", err)
			}
		}
		return nil
	})
}

type equipBestCmd struct{}

func (a *equipBestCmd) Run(c *cli, g *globals) error {
	return c.edit(g, func(e *save.Editor) error {
		if err := e.Apply(save.EquipBest{}); err != nil {
			return fmt.Errorf("equip-best: %w", err)
		}
		return nil
	})
}

type backupsCmd struct {
	jsonFlag
}

func (a *backupsCmd) Run(c *cli, g *globals) error {
	st := store.New(c.backupDir())
	list, err := st.List()
	if err != nil {
		return fmt.Errorf("backups: %w", err)
	}
	return emit(g, a.jsonFlag, list, func(w io.Writer) {
		fmt.Fprintf(w, "# %s\n", st.Dir)
		for _, b := range list {
			legacy := ""
			if b.Legacy {
				legacy = "  (folder from the Python version)"
			}
			fmt.Fprintf(w, "%s  %s%s\n", b.Name, strings.Join(b.Files, ", "), legacy)
		}
	})
}

type restoreCmd struct {
	Name string `arg:"" help:"Backup name from 'gk2 backups'."`
}

func (a *restoreCmd) Run(c *cli, g *globals) error {
	path, err := c.savePath(g)
	if err != nil {
		return err
	}
	if c.DryRun {
		fmt.Fprintf(g.out, "Would restore %s into %s\n", a.Name, filepath.Dir(path))
		return nil
	}
	if c.Save == "" {
		if err := c.waitForGame(g); err != nil {
			return err
		}
	}
	st := store.New(c.backupDir())
	st.Warn = func(err error) { fmt.Fprintln(g.out, "warning:", err) }
	current, err := st.Create(path)
	if err != nil {
		return fmt.Errorf("backup current save: %w", err)
	}
	fmt.Fprintf(g.out, "Backup of the current save: %s\n", current.Path)
	restored, err := st.Restore(a.Name, filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("restore %s: %w", a.Name, err)
	}
	fmt.Fprintf(g.out, "Restored %s: %s\n", a.Name, strings.Join(restored, ", "))
	return nil
}
