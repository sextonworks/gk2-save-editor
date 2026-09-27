package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/save"
)

func TestInspirationsAndTechs(t *testing.T) {
	w := newWriteEnv(t)
	out, err := w.run(t, "inspirations")
	require.NoError(t, err)
	assert.Contains(t, out, "  talent_orange  insp_orange_0")
	assert.Contains(t, out, "* talent_orange  insp_orange_1")

	out, err = w.run(t, "techs")
	require.NoError(t, err)
	assert.Contains(t, out, "Unlocked (2)")
	assert.Contains(t, out, "Hidden (1)")
}

func TestInspire(t *testing.T) {
	w := newWriteEnv(t)
	out, err := w.run(t, "inspire", "--talent", "talent_red")
	require.NoError(t, err, out)
	list, err := w.save(t).Inspirations()
	require.NoError(t, err)
	ready := map[string]bool{}
	for _, i := range list {
		ready[i.ID] = i.Ready()
	}
	assert.Equal(t, map[string]bool{"insp_orange_0": false, "insp_orange_1": true, "insp_red_0": true, "insp_red_1": true}, ready)

	out, err = w.run(t, "inspire", "insp_red_0")
	require.NoError(t, err)
	assert.Contains(t, out, "Nothing to change.")

	_, err = w.run(t, "inspire", "missing")
	require.ErrorIs(t, err, save.ErrField)
}
