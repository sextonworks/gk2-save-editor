package session

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/gamedata/gamedatatest"
	"github.com/sextonworks/gk2-save-editor/internal/locate"
	"github.com/sextonworks/gk2-save-editor/internal/odin/odintest"
	"github.com/sextonworks/gk2-save-editor/internal/save"
)

type harness struct {
	s       *Session
	saves   string
	running atomic.Bool
	events  atomic.Int32
}

func newHarness(t *testing.T, withGame bool) *harness {
	t.Helper()
	h := &harness{saves: t.TempDir()}
	require.NoError(t, os.WriteFile(filepath.Join(h.saves, "Steam_1.dat"), odintest.BuildSave(odintest.DefaultSpec()), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(h.saves, "Steam_1.info"), []byte(`{"day": 7}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(h.saves, "Steam_1_backup_1.dat"), []byte("x"), 0o600))
	vars := map[string]string{"GK2_SAVE_DIR": h.saves}
	if withGame {
		game := t.TempDir()
		require.NoError(t, gamedatatest.Write(game, []string{"salt", "heal_potion", "candle_basic"}, nil,
			map[string]map[string]string{"en": {"candle_basic": "Simple Candle", "faith": "Faith"}, "ru": {"faith": "Вера"}}))
		vars["GK2_GAME_DIR"] = game
	}
	env := locate.System()
	env.Home = t.TempDir()
	env.Getenv = func(k string) string { return vars[k] }
	h.s = New(Config{
		Env:       env,
		Running:   func(context.Context) (bool, error) { return h.running.Load(), nil },
		BackupDir: filepath.Join(t.TempDir(), "backups"),
		CacheDir:  t.TempDir(),
		Notify:    func(string) { h.events.Add(1) },
		PollEvery: 10 * time.Millisecond,
	})
	return h
}

func (h *harness) open(t *testing.T) {
	t.Helper()
	h.s.Startup(context.Background())
	t.Cleanup(h.s.Shutdown)
	st, err := h.s.Open("Steam_1")
	require.NoError(t, err)
	require.True(t, st.Open)
}

func TestEnvironmentAndSlots(t *testing.T) {
	h := newHarness(t, true)
	env := h.s.Environment()
	assert.Equal(t, h.saves, env.SaveDir)
	assert.NotEmpty(t, env.GameDir)
	assert.Empty(t, env.CatalogError)
	require.Len(t, env.Slots, 1)
	assert.Equal(t, "Steam_1", env.Slots[0].Name)
	assert.Equal(t, 7, env.Slots[0].Day)

	noGame := newHarness(t, false)
	env = noGame.s.Environment()
	assert.NotEmpty(t, env.GameDirError)
}

func TestViewsWithNames(t *testing.T) {
	h := newHarness(t, true)
	h.open(t)
	bag, err := h.s.Container("bag", "ru")
	require.NoError(t, err)
	assert.Equal(t, int64(25), bag.Capacity)
	assert.Equal(t, ItemView{UniqueID: bag.Items[0].UniqueID, ID: "faith", Name: "Вера", Count: 99, Icon: "i_faith", Stack: 999}, bag.Items[0])

	p, err := h.s.Player("en")
	require.NoError(t, err)
	assert.InDelta(t, 861, p.Money, 1e-6)
	assert.Len(t, p.Talents, 2)

	zombies, err := h.s.Zombies("en")
	require.NoError(t, err)
	require.Len(t, zombies, 1)
	assert.Len(t, zombies[0].Parts, 3)

	insp, err := h.s.Inspirations("en")
	require.NoError(t, err)
	assert.Len(t, insp, 4)

	techs, err := h.s.Techs("en")
	require.NoError(t, err)
	assert.Len(t, techs, 4)
	assert.Equal(t, "unlocked", techs[0].State)

	found, err := h.s.SearchItems("candle", "en", false, 10)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "Simple Candle", found[0].Name)
}

func TestEditUndoRedoAndWrite(t *testing.T) {
	h := newHarness(t, true)
	h.open(t)
	bag, err := h.s.Container("bag", "en")
	require.NoError(t, err)

	st, err := h.s.SetMoney(5000)
	require.NoError(t, err)
	assert.True(t, st.Dirty)
	_, err = h.s.AddItem("bag", "candle_basic", 3)
	require.NoError(t, err)
	_, err = h.s.AddItem("bag", "not_real", 1)
	require.Error(t, err)
	_, err = h.s.SetItemCount("bag", bag.Items[0].UniqueID, 5)
	require.NoError(t, err)
	_, err = h.s.RemoveItem("bag", bag.Items[1].UniqueID)
	require.NoError(t, err)
	_, err = h.s.SwapItem("bag", bag.Items[2].UniqueID, "salt")
	require.NoError(t, err)
	_, err = h.s.SetTalentPoints("talent_red", 99)
	require.NoError(t, err)
	_, err = h.s.MaxZombie("zombie_name_1", 999, 10)
	require.NoError(t, err)
	_, err = h.s.EquipBest()
	require.NoError(t, err)
	st, err = h.s.Inspire("", "")
	require.NoError(t, err)
	assert.Len(t, st.Changes, 10)
	kinds := make([]string, 0, len(st.Changes))
	for _, c := range st.Changes {
		kinds = append(kinds, c.Kind)
	}
	assert.Equal(t, []string{"money", "add", "count", "remove", "replace", "talent", "zombie", "equip", "inspire", "inspire"}, kinds)
	assert.Equal(t, Change{Kind: "add", Subject: "Simple Candle", Value: "3", Where: "bag"}, st.Changes[1])
	assert.Equal(t, "Faith", st.Changes[2].Subject)
	ru := h.s.SetLang("ru")
	assert.Equal(t, "Вера", ru.Changes[2].Subject)
	h.s.SetLang("en")

	st, err = h.s.Undo()
	require.NoError(t, err)
	assert.Len(t, st.Changes, 9)
	assert.True(t, st.CanRedo)
	st, err = h.s.Redo()
	require.NoError(t, err)
	assert.Len(t, st.Changes, 10)

	res, err := h.s.Write(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, res.Backup)
	assert.False(t, res.State.Dirty)

	reopened, err := save.Open(filepath.Join(h.saves, "Steam_1.dat"))
	require.NoError(t, err)
	money, err := reopened.Resource("money")
	require.NoError(t, err)
	assert.InDelta(t, 5000, money, 1e-6)

	backups, err := h.s.Backups()
	require.NoError(t, err)
	require.Len(t, backups, 1)
	st, err = h.s.Restore(context.Background(), backups[0].Name)
	require.NoError(t, err)
	assert.False(t, st.Dirty)
	p, err := h.s.Player("en")
	require.NoError(t, err)
	assert.InDelta(t, 861, p.Money, 1e-6)
}

func TestWriteRefusals(t *testing.T) {
	h := newHarness(t, false)
	_, err := h.s.Write(context.Background())
	require.ErrorIs(t, err, ErrNoSave)
	h.open(t)
	_, err = h.s.SetMoney(1)
	require.NoError(t, err)

	h.running.Store(true)
	_, err = h.s.Write(context.Background())
	require.ErrorIs(t, err, ErrGameRunning)
	_, err = h.s.Restore(context.Background(), "x.zip")
	require.ErrorIs(t, err, ErrGameRunning)
	h.running.Store(false)

	require.NoError(t, os.WriteFile(filepath.Join(h.saves, "Steam_1.dat"), append(odintest.BuildSave(odintest.DefaultSpec()), 0x31), 0o600))
	require.Eventually(t, func() bool { return h.s.State().Conflict }, time.Second, 5*time.Millisecond)
	assert.Positive(t, h.events.Load())
	_, err = h.s.Write(context.Background())
	require.ErrorIs(t, err, ErrConflict)

	st, err := h.s.Reload()
	require.NoError(t, err)
	assert.False(t, st.Conflict)
	assert.False(t, st.Dirty)
}

func TestWithoutGameData(t *testing.T) {
	h := newHarness(t, false)
	h.open(t)
	bag, err := h.s.Container("bag", "ru")
	require.NoError(t, err)
	assert.Equal(t, "faith", bag.Items[0].Name)
	_, err = h.s.SearchItems("x", "en", false, 5)
	require.ErrorIs(t, err, ErrNoCatalog)
	_, err = h.s.AddItem("bag", "anything", 1)
	require.NoError(t, err)
}

func TestNoSaveOpen(t *testing.T) {
	h := newHarness(t, false)
	_, err := h.s.Container("bag", "en")
	require.ErrorIs(t, err, ErrNoSave)
	_, err = h.s.SetMoney(1)
	require.ErrorIs(t, err, ErrNoSave)
	_, err = h.s.Undo()
	require.ErrorIs(t, err, ErrNoSave)
	_, err = h.s.Redo()
	require.ErrorIs(t, err, ErrNoSave)
	_, err = h.s.Reload()
	require.ErrorIs(t, err, ErrNoSave)
	h.s.Environment()
	_, err = h.s.Open("../escape")
	require.ErrorIs(t, err, ErrNoSave)
	_, err = h.s.Open("Steam_9")
	require.Error(t, err)
}

func TestInspector(t *testing.T) {
	h := newHarness(t, false)
	h.open(t)
	top, err := h.s.InspectChildren(-1, 0, 10)
	require.NoError(t, err)
	require.Len(t, top, 1)
	assert.Equal(t, "object", top[0].Kind)

	kids, err := h.s.InspectChildren(top[0].Offset, 0, 2)
	require.NoError(t, err)
	require.Len(t, kids, 2)
	assert.Equal(t, "gameSaveVersion", kids[0].Name)
	assert.Equal(t, "1.006", kids[0].Value)

	hits, err := h.s.InspectSearch("heal_potion", 10)
	require.NoError(t, err)
	assert.Len(t, hits, 2)
	empty, err := h.s.InspectSearch("  ", 10)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestUseFolders(t *testing.T) {
	h := newHarness(t, false)
	other := t.TempDir()
	env, err := h.s.UseFolders(other, "")
	require.NoError(t, err)
	assert.Equal(t, other, env.SaveDir)
	assert.Empty(t, env.Slots)
	_, err = h.s.UseFolders(filepath.Join(other, "missing"), "")
	require.ErrorIs(t, err, locate.ErrNotFound)
	_, err = h.s.UseFolders("", other)
	require.ErrorIs(t, err, locate.ErrNotFound)
}

func TestStoragesAndIcons(t *testing.T) {
	h := newHarness(t, true)
	h.open(t)
	list, err := h.s.Storages("en")
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "wgo:chest-home", list[0].UniqueID)
	assert.Equal(t, "chest_rough", list[1].ID)
	assert.Equal(t, int64(20), list[1].Capacity)
	require.Len(t, list[1].Items, 2)

	st, err := h.s.AddItem(list[0].UniqueID, "candle_basic", 2)
	require.NoError(t, err)
	assert.Equal(t, Change{Kind: "add", Subject: "Simple Candle", Value: "2", Where: "chest_kitchen"}, st.Changes[0])
	list, err = h.s.Storages("en")
	require.NoError(t, err)
	require.Len(t, list[0].Items, 1)
	assert.Equal(t, "i_candle_basic", list[0].Items[0].Icon)

	require.Eventually(t, func() bool { return h.s.State().IconsReady }, 5*time.Second, 10*time.Millisecond)
	path, err := h.s.IconPath("i_candle_basic")
	require.NoError(t, err)
	assert.FileExists(t, path)
	_, err = h.s.IconPath("../etc")
	require.Error(t, err)
}

func TestResourceGroups(t *testing.T) {
	h := newHarness(t, true)
	h.open(t)
	p, err := h.s.Player("en")
	require.NoError(t, err)
	groups := map[string]string{}
	for _, r := range p.Resources {
		groups[r.Type] = r.Group
	}
	assert.Equal(t, map[string]string{"tech_red": GroupMain, "energy": GroupMain}, groups)
	assert.Equal(t, "tech_red", p.Resources[0].Icon)
	assert.True(t, p.Resources[0].Editable)

	tests := []struct {
		res      string
		group    string
		editable bool
	}{
		{"village_REP", GroupReputation, true},
		{"wz_graveyard", GroupZones, false},
		{"perk_mason", GroupPerks, false},
		{"donkey_ready", GroupOther, true},
	}
	for _, tt := range tests {
		v := h.s.resource(save.Resource{Type: tt.res}, "en")
		assert.Equal(t, tt.group, v.Group, tt.res)
		assert.Equal(t, tt.editable, v.Editable, tt.res)
	}
}

func TestIconsNotReady(t *testing.T) {
	h := newHarness(t, false)
	_, err := h.s.IconPath("x")
	require.ErrorIs(t, err, ErrNoIcons)
}
