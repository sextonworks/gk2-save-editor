package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/locate"
	"github.com/sextonworks/gk2-save-editor/internal/save"
)

type writeEnv struct {
	dir     string
	backups string
	running []bool
	slept   []time.Duration
}

func newWriteEnv(t *testing.T) *writeEnv {
	t.Helper()
	return &writeEnv{dir: fixture(t), backups: filepath.Join(t.TempDir(), "backups")}
}

func (w *writeEnv) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	g := globals{
		env: locate.System(),
		running: func(context.Context) (bool, error) {
			if len(w.running) == 0 {
				return false, nil
			}
			r := w.running[0]
			w.running = w.running[1:]
			return r, nil
		},
		sleep: func(_ context.Context, d time.Duration) error {
			w.slept = append(w.slept, d)
			return nil
		},
	}
	full := append([]string{"--save-dir", w.dir, "--backup-dir", w.backups}, args...)
	err := run(context.Background(), full, &out, g)
	return out.String(), err
}

func (w *writeEnv) save(t *testing.T) *save.Save {
	t.Helper()
	s, err := save.Open(filepath.Join(w.dir, "Steam_1.dat"))
	require.NoError(t, err)
	return s
}

func (w *writeEnv) bag(t *testing.T) []string {
	t.Helper()
	items, err := w.save(t).Items(save.Bag)
	require.NoError(t, err)
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID+"="+strings.TrimSpace(number(float64(it.Count))))
	}
	return out
}

func TestWriteCommands(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(t *testing.T, w *writeEnv)
	}{
		{"add", []string{"add", "candle_basic=5", "heal_potion"}, func(t *testing.T, w *writeEnv) {
			assert.Equal(t, []string{"faith=99", "salt=1", "heal_potion=3", "heal_potion=2", "candle_basic=5", "heal_potion=1"}, w.bag(t))
		}},
		{"count first stack", []string{"count", "heal_potion", "9"}, func(t *testing.T, w *writeEnv) {
			assert.Equal(t, []string{"faith=99", "salt=1", "heal_potion=9", "heal_potion=2"}, w.bag(t))
		}},
		{"count all stacks", []string{"count", "heal_potion", "4", "--all"}, func(t *testing.T, w *writeEnv) {
			assert.Equal(t, []string{"faith=99", "salt=1", "heal_potion=4", "heal_potion=4"}, w.bag(t))
		}},
		{"remove", []string{"remove", "salt", "heal_potion", "--all"}, func(t *testing.T, w *writeEnv) {
			assert.Equal(t, []string{"faith=99"}, w.bag(t))
		}},
		{"swap", []string{"swap", "salt", "candle_master"}, func(t *testing.T, w *writeEnv) {
			assert.Equal(t, "candle_master=1", w.bag(t)[1])
		}},
		{"money", []string{"money", "9999999"}, func(t *testing.T, w *writeEnv) {
			v, err := w.save(t).Resource("money")
			require.NoError(t, err)
			assert.InDelta(t, 9999999, v, 1e-6)
		}},
		{"set-res", []string{"set-res", "tech_red", "999"}, func(t *testing.T, w *writeEnv) {
			v, err := w.save(t).Resource("tech_red")
			require.NoError(t, err)
			assert.InDelta(t, 999, v, 1e-6)
		}},
		{"set-talents branch", []string{"set-talents", "50", "--branch", "talent_red"}, func(t *testing.T, w *writeEnv) {
			talents, err := w.save(t).Talents()
			require.NoError(t, err)
			assert.Equal(t, []int64{2, 50}, []int64{talents[0].FreePoints, talents[1].FreePoints})
		}},
		{"zombies-max", []string{"zombies-max"}, func(t *testing.T, w *writeEnv) {
			zombies, err := w.save(t).Zombies()
			require.NoError(t, err)
			assert.Equal(t, int64(999999), zombies[0].TechBlue)
			assert.Equal(t, "brain_3_1:3", zombies[0].BodyParts[1].ID)
			assert.Equal(t, int64(328), zombies[0].BodyParts[1].Count)
		}},
		{"equip-best", []string{"equip-best"}, func(t *testing.T, w *writeEnv) {
			items, err := w.save(t).Items(save.Belt)
			require.NoError(t, err)
			assert.Equal(t, "sword_4", items[2].ID)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWriteEnv(t)
			out, err := w.run(t, tt.args...)
			require.NoError(t, err, out)
			assert.Contains(t, out, "Written. Backup: ")
			tt.check(t, w)
			entries, err := os.ReadDir(w.backups)
			require.NoError(t, err)
			assert.Len(t, entries, 1)
		})
	}
}

func TestDryRunAndNoop(t *testing.T) {
	w := newWriteEnv(t)
	path := filepath.Join(w.dir, "Steam_1.dat")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	out, err := w.run(t, "-n", "money", "5000")
	require.NoError(t, err)
	assert.Contains(t, out, "Dry run, nothing written.")

	out, err = w.run(t, "money", "861")
	require.NoError(t, err)
	assert.Contains(t, out, "Nothing to change.")

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	_, err = os.Stat(w.backups)
	assert.True(t, os.IsNotExist(err))
}

func TestGameRunning(t *testing.T) {
	w := newWriteEnv(t)
	w.running = []bool{true}
	_, err := w.run(t, "money", "5")
	require.ErrorIs(t, err, errGameRunning)

	w.running = []bool{true, true, false}
	out, err := w.run(t, "-w", "money", "5")
	require.NoError(t, err)
	assert.Contains(t, out, "Waiting for the game to close...")
	assert.Equal(t, []time.Duration{pollInterval, pollInterval, settleDelay}, w.slept)
}

func TestMoneyShowAndErrors(t *testing.T) {
	w := newWriteEnv(t)
	out, err := w.run(t, "money")
	require.NoError(t, err)
	assert.Equal(t, "861\n", out)

	_, err = w.run(t, "add", "candle=0")
	require.ErrorIs(t, err, errBadSpec)
	_, err = w.run(t, "add", "=3")
	require.ErrorIs(t, err, errBadSpec)
	_, err = w.run(t, "count", "missing", "1")
	require.ErrorIs(t, err, save.ErrNoItem)
	_, err = w.run(t, "set-res", "nope", "1")
	require.ErrorIs(t, err, save.ErrField)
	_, err = w.run(t, "add", "--where", "chest", "salt")
	require.Error(t, err)
}

func TestBackupsAndRestore(t *testing.T) {
	w := newWriteEnv(t)
	_, err := w.run(t, "money", "1")
	require.NoError(t, err)
	_, err = w.run(t, "money", "2")
	require.NoError(t, err)

	out, err := w.run(t, "backups")
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 3)
	first := strings.Fields(lines[1])[0]

	out, err = w.run(t, "-n", "restore", first)
	require.NoError(t, err)
	assert.Contains(t, out, "Would restore")

	out, err = w.run(t, "restore", first)
	require.NoError(t, err)
	assert.Contains(t, out, "Restored "+first)
	v, err := w.save(t).Resource("money")
	require.NoError(t, err)
	assert.InDelta(t, 861, v, 1e-6)

	_, err = w.run(t, "restore", "missing.zip")
	require.Error(t, err)
}

func TestExplicitSaveSkipsGameCheck(t *testing.T) {
	w := newWriteEnv(t)
	w.running = []bool{true}
	var out bytes.Buffer
	g := globals{env: locate.System(), running: func(context.Context) (bool, error) { return true, nil }, sleep: sleep}
	err := run(context.Background(), []string{"--save", filepath.Join(w.dir, "Steam_1.dat"), "--backup-dir", w.backups, "money", "7"}, &out, g)
	require.NoError(t, err, out.String())
}

func TestSleepHonoursContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, sleep(ctx, time.Hour), context.Canceled)
	require.NoError(t, sleep(context.Background(), time.Millisecond))
}
