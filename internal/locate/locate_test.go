package locate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEnv(t *testing.T, goos string, vars map[string]string) (Env, string) {
	t.Helper()
	home := t.TempDir()
	e := System()
	e.Home = home
	e.GOOS = goos
	e.Getenv = func(k string) string { return vars[k] }
	return e, home
}

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(p, 0o700))
	return p
}

func TestSaveDirLayouts(t *testing.T) {
	tests := []struct {
		name  string
		goos  string
		setup func(t *testing.T, home string, vars map[string]string) string
	}{
		{"windows profile", "windows", func(t *testing.T, home string, vars map[string]string) string {
			vars["USERPROFILE"] = home
			return mkdir(t, home, saveSubpath)
		}},
		{"proton", "linux", func(t *testing.T, home string, _ map[string]string) string {
			return mkdir(t, home, ".steam", "steam", "steamapps", "compatdata", AppID, "pfx", "drive_c", "users", "steamuser", saveSubpath)
		}},
		{"proton on extra library", "linux", func(t *testing.T, home string, _ map[string]string) string {
			root := mkdir(t, home, ".local", "share", "Steam", "steamapps")
			lib := filepath.Join(home, "games")
			vdf := `"libraryfolders" { "1" { "path" "` + lib + `" } }`
			require.NoError(t, os.WriteFile(filepath.Join(root, "libraryfolders.vdf"), []byte(vdf), 0o600))
			return mkdir(t, lib, "steamapps", "compatdata", AppID, "pfx", "drive_c", "users", "steamuser", saveSubpath)
		}},
		{"crossover", "darwin", func(t *testing.T, home string, _ map[string]string) string {
			return mkdir(t, home, crossoverBottle, "Steam", "drive_c", "users", "crossover", saveSubpath)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := map[string]string{}
			e, home := testEnv(t, tt.goos, vars)
			want := tt.setup(t, home, vars)
			got, err := e.SaveDir("")
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestSaveDirOverrides(t *testing.T) {
	e, home := testEnv(t, "linux", map[string]string{envSaveDir: "/nonexistent-gk2"})
	_, err := e.SaveDir("")
	require.ErrorIs(t, err, ErrNotFound)
	got, err := e.SaveDir(home)
	require.NoError(t, err)
	assert.Equal(t, home, got)
	e.Getenv = func(string) string { return "" }
	_, err = e.SaveDir("")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestGameDir(t *testing.T) {
	e, home := testEnv(t, "linux", map[string]string{})
	_, err := e.GameDir("")
	require.ErrorIs(t, err, ErrNotFound)
	game := filepath.Join(home, ".steam", "steam", "steamapps", "common", GameFolder)
	mkdir(t, game, DataFolder)
	got, err := e.GameDir("")
	require.NoError(t, err)
	assert.Equal(t, game, got)
	_, err = e.GameDir(home)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestUnityVersion(t *testing.T) {
	e, home := testEnv(t, "linux", nil)
	assert.Equal(t, FallbackUnity, e.UnityVersion(home))
	data := mkdir(t, home, DataFolder)
	require.NoError(t, os.WriteFile(filepath.Join(data, "globalgamemanagers"), []byte("\x00\x00 2022.3.62f1\x00"), 0o600))
	assert.Equal(t, "2022.3.62f1", e.UnityVersion(home))
}
