package unityasset

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/unityasset/unitytest"
)

func sample() unitytest.Builder {
	return unitytest.Builder{Version: SupportedVersion, Objects: []unitytest.Object{
		{PathID: 1, Class: ClassMonoBehavour, Data: unitytest.Mono("GameBalance", []byte("items"))},
		{PathID: 2, Class: 49, Data: []byte("text asset")},
		{PathID: 3, Class: ClassMonoBehavour, Data: unitytest.Mono("lng_ru", []byte("ru"))},
	}}
}

func parse(t *testing.T, raw []byte) (*File, error) {
	t.Helper()
	return Parse(bytes.NewReader(raw), int64(len(raw)))
}

func TestParseAndFind(t *testing.T) {
	af, err := parse(t, sample().Build())
	require.NoError(t, err)
	assert.Equal(t, "6000.3.9f1", af.UnityVersion)
	require.Len(t, af.Objects, 3)
	assert.Equal(t, int32(49), af.Objects[1].ClassID)

	raw, err := af.Read(af.Objects[1])
	require.NoError(t, err)
	assert.Equal(t, "text asset", string(raw))

	found, err := af.FindMonoBehaviours(func(n string) bool { return n == "GameBalance" || n == "lng_ru" })
	require.NoError(t, err)
	require.Len(t, found, 2)
	name, off, err := MonoBehaviourName(found["GameBalance"])
	require.NoError(t, err)
	assert.Equal(t, "GameBalance", name)
	assert.Equal(t, "items", string(found["GameBalance"][off:]))
}

func TestParseSkipsTypeTrees(t *testing.T) {
	b := sample()
	b.TypeTree = 1
	af, err := parse(t, b.Build())
	require.NoError(t, err)
	require.Len(t, af.Objects, 3)
	assert.Equal(t, int32(49), af.Objects[1].ClassID)
	raw, err := af.Read(af.Objects[1])
	require.NoError(t, err)
	assert.Equal(t, "text asset", string(raw))
}

func TestClassIDs(t *testing.T) {
	for _, tree := range []byte{0, 1} {
		b := sample()
		b.TypeTree = tree
		ids, err := ClassIDs(b.Build())
		require.NoError(t, err)
		assert.Equal(t, []int32{ClassMonoBehavour, 49}, ids)
	}
	_, err := ClassIDs([]byte("short"))
	require.ErrorIs(t, err, ErrCorrupt)
}

func TestParseRejects(t *testing.T) {
	good := sample().Build()
	tests := []struct {
		name string
		raw  []byte
		want error
	}{
		{"old version", func() []byte { b := sample(); b.Version = 17; return b.Build() }(), ErrUnsupported},
		{"size mismatch", append(append([]byte{}, good...), 0), ErrCorrupt},
		{"truncated", good[:40], ErrCorrupt},
		{"object offset overflow", func() []byte {
			raw := sample().Build()
			objStart := bytes.LastIndex(raw[:len(raw)-40], []byte{3, 0, 0, 0, 0, 0, 0, 0}) + 8
			binary.LittleEndian.PutUint64(raw[objStart:], 0x7fffffffffffff00)
			return raw
		}(), ErrCorrupt},
		{"bad type index", func() []byte { b := sample(); b.BadType = true; return b.Build() }(), ErrCorrupt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(t, tt.raw)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestDuplicateNamesAreRejected(t *testing.T) {
	b := sample()
	b.Objects = append(b.Objects, unitytest.Object{PathID: 4, Class: ClassMonoBehavour, Data: unitytest.Mono("lng_ru", nil)})
	af, err := parse(t, b.Build())
	require.NoError(t, err)
	_, err = af.FindMonoBehaviours(func(n string) bool { return n == "lng_ru" })
	require.ErrorIs(t, err, ErrCorrupt)
}

func TestMonoBehaviourName(t *testing.T) {
	_, _, err := MonoBehaviourName([]byte{1, 2})
	require.ErrorIs(t, err, ErrCorrupt)
	bad := unitytest.Mono("x", nil)
	binary.LittleEndian.PutUint32(bad[monoHeader:], 1000)
	_, _, err = MonoBehaviourName(bad)
	require.ErrorIs(t, err, ErrCorrupt)
}

func TestOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resources.assets")
	require.NoError(t, os.WriteFile(path, sample().Build(), 0o600))
	af, f, err := Open(path)
	require.NoError(t, err)
	defer f.Close()
	assert.Len(t, af.Objects, 3)
	_, _, err = Open(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
	require.NoError(t, os.WriteFile(path, []byte("short"), 0o600))
	_, _, err = Open(path)
	require.ErrorIs(t, err, ErrCorrupt)
}

func FuzzParse(f *testing.F) {
	f.Add(sample().Build())
	f.Fuzz(func(t *testing.T, raw []byte) {
		af, err := Parse(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return
		}
		for _, o := range af.Objects {
			if o.Offset < 0 || o.Offset+int64(o.Size) > int64(len(raw)) {
				t.Fatalf("object %d out of bounds", o.PathID)
			}
		}
		_, _ = af.FindMonoBehaviours(func(string) bool { return true })
	})
}
