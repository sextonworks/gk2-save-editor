package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/locate"
)

func TestErrors(t *testing.T) {
	dir := fixture(t)
	_, err := invoke(t, false, "--save-dir", dir, "--slot", "Steam_9", "bag")
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = invoke(t, false, "--save-dir", filepath.Join(dir, "missing"), "bag")
	require.ErrorIs(t, err, locate.ErrNotFound)
	_, err = invoke(t, false, "--save-dir", dir, "res", "(")
	require.Error(t, err)
	_, err = invoke(t, false, "no-such-command")
	require.Error(t, err)
}

func TestHelpAndVersion(t *testing.T) {
	out, err := invoke(t, false)
	require.NoError(t, err)
	assert.Contains(t, out, "Usage: gk2")
	out, err = invoke(t, false, "--version")
	require.NoError(t, err)
	assert.Contains(t, out, "gk2 ")
}
