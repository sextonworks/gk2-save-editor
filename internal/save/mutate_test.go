package save

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/odin"
	"github.com/sextonworks/gk2-save-editor/internal/odin/odintest"
)

func TestInsertedItemIsIndependentCopy(t *testing.T) {
	s := openDefault(t)
	require.NoError(t, s.insertItem(Bag, "a", 1, "11111111-1111-4111-8111-111111111111"))
	require.NoError(t, s.insertItem(Bag, "b", 2, "22222222-2222-4222-8222-222222222222"))
	got := stacks(t, s, Bag)
	require.Len(t, got, 6)
	assert.Equal(t, Stack{ID: "a", Count: 1, UniqueID: "11111111-1111-4111-8111-111111111111"}, got[4])
	assert.Equal(t, Stack{ID: "b", Count: 2, UniqueID: "22222222-2222-4222-8222-222222222222"}, got[5])
	_, err := odin.Validate(s.Bytes())
	require.NoError(t, err)
}

func TestDeleteLastAndOnlyItems(t *testing.T) {
	spec := odintest.DefaultSpec()
	spec.Bag = []odintest.Stack{{ID: "salt", Count: 1}}
	s, err := Open(writeSave(t, spec))
	require.NoError(t, err)
	only := stacks(t, s, Bag)[0]
	require.NoError(t, s.deleteItem(Bag, only.UniqueID))
	assert.Empty(t, stacks(t, s, Bag))
	require.ErrorIs(t, s.deleteItem(Bag, only.UniqueID), ErrNoItem)
}

func TestBytesIsACopy(t *testing.T) {
	s := openDefault(t)
	b := s.Bytes()
	b[0] ^= 0xff
	assert.NotEqual(t, b[0], s.Bytes()[0])
}
