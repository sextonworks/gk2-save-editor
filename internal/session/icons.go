package session

import (
	"context"
	"errors"

	"github.com/sextonworks/gk2-save-editor/internal/gamedata"
)

var ErrNoIcons = errors.New("icons are not ready")

func (s *Session) prepareIcons(ctx context.Context) {
	cat := s.catalog
	if cat == nil || s.cfg.CacheDir == "" || s.iconsFor == cat.Source {
		return
	}
	s.iconsFor = cat.Source
	gameDir, cacheDir := s.gameDir, s.cfg.CacheDir
	go func() {
		err := cat.EnsureIcons(ctx, gameDir, cacheDir)
		s.mu.Lock()
		if err == nil && s.iconsFor == cat.Source {
			s.iconDir = cat.IconDir
		}
		s.mu.Unlock()
		if err == nil {
			s.cfg.Notify(EventState)
		}
	}()
}

func (s *Session) IconPath(name string) (string, error) {
	s.mu.Lock()
	dir := s.iconDir
	s.mu.Unlock()
	if dir == "" {
		return "", ErrNoIcons
	}
	return gamedata.IconPath(dir, name)
}
