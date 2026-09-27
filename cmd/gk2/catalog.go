package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sextonworks/gk2-save-editor/internal/gamedata"
)

var errUnknownID = errors.New("not a known id")

func cacheDir() string {
	if v := os.Getenv("GK2_CACHE_DIR"); v != "" {
		return v
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "gk2-cache")
	}
	return filepath.Join(base, "gk2")
}

func (c *cli) catalog(g *globals, refresh bool) (*gamedata.Catalog, error) {
	if g.catalog != nil && !refresh {
		return g.catalog, nil
	}
	dir, err := g.env.GameDir(c.GameDir)
	if err != nil {
		return nil, fmt.Errorf("find game: %w", err)
	}
	cat, err := gamedata.Load(dir, c.cacheDir(), refresh)
	if err != nil {
		return nil, fmt.Errorf("read game data: %w", err)
	}
	g.catalog = cat
	return cat, nil
}

func (c *cli) cacheDir() string {
	if c.CacheDir != "" {
		return c.CacheDir
	}
	return cacheDir()
}

func (c *cli) requireKnown(g *globals, ids ...string) error {
	cat, err := c.catalog(g, false)
	if err != nil {
		return fmt.Errorf("%w (pass --no-check to skip the id check)", err)
	}
	for _, id := range ids {
		if !cat.Known(id) {
			return fmt.Errorf("%q: %w, try: gk2 find %s", id, errUnknownID, id)
		}
	}
	return nil
}

type findCmd struct {
	jsonFlag
	Query   string `arg:"" help:"Part of an id or of a name in the chosen language."`
	All     bool   `help:"Search every id (techs, buildings, crafts), not only items."`
	Refresh bool   `help:"Re-read the game data."`
	Limit   int    `default:"200" help:"Maximum number of results."`
}

func (a *findCmd) Run(c *cli, g *globals) error {
	cat, err := c.catalog(g, a.Refresh)
	if err != nil {
		return err
	}
	hits := cat.Search(a.Query, c.Lang, !a.All, a.Limit)
	return emit(g, a.jsonFlag, hits, func(w io.Writer) {
		for _, h := range hits {
			if h.Name != h.ID {
				fmt.Fprintf(w, "%-32s %s\n", h.ID, h.Name)
			} else {
				fmt.Fprintln(w, h.ID)
			}
		}
		if len(hits) == 0 {
			fmt.Fprintln(w, "Nothing found.")
		}
	})
}
