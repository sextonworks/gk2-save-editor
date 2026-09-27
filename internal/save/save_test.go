package save

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/odin"
	"github.com/sextonworks/gk2-save-editor/internal/odin/odintest"
)

func writeSave(t *testing.T, spec odintest.SaveSpec) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Steam_1.dat")
	require.NoError(t, os.WriteFile(path, odintest.BuildSave(spec), 0o600))
	info := "\uFEFF{\"day\": 7, \"saveDateTime\": \"9/26/2026 1:43:05 AM\", \"gameSaveVersion\": \"1.006\"}"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Steam_1.info"), []byte(info), 0o600))
	return path
}

func openDefault(t *testing.T) *Save {
	t.Helper()
	s, err := Open(writeSave(t, odintest.DefaultSpec()))
	require.NoError(t, err)
	return s
}

func TestInfo(t *testing.T) {
	info, err := openDefault(t).Info()
	require.NoError(t, err)
	assert.Equal(t, 7, info.Day)
	assert.Equal(t, "1.006", info.GameSaveVersion)
}

func TestItems(t *testing.T) {
	s := openDefault(t)
	bag, err := s.Items(Bag)
	require.NoError(t, err)
	ids := make([]string, 0, len(bag))
	for _, st := range bag {
		ids = append(ids, st.ID)
	}
	assert.Equal(t, []string{"faith", "salt", "heal_potion", "heal_potion"}, ids)
	assert.Equal(t, int64(99), bag[0].Count)
	assert.NotEmpty(t, bag[0].UniqueID)

	belt, err := s.Items(Belt)
	require.NoError(t, err)
	assert.Len(t, belt, 3)

	capacity, err := s.Capacity(Bag)
	require.NoError(t, err)
	assert.Equal(t, int64(25), capacity)

	_, err = s.Items("chest")
	require.ErrorIs(t, err, ErrContainer)
}

func TestResources(t *testing.T) {
	s := openDefault(t)
	money, err := s.Resource("money")
	require.NoError(t, err)
	assert.InDelta(t, 861, money, 1e-6)
	_, err = s.Resource("nope")
	require.ErrorIs(t, err, ErrField)
}

func TestTalentsAndZombies(t *testing.T) {
	s := openDefault(t)
	talents, err := s.Talents()
	require.NoError(t, err)
	require.Len(t, talents, 2)
	assert.Equal(t, Talent{ID: "talent_red", Level: 4, Exp: 1, FreePoints: 3, Value: 1}, talents[1])

	zombies, err := s.Zombies()
	require.NoError(t, err)
	require.Len(t, zombies, 1)
	assert.Equal(t, "zombie_name_1", zombies[0].Name)
	assert.Equal(t, int64(5), zombies[0].TechGreen)
	assert.Len(t, zombies[0].BodyParts, 3)
}

func TestLoadRejectsNonSave(t *testing.T) {
	w := odintest.NewWriter()
	w.Ref("", "Other, Assembly-CSharp").End()
	_, err := Load("x.dat", w.Bytes())
	require.ErrorIs(t, err, odin.ErrMalformed)
	_, err = Load("x.dat", []byte{0x99})
	require.Error(t, err)
}
