package unityasset

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/unityasset/unitytest"
)

func bundleSpec() unitytest.BundleSpec {
	return unitytest.BundleSpec{
		Files: []unitytest.BundleFile{
			{Name: "CAB-1", Data: sample().Build()},
			{Name: "CAB-1.resS", Data: bytes.Repeat([]byte("pixels"), 1000)},
		},
		BlockSize: 512,
	}
}

func TestParseBundle(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*unitytest.BundleSpec)
	}{
		{"lz4", func(*unitytest.BundleSpec) {}},
		{"stored", func(s *unitytest.BundleSpec) { s.Stored = true }},
		{"info at end", func(s *unitytest.BundleSpec) { s.InfoAtEnd = true }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spec := bundleSpec()
			tt.mutate(&spec)
			b, err := ParseBundle(spec.Build())
			require.NoError(t, err)
			assert.Equal(t, []string{"CAB-1", "CAB-1.resS"}, b.Names())
			af, _, err := b.Assets()
			require.NoError(t, err)
			require.Len(t, af.Objects, 3)
			stream, ok := b.Stream("archive:/CAB-1/CAB-1.resS")
			require.True(t, ok)
			assert.Equal(t, spec.Files[1].Data, stream)
		})
	}
}

func TestPeekAndOpenBundle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.bundle")
	require.NoError(t, os.WriteFile(path, bundleSpec().Build(), 0o600))
	head, err := PeekBundle(path, 256)
	require.NoError(t, err)
	ids, err := ClassIDs(head)
	require.NoError(t, err)
	assert.Equal(t, []int32{ClassMonoBehavour, 49}, ids)
	b, err := OpenBundle(path)
	require.NoError(t, err)
	_, ok := b.File("CAB-1")
	assert.True(t, ok)
	_, err = OpenBundle(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
}

func TestBundleRejects(t *testing.T) {
	good := bundleSpec().Build()
	tests := []struct {
		name string
		raw  []byte
		want error
	}{
		{"signature", append([]byte("UnityWeb\x00"), good[8:]...), ErrUnsupported},
		{"version", func() []byte { b := bytes.Clone(good); b[11] = 3; return b }(), ErrUnsupported},
		{"truncated", good[:60], ErrCorrupt},
		{"empty", nil, ErrCorrupt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseBundle(tt.raw)
			require.ErrorIs(t, err, tt.want)
		})
	}
	only := unitytest.BundleSpec{Files: []unitytest.BundleFile{{Name: "x.resS", Data: []byte("a")}}}
	b, err := ParseBundle(only.Build())
	require.NoError(t, err)
	_, _, err = b.Assets()
	require.ErrorIs(t, err, ErrCorrupt)
	_, err = decompress([]byte{1}, 1, 1)
	require.ErrorIs(t, err, ErrUnsupported)
	_, err = decompress([]byte{1, 2}, 1, 0)
	require.ErrorIs(t, err, ErrCorrupt)
}
