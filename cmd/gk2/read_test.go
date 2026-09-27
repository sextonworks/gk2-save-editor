package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/locate"
	"github.com/sextonworks/gk2-save-editor/internal/odin/odintest"
	"github.com/sextonworks/gk2-save-editor/internal/save"
)

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Steam_1.dat"), odintest.BuildSave(odintest.DefaultSpec()), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Steam_1.info"), []byte(`{"day": 7, "gameSaveVersion": "1.006"}`), 0o600))
	return dir
}

func invoke(t *testing.T, running bool, args ...string) (string, error) {
	t.Helper()
	if len(args) > 0 {
		args = append([]string{"--game-dir", t.TempDir(), "--cache-dir", t.TempDir()}, args...)
	}
	var out bytes.Buffer
	g := globals{
		env:     locate.System(),
		running: func(context.Context) (bool, error) { return running, nil },
	}
	err := run(context.Background(), args, &out, g)
	return out.String(), err
}

func TestReadCommands(t *testing.T) {
	dir := fixture(t)
	tests := []struct {
		name    string
		args    []string
		running bool
		want    []string
	}{
		{"info", []string{"info"}, false, []string{"Day:      7", "Money:    861", "Backpack: 4/25", "Zombies:  1", "Game:     closed"}},
		{"info running", []string{"info"}, true, []string{"Game:     RUNNING"}},
		{"bag", []string{"bag"}, false, []string{"    99  faith", "     2  heal_potion"}},
		{"belt", []string{"belt"}, false, []string{"axe_1", "sword_0"}},
		{"res filter", []string{"res", "^tech"}, false, []string{"100  tech_red"}},
		{"talents", []string{"talents"}, false, []string{"talent_red      level   4  exp   1  free    3"}},
		{"zombies", []string{"zombies"}, false, []string{"zombie_name_1  type 1  zone yard  tech R10 B0 G5", "skin_0_0:1, brain_1_0:1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := invoke(t, tt.running, append([]string{"--save-dir", dir}, tt.args...)...)
			require.NoError(t, err)
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
		})
	}
}

func TestJSONOutput(t *testing.T) {
	dir := fixture(t)
	out, err := invoke(t, false, "--save-dir", dir, "bag", "--json")
	require.NoError(t, err)
	var items []save.Stack
	require.NoError(t, json.Unmarshal([]byte(out), &items))
	require.Len(t, items, 4)
	assert.Equal(t, "faith", items[0].ID)

	out, err = invoke(t, false, "--save", filepath.Join(dir, "Steam_1.dat"), "res", "money", "--json")
	require.NoError(t, err)
	var res []save.Resource
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Equal(t, []save.Resource{{Type: "money", Value: 861}}, res)
}

func TestPaths(t *testing.T) {
	dir := fixture(t)
	out, err := invoke(t, false, "--save-dir", dir, "--game-dir", dir, "paths")
	require.NoError(t, err)
	assert.Contains(t, out, filepath.Join(dir, "Steam_1.dat"))
	assert.Contains(t, out, "game     not found")
}
