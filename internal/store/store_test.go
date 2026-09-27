package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pair(t *testing.T, dat, info string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Steam_1.dat")
	require.NoError(t, os.WriteFile(path, []byte(dat), 0o600))
	if info != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Steam_1.info"), []byte(info), 0o600))
	}
	return path
}

func clock(start time.Time) func() time.Time {
	cur := start
	return func() time.Time {
		cur = cur.Add(time.Second)
		return cur
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s := New(filepath.Join(t.TempDir(), "backups"))
	s.Now = clock(time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC))
	return s
}

func TestWriteAtomic(t *testing.T) {
	path := pair(t, "old", "")
	require.NoError(t, WriteAtomic(path, []byte("new"), Hash([]byte("old"))))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))

	err = WriteAtomic(path, []byte("newer"), Hash([]byte("old")))
	require.ErrorIs(t, err, ErrChangedOnDisk)
	got, _ = os.ReadFile(path)
	assert.Equal(t, "new", string(got))

	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temp files left behind")

	require.Error(t, WriteAtomic(filepath.Join(t.TempDir(), "missing.dat"), nil, Hash(nil)))
}

func TestCreateListRestore(t *testing.T) {
	path := pair(t, "save-v1", `{"day":1}`)
	s := testStore(t)
	b, err := s.Create(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"Steam_1.dat", "Steam_1.info"}, b.Files)
	assert.False(t, b.Legacy)

	require.NoError(t, os.WriteFile(path, []byte("save-v2"), 0o600))
	require.NoError(t, os.Remove(filepath.Join(filepath.Dir(path), "Steam_1.info")))

	list, err := s.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, b.Name, list[0].Name)

	restored, err := s.Restore(b.Name, filepath.Dir(path))
	require.NoError(t, err)
	assert.Equal(t, []string{"Steam_1.dat", "Steam_1.info"}, restored)
	got, _ := os.ReadFile(path)
	assert.Equal(t, "save-v1", string(got))
	info, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "Steam_1.info"))
	assert.JSONEq(t, `{"day":1}`, string(info))
}

func TestDamagedBackupIsRejected(t *testing.T) {
	path := pair(t, "data", "")
	s := testStore(t)
	b, err := s.Create(path)
	require.NoError(t, err)
	raw, err := os.ReadFile(b.Path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(b.Path, raw[:len(raw)/2], 0o600))
	_, err = s.Restore(b.Name, filepath.Dir(path))
	require.ErrorIs(t, err, ErrBadBackup)
	list, err := s.List()
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestLegacyFolderBackup(t *testing.T) {
	s := testStore(t)
	legacy := filepath.Join(s.Dir, "20260926_020947")
	require.NoError(t, os.MkdirAll(legacy, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(legacy, "Steam_1.dat"), []byte("old"), 0o600))
	list, err := s.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, list[0].Legacy)

	target := t.TempDir()
	restored, err := s.Restore("20260926_020947", target)
	require.NoError(t, err)
	assert.Equal(t, []string{"Steam_1.dat"}, restored)
}

func TestRestoreRejectsBadNames(t *testing.T) {
	s := testStore(t)
	for _, name := range []string{"../x.zip", "missing.zip"} {
		_, err := s.Restore(name, t.TempDir())
		require.ErrorIs(t, err, ErrNoBackup, name)
	}
}

func TestPruneKeepsNewest(t *testing.T) {
	path := pair(t, "x", "")
	s := testStore(t)
	s.Keep = 2
	var warned []error
	s.Warn = func(err error) { warned = append(warned, err) }
	names := make([]string, 0, 3)
	for range 3 {
		b, err := s.Create(path)
		require.NoError(t, err)
		names = append(names, b.Name)
	}
	list, err := s.List()
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, names[1:], []string{list[0].Name, list[1].Name})
	assert.Empty(t, warned)
}

func TestSameSecondNamesDoNotCollide(t *testing.T) {
	path := pair(t, "x", "")
	s := testStore(t)
	fixed := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return fixed }
	a, err := s.Create(path)
	require.NoError(t, err)
	b, err := s.Create(path)
	require.NoError(t, err)
	assert.NotEqual(t, a.Name, b.Name)
}

func TestListMissingDir(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "none"))
	list, err := s.List()
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestDataDir(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	tests := []struct {
		name string
		vars map[string]string
		goos string
		want string
	}{
		{"override", map[string]string{"GK2_DATA_DIR": "/x"}, "linux", "/x"},
		{"windows", map[string]string{"LOCALAPPDATA": `C:\Users\u\AppData\Local`}, "windows", filepath.Join(`C:\Users\u\AppData\Local`, "gk2")},
		{"windows fallback", nil, "windows", filepath.Join("/home/u", "AppData", "Local", "gk2")},
		{"darwin", nil, "darwin", filepath.Join("/home/u", "Library", "Application Support", "gk2")},
		{"xdg", map[string]string{"XDG_DATA_HOME": "/data"}, "linux", filepath.Join("/data", "gk2")},
		{"linux", nil, "linux", filepath.Join("/home/u", ".local", "share", "gk2")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DataDir(env(tt.vars), tt.goos, "/home/u"))
		})
	}
	assert.NotEmpty(t, DefaultBackupDir())
}

func TestCreateMissingSave(t *testing.T) {
	s := testStore(t)
	_, err := s.Create(filepath.Join(t.TempDir(), "missing.dat"))
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrBadBackup))
}
