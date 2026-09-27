package odin

import (
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/odin/odintest"
)

func child(t *testing.T, n *Node, names ...string) *Node {
	t.Helper()
	cur := n
	for _, name := range names {
		next, ok := cur.Child(name)
		require.True(t, ok, "no child %q under %q", name, cur.Path())
		cur = next
	}
	return cur
}

func TestParseSyntheticSave(t *testing.T) {
	root, err := Parse(odintest.BuildSave(odintest.DefaultSpec()))
	require.NoError(t, err)
	require.Len(t, root.Children, 1)
	game := root.Children[0]
	assert.Equal(t, "GameSave, Assembly-CSharp", game.TypeName)
	assert.Equal(t, "1.006", child(t, game, "gameSaveVersion").Str)

	bag := child(t, game, "playerData", "inventory", "inventoryItem", "inventory")
	arr, ok := bag.Array()
	require.True(t, ok)
	assert.Equal(t, int64(4), arr.ArrayLen)
	require.Len(t, arr.Children, 4)
	first := arr.Children[0]
	assert.Equal(t, "faith", child(t, first, "id").Str)
	assert.Equal(t, WireInt32, child(t, first, "count").Wire)
	assert.Equal(t, int64(99), child(t, first, "count").Int)
	assert.Equal(t, "playerData/inventory/inventoryItem/inventory/#/#/id", child(t, first, "id").Path()[2:])

	resArr, ok := child(t, game, "playerData", "res", "resValues").Array()
	require.True(t, ok)
	money := child(t, resArr.Children[0], "value")
	assert.Equal(t, WireFloat32, money.Wire)
	assert.InDelta(t, 861, money.Float, 1e-6)
}

func TestParseOffsetsAreConsistent(t *testing.T) {
	data := odintest.BuildSave(odintest.DefaultSpec())
	root, err := Parse(data)
	require.NoError(t, err)
	ids := map[int32]bool{}
	Walk(root, func(n *Node) bool {
		if n.Kind == KindRoot {
			return true
		}
		assert.Greater(t, n.End, n.Off, n.Path())
		assert.LessOrEqual(t, n.End, len(data), n.Path())
		if n.Parent != nil && n.Parent.Kind != KindRoot {
			assert.GreaterOrEqual(t, n.Off, n.Parent.Off)
			assert.LessOrEqual(t, n.End, n.Parent.End)
		}
		if n.HasRefID {
			assert.False(t, ids[n.RefID], "duplicate ref id %d", n.RefID)
			ids[n.RefID] = true
		}
		return true
	})
}

func TestParseErrors(t *testing.T) {
	good := odintest.BuildSave(odintest.DefaultSpec())
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"truncated", good[:len(good)-1], ErrTruncated},
		{"cut in string", good[:12], ErrTruncated},
		{"unknown entry", []byte{0x99}, ErrMalformed},
		{"unbalanced end", []byte{0x05}, ErrMalformed},
		{"bad string flag", []byte{0x27, 0x07, 0x00, 0x00, 0x00, 0x00}, ErrMalformed},
		{"negative array", []byte{0x06, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, ErrMalformed},
		{"end array closes ref", []byte{0x02, 0x2e, 0x01, 0x00, 0x00, 0x00, 0x07}, ErrMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.data)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestParseValueWires(t *testing.T) {
	data := []byte{
		0x0f, 0x00, 0x01, 0x00, 0x00, 0x00, 'a', 0xfe,
		0x2b, 0x00, 0x01, 0x00, 0x00, 0x00, 'b', 0x01,
		0x2d, 0x00, 0x01, 0x00, 0x00, 0x00, 'c',
		0x23, 0x00, 0x01, 0x00, 0x00, 0x00, 'd', 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		0x27, 0x00, 0x01, 0x00, 0x00, 0x00, 'e', 0x00, 0x02, 0x00, 0x00, 0x00, 'h', 'i',
		0x31,
	}
	root, err := Parse(data)
	require.NoError(t, err)
	require.Len(t, root.Children, 5)
	assert.Equal(t, int64(-2), root.Children[0].Int)
	assert.True(t, root.Children[1].Bool)
	assert.Equal(t, WireNull, root.Children[2].Wire)
	assert.Equal(t, WireDecimal, root.Children[3].Wire)
	assert.Equal(t, "hi", root.Children[4].Str)
}

func named(code byte, payload ...byte) []byte {
	return append([]byte{code, 0x00, 0x01, 0x00, 0x00, 0x00, 'v'}, payload...)
}

func TestParseAllValueWires(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		check func(t *testing.T, n *Node)
	}{
		{"uint8", named(0x11, 0xfe), func(t *testing.T, n *Node) { assert.Equal(t, uint64(254), n.Uint) }},
		{"int16", named(0x13, 0xfe, 0xff), func(t *testing.T, n *Node) { assert.Equal(t, int64(-2), n.Int) }},
		{"uint16", named(0x15, 0xfe, 0xff), func(t *testing.T, n *Node) { assert.Equal(t, uint64(65534), n.Uint) }},
		{"uint32", named(0x19, 0xff, 0xff, 0xff, 0xff), func(t *testing.T, n *Node) { assert.Equal(t, uint64(math.MaxUint32), n.Uint) }},
		{"int64", named(0x1b, 0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff), func(t *testing.T, n *Node) { assert.Equal(t, int64(-2), n.Int) }},
		{"unnamed uint64", []byte{0x1e, 7, 0, 0, 0, 0, 0, 0, 0}, func(t *testing.T, n *Node) { assert.Equal(t, uint64(7), n.Uint) }},
		{"float64", named(0x21, 0, 0, 0, 0, 0, 0, 0xf8, 0x3f), func(t *testing.T, n *Node) { assert.InDelta(t, 1.5, n.Float, 1e-12) }},
		{"char", named(0x25, 'z', 0), func(t *testing.T, n *Node) { assert.Equal(t, "z", n.Str) }},
		{"internal ref", named(0x09, 3, 0, 0, 0), func(t *testing.T, n *Node) {
			assert.Equal(t, WireInternalRef, n.Wire)
			assert.Equal(t, int64(3), n.Int)
		}},
		{"external ref index", []byte{0x0c, 4, 0, 0, 0}, func(t *testing.T, n *Node) { assert.Equal(t, WireExternalRef, n.Wire) }},
		{"guid", named(0x29, make([]byte, 16)...), func(t *testing.T, n *Node) { assert.Len(t, n.Raw, 16) }},
		{"primitive array", []byte{0x08, 2, 0, 0, 0, 4, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8}, func(t *testing.T, n *Node) {
			assert.Equal(t, KindPrimitiveArray, n.Kind)
			assert.Equal(t, int64(2), n.ArrayLen)
			assert.Len(t, n.Raw, 8)
		}},
		{"struct", []byte{0x04, 0x2e, 0x05}, func(t *testing.T, n *Node) {
			assert.Equal(t, KindStruct, n.Kind)
			assert.False(t, n.HasRefID)
		}},
		{"latin1 string", []byte{0x28, 0x00, 0x02, 0x00, 0x00, 0x00, 0xe9, 'a'}, func(t *testing.T, n *Node) { assert.Equal(t, "éa", n.Str) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := Parse(tt.data)
			require.NoError(t, err)
			require.Len(t, root.Children, 1)
			tt.check(t, root.Children[0])
		})
	}
}

func TestParseTruncatedEverywhere(t *testing.T) {
	data := odintest.BuildSave(odintest.DefaultSpec())
	for cut := 1; cut < 400; cut++ {
		_, err := Parse(data[:cut])
		require.Error(t, err, "cut at %d", cut)
	}
	_, err := Parse([]byte{0x08, 0xff, 0xff, 0xff, 0x7f, 0x01, 0, 0, 0})
	require.ErrorIs(t, err, ErrTruncated)
	_, err = Parse([]byte{0x02, 0x99})
	require.ErrorIs(t, err, ErrMalformed)
}

func TestInteger(t *testing.T) {
	tests := []struct {
		name string
		node Node
		want int64
		ok   bool
	}{
		{"int32", Node{Wire: WireInt32, Int: -5}, -5, true},
		{"uint64", Node{Wire: WireUint64, Uint: 2}, 2, true},
		{"uint64 overflow", Node{Wire: WireUint64, Uint: math.MaxUint64}, 0, false},
		{"string", Node{Wire: WireString, Str: "1"}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.node.Integer()
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.ok, ok)
		})
	}
}

func FuzzParse(f *testing.F) {
	f.Add(odintest.BuildSave(odintest.DefaultSpec()))
	f.Add([]byte{0x02, 0x2e, 0x01, 0x00, 0x00, 0x00, 0x05})
	f.Fuzz(func(t *testing.T, data []byte) {
		root, err := Parse(data)
		if err != nil {
			return
		}
		Walk(root, func(n *Node) bool {
			if n.End > len(data) || n.Off > len(data) {
				t.Fatalf("node %q out of bounds", n.Path())
			}
			return true
		})
	})
}

func TestDumpRealSave(t *testing.T) {
	path, out := os.Getenv("GK2_REAL_SAVE"), os.Getenv("GK2_DUMP_OUT")
	if path == "" || out == "" {
		t.Skip("set GK2_REAL_SAVE and GK2_DUMP_OUT to dump a real save")
	}
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	root, err := Parse(data)
	require.NoError(t, err)
	var sb strings.Builder
	Walk(root, func(n *Node) bool {
		if n.Kind == KindRoot {
			return true
		}
		switch n.Kind {
		case KindRef, KindStruct:
			fmt.Fprintf(&sb, "%s\tnode\t%s\t%d\n", n.Path(), n.TypeName, n.RefID)
		case KindArray:
			fmt.Fprintf(&sb, "%s\tarray\t%d\n", n.Path(), n.ArrayLen)
		case KindPrimitiveArray:
			fmt.Fprintf(&sb, "%s\tprim\t%d\n", n.Path(), n.ArrayLen)
		default:
			fmt.Fprintf(&sb, "%s\tval\t%s\n", n.Path(), dumpValue(n))
		}
		return true
	})
	require.NoError(t, os.WriteFile(out, []byte(sb.String()), 0o600))
}

func dumpValue(n *Node) string {
	switch n.Wire {
	case WireString, WireChar:
		return n.Str
	case WireBool:
		return strconv.FormatBool(n.Bool)
	case WireNull:
		return "null"
	case WireFloat32, WireFloat64:
		return strconv.FormatFloat(n.Float, 'g', 9, 64)
	case WireUint8, WireUint16, WireUint32, WireUint64:
		return strconv.FormatUint(n.Uint, 10)
	case WireGUID, WireDecimal:
		return hex.EncodeToString(n.Raw)
	case WireInternalRef, WireExternalRef:
		return "ref:" + strconv.FormatInt(n.Int, 10)
	}
	return strconv.FormatInt(n.Int, 10)
}
