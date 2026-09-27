package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sextonworks/gk2-save-editor/internal/gamedata"
	"github.com/sextonworks/gk2-save-editor/internal/locate"
	"github.com/sextonworks/gk2-save-editor/internal/save"
	"github.com/sextonworks/gk2-save-editor/internal/store"
)

var (
	ErrNoSave      = errors.New("no save is open")
	ErrGameRunning = errors.New("the game is running: save, quit to desktop and try again")
	ErrConflict    = errors.New("the save was changed outside the editor: reload it before saving")
	ErrNoCatalog   = errors.New("game data is not available")
)

const (
	EventState = "state"
	pollEvery  = 2 * time.Second
)

type Config struct {
	Env       locate.Env
	Running   func(context.Context) (bool, error)
	BackupDir string
	CacheDir  string
	Notify    func(event string)
	PollEvery time.Duration
}

type Slot struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Day      int    `json:"day"`
	SavedAt  string `json:"savedAt"`
	Modified string `json:"modified"`
}

type Environment struct {
	SaveDir      string `json:"saveDir"`
	SaveDirError string `json:"saveDirError,omitempty"`
	GameDir      string `json:"gameDir"`
	GameDirError string `json:"gameDirError,omitempty"`
	CatalogError string `json:"catalogError,omitempty"`
	BackupDir    string `json:"backupDir"`
	Slots        []Slot `json:"slots"`
}

type State struct {
	Open        bool      `json:"open"`
	Slot        string    `json:"slot"`
	Path        string    `json:"path"`
	Info        save.Info `json:"info"`
	Dirty       bool      `json:"dirty"`
	CanUndo     bool      `json:"canUndo"`
	CanRedo     bool      `json:"canRedo"`
	Changes     []Change  `json:"changes"`
	GameRunning bool      `json:"gameRunning"`
	Conflict    bool      `json:"conflict"`
}

type Session struct {
	cfg Config

	mu       sync.Mutex
	saveDir  string
	gameDir  string
	env      Environment
	catalog  *gamedata.Catalog
	editor   *save.Editor
	slot     string
	running  bool
	conflict bool
	lang     string
	stop     context.CancelFunc
	done     chan struct{}
}

func New(cfg Config) *Session {
	if cfg.Notify == nil {
		cfg.Notify = func(string) {}
	}
	if cfg.PollEvery == 0 {
		cfg.PollEvery = pollEvery
	}
	return &Session{cfg: cfg}
}

func (s *Session) Startup(ctx context.Context) {
	s.mu.Lock()
	s.discover()
	s.mu.Unlock()
	wctx, cancel := context.WithCancel(ctx)
	s.stop, s.done = cancel, make(chan struct{})
	go s.watch(wctx)
}

func (s *Session) Shutdown() {
	if s.stop != nil {
		s.stop()
		<-s.done
	}
}

func (s *Session) discover() {
	s.env = Environment{BackupDir: s.cfg.BackupDir, Slots: []Slot{}}
	if dir, err := s.cfg.Env.SaveDir(""); err == nil {
		s.saveDir, s.env.SaveDir = dir, dir
	} else {
		s.env.SaveDirError = err.Error()
	}
	if dir, err := s.cfg.Env.GameDir(""); err == nil {
		s.gameDir, s.env.GameDir = dir, dir
		if cat, err := gamedata.Load(dir, s.cfg.CacheDir, false); err == nil {
			s.catalog = cat
		} else {
			s.env.CatalogError = err.Error()
		}
	} else {
		s.env.GameDirError = err.Error()
	}
	s.env.Slots = s.slots()
}

func (s *Session) slots() []Slot {
	if s.saveDir == "" {
		return []Slot{}
	}
	matches, err := filepath.Glob(filepath.Join(s.saveDir, "*.dat"))
	if err != nil {
		return []Slot{}
	}
	out := make([]Slot, 0, len(matches))
	for _, p := range matches {
		name := strings.TrimSuffix(filepath.Base(p), ".dat")
		if strings.Contains(name, "_backup_") {
			continue
		}
		sl := Slot{Name: name, Path: p}
		if st, err := os.Stat(p); err == nil {
			sl.Modified = st.ModTime().Format(time.RFC3339)
		}
		if sv, err := save.Open(p); err == nil {
			if info, err := sv.Info(); err == nil {
				sl.Day, sl.SavedAt = info.Day, info.SaveDateTime
			}
		}
		out = append(out, sl)
	}
	slices.SortFunc(out, func(a, b Slot) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func (s *Session) SetLang(lang string) State {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lang = lang
	return s.stateLocked()
}

func (s *Session) Environment() Environment {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discover()
	return s.env
}

func (s *Session) UseFolders(saveDir, gameDir string) (Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if saveDir != "" {
		if _, err := s.cfg.Env.SaveDir(saveDir); err != nil {
			return s.env, err
		}
	}
	if gameDir != "" {
		if _, err := s.cfg.Env.GameDir(gameDir); err != nil {
			return s.env, err
		}
	}
	env := s.cfg.Env
	get := env.Getenv
	env.Getenv = func(k string) string {
		switch {
		case k == "GK2_SAVE_DIR" && saveDir != "":
			return saveDir
		case k == "GK2_GAME_DIR" && gameDir != "":
			return gameDir
		}
		return get(k)
	}
	s.cfg.Env = env
	s.catalog = nil
	s.discover()
	return s.env, nil
}

func (s *Session) stateLocked() State {
	st := State{GameRunning: s.running, Conflict: s.conflict, Changes: []Change{}}
	if s.editor == nil {
		return st
	}
	st.Open, st.Slot, st.Path = true, s.slot, s.editor.Save().Path()
	if info, err := s.editor.Save().Info(); err == nil {
		st.Info = info
	}
	st.Dirty, st.CanUndo, st.CanRedo = s.editor.Dirty(), s.editor.CanUndo(), s.editor.CanRedo()
	st.Changes = s.changes()
	return st
}

func (s *Session) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateLocked()
}

func (s *Session) Open(slot string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveDir == "" {
		return s.stateLocked(), fmt.Errorf("open %s: %w", slot, locate.ErrNotFound)
	}
	if slot != filepath.Base(slot) {
		return s.stateLocked(), fmt.Errorf("open %s: %w", slot, ErrNoSave)
	}
	e, err := save.OpenEditor(filepath.Join(s.saveDir, slot+".dat"))
	if err != nil {
		return s.stateLocked(), fmt.Errorf("open %s: %w", slot, err)
	}
	s.editor, s.slot, s.conflict = e, slot, false
	return s.stateLocked(), nil
}

func (s *Session) edit(ops ...save.Op) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.editor == nil {
		return s.stateLocked(), ErrNoSave
	}
	for _, op := range ops {
		if err := s.editor.Apply(op); err != nil {
			return s.stateLocked(), err
		}
	}
	return s.stateLocked(), nil
}

func (s *Session) Undo() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.editor == nil {
		return s.stateLocked(), ErrNoSave
	}
	err := s.editor.Undo()
	return s.stateLocked(), err
}

func (s *Session) Redo() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.editor == nil {
		return s.stateLocked(), ErrNoSave
	}
	err := s.editor.Redo()
	return s.stateLocked(), err
}

func (s *Session) Reload() (State, error) {
	s.mu.Lock()
	slot := s.slot
	s.mu.Unlock()
	if slot == "" {
		return s.State(), ErrNoSave
	}
	return s.Open(slot)
}

type WriteResult struct {
	Backup string `json:"backup"`
	State  State  `json:"state"`
}

func (s *Session) Write(ctx context.Context) (WriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.editor == nil {
		return WriteResult{State: s.stateLocked()}, ErrNoSave
	}
	if s.conflict {
		return WriteResult{State: s.stateLocked()}, ErrConflict
	}
	if running, err := s.cfg.Running(ctx); err != nil {
		return WriteResult{State: s.stateLocked()}, fmt.Errorf("check game process: %w", err)
	} else if running {
		s.running = true
		return WriteResult{State: s.stateLocked()}, ErrGameRunning
	}
	if !s.editor.Dirty() {
		return WriteResult{State: s.stateLocked()}, nil
	}
	st := store.New(s.cfg.BackupDir)
	path := s.editor.Save().Path()
	b, err := st.Create(path)
	if err != nil {
		return WriteResult{State: s.stateLocked()}, fmt.Errorf("backup before writing: %w", err)
	}
	data := s.editor.Save().Bytes()
	if err := store.WriteAtomic(path, data, s.editor.OriginalHash()); err != nil {
		if errors.Is(err, store.ErrChangedOnDisk) {
			s.conflict = true
		}
		return WriteResult{Backup: b.Path, State: s.stateLocked()}, fmt.Errorf("write save: %w", err)
	}
	s.editor.MarkSaved(data)
	return WriteResult{Backup: b.Path, State: s.stateLocked()}, nil
}

type BackupView struct {
	Name    string   `json:"name"`
	Created string   `json:"created"`
	Files   []string `json:"files"`
	Legacy  bool     `json:"legacy"`
}

func (s *Session) Backups() ([]BackupView, error) {
	list, err := store.New(s.cfg.BackupDir).List()
	if err != nil {
		return nil, err
	}
	out := make([]BackupView, 0, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		b := list[i]
		out = append(out, BackupView{Name: b.Name, Created: b.Created.Format(time.RFC3339), Files: b.Files, Legacy: b.Legacy})
	}
	return out, nil
}

func (s *Session) Restore(ctx context.Context, name string) (State, error) {
	s.mu.Lock()
	slot, saveDir := s.slot, s.saveDir
	s.mu.Unlock()
	if saveDir == "" {
		return s.State(), locate.ErrNotFound
	}
	if running, err := s.cfg.Running(ctx); err != nil {
		return s.State(), fmt.Errorf("check game process: %w", err)
	} else if running {
		return s.State(), ErrGameRunning
	}
	st := store.New(s.cfg.BackupDir)
	if slot != "" {
		if _, err := st.Create(filepath.Join(saveDir, slot+".dat")); err != nil {
			return s.State(), fmt.Errorf("backup current save: %w", err)
		}
	}
	if _, err := st.Restore(name, saveDir); err != nil {
		return s.State(), err
	}
	if slot == "" {
		return s.State(), nil
	}
	return s.Open(slot)
}

func (s *Session) watch(ctx context.Context) {
	defer close(s.done)
	t := time.NewTicker(s.cfg.PollEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.poll(ctx) {
				s.cfg.Notify(EventState)
			}
		}
	}
}

func (s *Session) poll(ctx context.Context) bool {
	running, err := s.cfg.Running(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	if err == nil && running != s.running {
		s.running, changed = running, true
	}
	if s.editor != nil && !s.conflict {
		data, err := os.ReadFile(s.editor.Save().Path())
		if err == nil && store.Hash(data) != s.editor.OriginalHash() {
			s.conflict, changed = true, true
		}
	}
	return changed
}
