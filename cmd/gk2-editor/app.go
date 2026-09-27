package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sextonworks/gk2-save-editor/internal/gamedata"
	"github.com/sextonworks/gk2-save-editor/internal/session"
)

type App struct {
	ctx context.Context
	s   *session.Session
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.s.Startup(ctx)
}

func (a *App) shutdown(context.Context) {
	a.s.Shutdown()
}

func (a *App) notify(event string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, event)
	}
}

func (a *App) Version() string                         { return version() }
func (a *App) Environment() session.Environment        { return a.s.Environment() }
func (a *App) State() session.State                    { return a.s.State() }
func (a *App) SetLang(lang string) session.State       { return a.s.SetLang(lang) }
func (a *App) Open(slot string) (session.State, error) { return a.s.Open(slot) }
func (a *App) Reload() (session.State, error)          { return a.s.Reload() }
func (a *App) Undo() (session.State, error)            { return a.s.Undo() }
func (a *App) Redo() (session.State, error)            { return a.s.Redo() }

func (a *App) UseFolders(saveDir, gameDir string) (session.Environment, error) {
	return a.s.UseFolders(saveDir, gameDir)
}

func (a *App) PickFolder(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}

func (a *App) Write() (session.WriteResult, error)    { return a.s.Write(a.ctx) }
func (a *App) Backups() ([]session.BackupView, error) { return a.s.Backups() }
func (a *App) Restore(name string) (session.State, error) {
	return a.s.Restore(a.ctx, name)
}

func (a *App) Container(container, lang string) (session.ContainerView, error) {
	return a.s.Container(container, lang)
}
func (a *App) Player(lang string) (session.Player, error)        { return a.s.Player(lang) }
func (a *App) Zombies(lang string) ([]session.ZombieView, error) { return a.s.Zombies(lang) }
func (a *App) Inspirations(lang string) ([]session.InspirationView, error) {
	return a.s.Inspirations(lang)
}
func (a *App) Techs(lang string) ([]session.TechView, error) { return a.s.Techs(lang) }
func (a *App) SearchItems(query, lang string, all bool) ([]gamedata.Entry, error) {
	return a.s.SearchItems(query, lang, all, 300)
}

func (a *App) SetMoney(v float64) (session.State, error) { return a.s.SetMoney(v) }
func (a *App) SetResource(t string, v float64) (session.State, error) {
	return a.s.SetResource(t, v)
}
func (a *App) SetTalentPoints(talent string, points int64) (session.State, error) {
	return a.s.SetTalentPoints(talent, points)
}
func (a *App) SetItemCount(container, uniqueID string, count int64) (session.State, error) {
	return a.s.SetItemCount(container, uniqueID, count)
}
func (a *App) RemoveItem(container, uniqueID string) (session.State, error) {
	return a.s.RemoveItem(container, uniqueID)
}
func (a *App) AddItem(container, id string, count int64) (session.State, error) {
	return a.s.AddItem(container, id, count)
}
func (a *App) SwapItem(container, uniqueID, newID string) (session.State, error) {
	return a.s.SwapItem(container, uniqueID, newID)
}
func (a *App) EquipBest() (session.State, error) { return a.s.EquipBest() }
func (a *App) MaxZombie(name string, tech, brains int64) (session.State, error) {
	return a.s.MaxZombie(name, tech, brains)
}
func (a *App) Inspire(talent, id string) (session.State, error) { return a.s.Inspire(talent, id) }

func (a *App) InspectChildren(offset, from, limit int) ([]session.NodeView, error) {
	return a.s.InspectChildren(offset, from, limit)
}
func (a *App) InspectSearch(query string) ([]session.NodeView, error) {
	return a.s.InspectSearch(query, 500)
}
