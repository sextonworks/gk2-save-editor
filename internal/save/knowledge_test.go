package save

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/odin/odintest"
)

func TestInspirations(t *testing.T) {
	list, err := openDefault(t).Inspirations()
	require.NoError(t, err)
	require.Len(t, list, 4)
	assert.Equal(t, Inspiration{Talent: "talent_orange", ID: "insp_orange_0", Current: 3, Goal: 6}, list[0])
	assert.False(t, list[0].Ready())
	assert.True(t, list[1].Ready())
}

func TestTechs(t *testing.T) {
	techs := openDefault(t).Techs()
	assert.Equal(t, []string{"candles_1", "garden_improve_1"}, techs.Unlocked)
	assert.Equal(t, []string{"garden_honey"}, techs.Revealed)
	assert.Equal(t, []string{"zombie_wood"}, techs.Hidden)
}

func TestSetInspirationProgress(t *testing.T) {
	e := editor(t, odintest.DefaultSpec())
	require.NoError(t, e.Apply(SetInspirationProgress{Talent: "talent_red", ID: "insp_red_0", Value: 6}))
	list, err := e.Save().Inspirations()
	require.NoError(t, err)
	assert.Equal(t, int64(6), list[2].Current)
	assert.True(t, list[2].Ready())
	require.ErrorIs(t, e.Apply(SetInspirationProgress{Talent: "talent_red", ID: "nope", Value: 1}), ErrField)
	require.ErrorIs(t, e.Apply(SetInspirationProgress{Talent: "talent_red", ID: "insp_red_1", Value: 0}), ErrField)
	assert.True(t, Inspiration{Current: 1, Goal: 5}.CanBringToGoal())
	assert.False(t, Inspiration{Current: 22, Goal: 0}.CanBringToGoal())
	assert.False(t, Inspiration{Current: 5, Goal: 5}.CanBringToGoal())
	require.ErrorIs(t, e.Apply(SetInspirationProgress{Talent: "talent_x", ID: "insp_red_0", Value: 1}), ErrField)
	assert.NotEmpty(t, SetInspirationProgress{}.String())
}
