package locate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

const (
	AppID           = "4358690"
	GameFolder      = "Graveyard Keeper 2"
	ExeName         = "GraveyardKeeper2.exe"
	DataFolder      = "GraveyardKeeper2_Data"
	DefaultSlot     = "Steam_1"
	FallbackUnity   = "6000.3.9f1"
	envSaveDir      = "GK2_SAVE_DIR"
	envGameDir      = "GK2_GAME_DIR"
	crossoverBottle = "Library/Application Support/CrossOver/Bottles"
)

var (
	ErrNotFound = errors.New("not found")

	saveSubpath   = filepath.Join("AppData", "LocalLow", "Lazy Bear Games", "Graveyard Keeper 2")
	libraryPathRe = regexp.MustCompile(`"path"\s+"([^"]+)"`)
	unityVerRe    = regexp.MustCompile(`\d{4}\.\d+\.\d+[abfp]\d+`)
)

type Env struct {
	Home     string
	GOOS     string
	Getenv   func(string) string
	IsDir    func(string) bool
	ReadFile func(string) ([]byte, error)
	Glob     func(string) ([]string, error)
}

func System() Env {
	home, _ := os.UserHomeDir()
	return Env{
		Home:     home,
		GOOS:     runtime.GOOS,
		Getenv:   os.Getenv,
		IsDir:    func(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() },
		ReadFile: os.ReadFile,
		Glob:     filepath.Glob,
	}
}

func (e Env) crossoverDrives() []string {
	drives, err := e.Glob(filepath.Join(e.Home, crossoverBottle, "*", "drive_c"))
	if err != nil {
		return nil
	}
	slices.Sort(drives)
	return drives
}

func (e Env) steamRoots() []string {
	roots := make([]string, 0, 8)
	if e.GOOS == "windows" {
		for _, v := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
			if base := e.Getenv(v); base != "" {
				roots = append(roots, filepath.Join(base, "Steam"))
			}
		}
	}
	roots = append(roots,
		filepath.Join(e.Home, ".steam", "steam"),
		filepath.Join(e.Home, ".local", "share", "Steam"),
		filepath.Join(e.Home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(e.Home, "Library", "Application Support", "Steam"),
	)
	for _, d := range e.crossoverDrives() {
		roots = append(roots, filepath.Join(d, "Program Files (x86)", "Steam"))
	}
	return roots
}

func (e Env) libraries() []string {
	seen := map[string]bool{}
	libs := make([]string, 0, 8)
	add := func(p string) {
		key := filepath.Clean(p)
		if real, err := filepath.EvalSymlinks(p); err == nil {
			key = real
		}
		if seen[key] {
			return
		}
		seen[key] = true
		libs = append(libs, p)
	}
	for _, root := range e.steamRoots() {
		if !e.IsDir(root) {
			continue
		}
		add(root)
		data, err := e.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
		if err != nil {
			continue
		}
		for _, m := range libraryPathRe.FindAllStringSubmatch(string(data), -1) {
			add(filepath.FromSlash(strings.ReplaceAll(m[1], `\\`, `\`)))
		}
	}
	return libs
}

func (e Env) SaveDirCandidates() []string {
	out := make([]string, 0, 8)
	if e.GOOS == "windows" {
		if p := e.Getenv("USERPROFILE"); p != "" {
			out = append(out, filepath.Join(p, saveSubpath))
		}
	}
	for _, lib := range e.libraries() {
		out = append(out, filepath.Join(lib, "steamapps", "compatdata", AppID, "pfx", "drive_c", "users", "steamuser", saveSubpath))
	}
	for _, d := range e.crossoverDrives() {
		matches, err := e.Glob(filepath.Join(d, "users", "*", saveSubpath))
		if err != nil {
			continue
		}
		slices.Sort(matches)
		out = append(out, matches...)
	}
	return out
}

func (e Env) SaveDir(explicit string) (string, error) {
	if explicit == "" {
		explicit = e.Getenv(envSaveDir)
	}
	if explicit != "" {
		if !e.IsDir(explicit) {
			return "", fmt.Errorf("save folder %s: %w", explicit, ErrNotFound)
		}
		return explicit, nil
	}
	for _, p := range e.SaveDirCandidates() {
		if e.IsDir(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("save folder (pass --save-dir or set %s): %w", envSaveDir, ErrNotFound)
}

func (e Env) GameDir(explicit string) (string, error) {
	if explicit == "" {
		explicit = e.Getenv(envGameDir)
	}
	if explicit != "" {
		if !e.IsDir(filepath.Join(explicit, DataFolder)) {
			return "", fmt.Errorf("game folder %s: %w", explicit, ErrNotFound)
		}
		return explicit, nil
	}
	for _, lib := range e.libraries() {
		p := filepath.Join(lib, "steamapps", "common", GameFolder)
		if e.IsDir(filepath.Join(p, DataFolder)) {
			return p, nil
		}
	}
	return "", fmt.Errorf("game folder (pass --game-dir or set %s): %w", envGameDir, ErrNotFound)
}

func (e Env) UnityVersion(gameDir string) string {
	data, err := e.ReadFile(filepath.Join(gameDir, DataFolder, "globalgamemanagers"))
	if err != nil {
		return FallbackUnity
	}
	if m := unityVerRe.Find(data[:min(len(data), 4096)]); m != nil {
		return string(m)
	}
	return FallbackUnity
}
