package save

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/odin/odintest"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUndoRedoReplaysExactly(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	original := e.Save().Bytes()
	bag := stacks(t, e.Save(), Bag)

	require.NoError(t, e.Apply(NewAddItem(Bag, "candle_basic", 3)))
	afterAdd := e.Save().Bytes()
	require.NoError(t, e.Apply(RemoveItem{Container: Bag, UniqueID: bag[1].UniqueID}))
	afterRemove := e.Save().Bytes()
	assert.Equal(t, []string{"bag: + candle_basic x3", "bag: - stack " + bag[1].UniqueID}, e.Changes())
	assert.True(t, e.Dirty())

	require.NoError(t, e.Undo())
	assert.Equal(t, afterAdd, e.Save().Bytes())
	require.NoError(t, e.Undo())
	assert.Equal(t, original, e.Save().Bytes())
	assert.False(t, e.CanUndo())
	require.ErrorIs(t, e.Undo(), ErrNothingToUndo)

	require.NoError(t, e.Redo())
	require.NoError(t, e.Redo())
	assert.Equal(t, afterRemove, e.Save().Bytes())
	assert.False(t, e.CanRedo())
	require.ErrorIs(t, e.Redo(), ErrNothingToUndo)
}

func TestOpsAndOriginal(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	require.NoError(t, e.Apply(SetResource{Type: "money", Value: 1}))
	ops := e.Ops()
	require.Len(t, ops, 1)
	assert.Equal(t, SetResource{Type: "money", Value: 1}, ops[0])
	orig, err := e.Original()
	require.NoError(t, err)
	v, err := orig.Resource("money")
	require.NoError(t, err)
	assert.InDelta(t, 861, v, 1e-6)
}

func TestApplyAfterUndoDropsRedoTail(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	require.NoError(t, e.Apply(SetResource{Type: "money", Value: 1}))
	require.NoError(t, e.Apply(SetResource{Type: "money", Value: 2}))
	require.NoError(t, e.Undo())
	require.NoError(t, e.Apply(SetResource{Type: "money", Value: 3}))
	assert.False(t, e.CanRedo())
	assert.Equal(t, []string{"set money = 1", "set money = 3"}, e.Changes())
}

func TestMarkSavedResetsJournal(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	before := e.OriginalHash()
	require.NoError(t, e.Apply(SetResource{Type: "money", Value: 42}))
	e.MarkSaved(e.Save().Bytes())
	assert.False(t, e.Dirty())
	assert.False(t, e.CanUndo())
	assert.NotEqual(t, before, e.OriginalHash())
	v, err := e.Save().Resource("money")
	require.NoError(t, err)
	assert.InDelta(t, 42, v, 1e-6)
}

func TestNewUUID(t *testing.T) {
	a, b := NewUUID(), NewUUID()
	assert.Regexp(t, uuidRe, a)
	assert.NotEqual(t, a, b)
}

func TestOpenEditorMissingFile(t *testing.T) {
	_, err := OpenEditor(t.TempDir() + "/missing.dat")
	require.Error(t, err)
}
