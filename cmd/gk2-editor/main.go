package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/sextonworks/gk2-save-editor/internal/locate"
	"github.com/sextonworks/gk2-save-editor/internal/session"
	"github.com/sextonworks/gk2-save-editor/internal/store"
)

var revision = ""

//go:embed all:frontend/dist
var assets embed.FS

func version() string {
	if revision != "" {
		return revision
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				return s.Value[:7]
			}
		}
	}
	return "dev"
}

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

func run() error {
	app := &App{}
	app.s = session.New(session.Config{
		Env:       locate.System(),
		Running:   locate.GameRunning,
		BackupDir: store.DefaultBackupDir(),
		CacheDir:  cacheDir(),
		Notify:    app.notify,
	})
	return wails.Run(&options.App{
		Title:            "GK2 Save Editor",
		Width:            1180,
		Height:           780,
		MinWidth:         900,
		MinHeight:        600,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 22, G: 27, B: 32, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
	})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
