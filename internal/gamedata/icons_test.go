package gamedata

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/gamedata/gamedatatest"
)

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	img, err := png.Decode(f)
	require.NoError(t, err)
	return img
}

func TestExtractIcons(t *testing.T) {
	game := t.TempDir()
	require.NoError(t, gamedatatest.Write(game, []string{"candle_basic"}, nil, nil))
	cache := t.TempDir()
	c, err := Load(game, cache, false)
	require.NoError(t, err)
	require.NoError(t, c.EnsureIcons(context.Background(), game, cache))
	assert.Equal(t, IconsDir(cache, c.Source), c.IconDir)

	tests := []struct {
		name string
		want color.NRGBA
	}{
		{"i_candle_basic", color.NRGBA{255, 0, 0, 255}},
		{"i_faith", color.NRGBA{0, 0, 255, 255}},
		{"i_bundle_icon", color.NRGBA{0, 255, 0, 255}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := IconPath(c.IconDir, tt.name)
			require.NoError(t, err)
			img := readPNG(t, path)
			assert.Equal(t, image.Rect(0, 0, 6, 3), img.Bounds())
			for x := range 6 {
				for y := range 3 {
					got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
					if y == 1 && x >= 1 && x <= 4 {
						assert.Equal(t, tt.want, got, "pixel %d,%d", x, y)
					} else {
						assert.Zero(t, got.A, "pixel %d,%d", x, y)
					}
				}
			}
		})
	}

	_, err = IconPath(c.IconDir, "hero_walk")
	require.ErrorIs(t, err, ErrNoIcon)
	n, err := ExtractIcons(context.Background(), game, c.IconDir)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

func TestIconPathRejectsBadNames(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ok.png"), nil, 0o600))
	_, err := IconPath(dir, "ok")
	require.NoError(t, err)
	for _, name := range []string{"../ok", "a/b", "", "missing", ".."} {
		_, err := IconPath(dir, name)
		require.ErrorIs(t, err, ErrNoIcon, name)
	}
}

func TestDecodeTexture(t *testing.T) {
	tests := []struct {
		name   string
		pix    []byte
		format int
		want   color.NRGBA
	}{
		{"alpha8", []byte{7}, formatAlpha8, color.NRGBA{255, 255, 255, 7}},
		{"rgb24", []byte{1, 2, 3}, formatRGB24, color.NRGBA{1, 2, 3, 255}},
		{"rgba32", []byte{1, 2, 3, 4}, formatRGBA32, color.NRGBA{1, 2, 3, 4}},
		{"argb32", []byte{4, 1, 2, 3}, formatARGB32, color.NRGBA{1, 2, 3, 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := decodeTexture(tt.pix, 1, 1, tt.format)
			require.NoError(t, err)
			assert.Equal(t, tt.want, img.NRGBAAt(0, 0))
		})
	}
	_, err := decodeTexture([]byte{1}, 1, 1, 99)
	require.ErrorIs(t, err, ErrBadTexture)
	_, err = decodeTexture([]byte{1}, 2, 2, formatRGBA32)
	require.ErrorIs(t, err, ErrBadTexture)
	_, err = decodeTexture(nil, 0, 1, formatRGBA32)
	require.ErrorIs(t, err, ErrBadTexture)
}
