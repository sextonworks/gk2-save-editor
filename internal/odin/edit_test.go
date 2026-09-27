package odin

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sextonworks/gk2-save-editor/internal/odin/odintest"
)

func parsed(t *testing.T) ([]byte, *Node) {
	t.Helper()
	data := odintest.BuildSave(odintest.DefaultSpec())
	root, err := Parse(data)
	require.NoError(t, err)
	return data, root
}

func firstBagItem(t *testing.T, root *Node) *Node {
	t.Helper()
	arr, ok := child(t, root.Children[0], "playerData", "inventory", "inventoryItem", "inventory").Array()
	require.True(t, ok)
	return arr.Children[0]
}

func TestSetIntRanges(t *testing.T) {
	tests := []struct {
		wire Wire
		v    int64
		ok   bool
	}{
		{WireInt8, -128, true}, {WireInt8, 128, false},
		{WireUint8, 255, true}, {WireUint8, -1, false},
		{WireInt16, -32768, true}, {WireInt16, 40000, false},
		{WireUint16, 65535, true}, {WireUint16, 70000, false},
		{WireInt32, math.MaxInt32, true}, {WireInt32, math.MaxInt32 + 1, false},
		{WireUint32, math.MaxUint32, true}, {WireUint32, -1, false},
		{WireInt64, math.MinInt64, true},
		{WireUint64, 5, true}, {WireUint64, -5, false},
	}
	for _, tt := range tests {
		buf := make([]byte, 8)
		n := &Node{Wire: tt.wire}
		err := SetInt(buf, n, tt.v)
		if tt.ok {
			require.NoError(t, err, "%v %d", tt.wire, tt.v)
			got, ok := n.Integer()
			require.True(t, ok)
			assert.Equal(t, tt.v, got)
		} else {
			require.ErrorIs(t, err, ErrOutOfRange, "%v %d", tt.wire, tt.v)
		}
	}
	require.ErrorIs(t, SetInt(make([]byte, 8), &Node{Wire: WireString}, 1), ErrWire)
}

func TestSetIntAndFloatRoundTrip(t *testing.T) {
	data, root := parsed(t)
	count := child(t, firstBagItem(t, root), "count")
	require.NoError(t, SetInt(data, count, 7))
	resArr, _ := child(t, root.Children[0], "playerData", "res", "resValues").Array()
	money := child(t, resArr.Children[0], "value")
	require.NoError(t, SetFloat(data, money, 5000))
	require.ErrorIs(t, SetFloat(data, count, 1), ErrWire)
	require.ErrorIs(t, SetFloat(data, money, math.MaxFloat64), ErrOutOfRange)

	root2, err := Validate(data)
	require.NoError(t, err)
	assert.Equal(t, int64(7), child(t, firstBagItem(t, root2), "count").Int)
	resArr2, _ := child(t, root2.Children[0], "playerData", "res", "resValues").Array()
	assert.InDelta(t, 5000, child(t, resArr2.Children[0], "value").Float, 1e-6)
}

func TestSetStringChangesLength(t *testing.T) {
	tests := []string{"x", "candle_master", "свеча", "蜡烛", ""}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			data, root := parsed(t)
			out, err := SetString(data, child(t, firstBagItem(t, root), "id"), s)
			require.NoError(t, err)
			root2, err := Validate(out)
			require.NoError(t, err)
			assert.Equal(t, s, child(t, firstBagItem(t, root2), "id").Str)
			resArr, _ := child(t, root2.Children[0], "playerData", "res", "resValues").Array()
			assert.InDelta(t, 861, child(t, resArr.Children[0], "value").Float, 1e-6)
		})
	}
}

func TestEncodeStringKeepsLatin1WhenPossible(t *testing.T) {
	assert.Equal(t, byte(0), EncodeString(0, "abc")[0])
	assert.Equal(t, byte(1), EncodeString(0, "свеча")[0])
	assert.Equal(t, byte(1), EncodeString(1, "abc")[0])
}

func TestValidateCatchesBrokenInvariants(t *testing.T) {
	data, root := parsed(t)
	arr, _ := child(t, root.Children[0], "playerData", "inventory", "inventoryItem", "inventory").Array()
	require.NoError(t, SetArrayLen(data, arr, 9))
	_, err := Validate(data)
	require.ErrorIs(t, err, ErrInvalid)

	data, root = parsed(t)
	items, _ := child(t, root.Children[0], "playerData", "inventory", "inventoryItem", "inventory").Array()
	second := items.Children[1]
	copy(data[second.RefIDOff:], data[items.Children[0].RefIDOff:items.Children[0].RefIDOff+4])
	_, err = Validate(data)
	require.ErrorIs(t, err, ErrInvalid)

	require.ErrorIs(t, SetArrayLen(data, second, 1), ErrWire)
	_, err = SetString(data, second, "x")
	require.ErrorIs(t, err, ErrWire)
	assert.Positive(t, MaxRefID(root))
}
