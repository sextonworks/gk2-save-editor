package odin

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf16"
)

var (
	ErrWire       = errors.New("odin: wrong value type")
	ErrOutOfRange = errors.New("odin: value out of range")
	ErrInvalid    = errors.New("odin: invalid stream")
)

type intRange struct {
	min, max int64
	size     int
}

var intRanges = map[Wire]intRange{
	WireInt8:   {math.MinInt8, math.MaxInt8, 1},
	WireUint8:  {0, math.MaxUint8, 1},
	WireInt16:  {math.MinInt16, math.MaxInt16, 2},
	WireUint16: {0, math.MaxUint16, 2},
	WireInt32:  {math.MinInt32, math.MaxInt32, 4},
	WireUint32: {0, math.MaxUint32, 4},
	WireInt64:  {math.MinInt64, math.MaxInt64, 8},
	WireUint64: {0, math.MaxInt64, 8},
}

func SetInt(data []byte, n *Node, v int64) error {
	r, ok := intRanges[n.Wire]
	if !ok {
		return fmt.Errorf("%s: %w", n.Path(), ErrWire)
	}
	if v < r.min || v > r.max {
		return fmt.Errorf("%s: %d: %w", n.Path(), v, ErrOutOfRange)
	}
	b := data[n.ValueOff:]
	switch r.size {
	case 1:
		b[0] = byte(v)
	case 2:
		binary.LittleEndian.PutUint16(b, uint16(v))
	case 4:
		binary.LittleEndian.PutUint32(b, uint32(v))
	default:
		binary.LittleEndian.PutUint64(b, uint64(v))
	}
	n.Int, n.Uint = v, uint64(v)
	return nil
}

func SetFloat(data []byte, n *Node, v float64) error {
	switch n.Wire {
	case WireFloat32:
		if math.Abs(v) > math.MaxFloat32 {
			return ErrOutOfRange
		}
		binary.LittleEndian.PutUint32(data[n.ValueOff:], math.Float32bits(float32(v)))
		n.Float = float64(float32(v))
	case WireFloat64:
		binary.LittleEndian.PutUint64(data[n.ValueOff:], math.Float64bits(v))
		n.Float = v
	default:
		return fmt.Errorf("%s: %w", n.Path(), ErrWire)
	}
	return nil
}

func EncodeString(flag byte, s string) []byte {
	latin := flag == 0
	for _, r := range s {
		if r > 0xff {
			latin = false
			break
		}
	}
	if latin {
		runes := []rune(s)
		out := make([]byte, 0, 5+len(runes))
		out = append(out, 0)
		out = binary.LittleEndian.AppendUint32(out, uint32(len(runes)))
		for _, r := range runes {
			out = append(out, byte(r))
		}
		return out
	}
	u := utf16.Encode([]rune(s))
	out := make([]byte, 0, 5+2*len(u))
	out = append(out, 1)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(u)))
	for _, c := range u {
		out = binary.LittleEndian.AppendUint16(out, c)
	}
	return out
}

func StringSpan(data []byte, n *Node) (start, end int, err error) {
	if n.Wire != WireString {
		return 0, 0, fmt.Errorf("%s: %w", n.Path(), ErrWire)
	}
	start = n.ValueOff
	flag := data[start]
	count := int(binary.LittleEndian.Uint32(data[start+1:]))
	width := 1
	if flag == 1 {
		width = 2
	}
	return start, start + 5 + count*width, nil
}

func SetString(data []byte, n *Node, s string) ([]byte, error) {
	start, end, err := StringSpan(data, n)
	if err != nil {
		return nil, err
	}
	return Splice(data, start, end, EncodeString(data[start], s)), nil
}

func Splice(data []byte, start, end int, repl []byte) []byte {
	out := make([]byte, 0, len(data)-(end-start)+len(repl))
	out = append(out, data[:start]...)
	out = append(out, repl...)
	return append(out, data[end:]...)
}

func SetArrayLen(data []byte, arr *Node, n int64) error {
	if arr.Kind != KindArray {
		return fmt.Errorf("%s: %w", arr.Path(), ErrWire)
	}
	binary.LittleEndian.PutUint64(data[arr.ValueOff:], uint64(n))
	arr.ArrayLen = n
	return nil
}

func MaxRefID(root *Node) int32 {
	var m int32
	Walk(root, func(n *Node) bool {
		if n.HasRefID && n.RefID > m {
			m = n.RefID
		}
		return true
	})
	return m
}

func Validate(data []byte) (*Node, error) {
	root, err := Parse(data)
	if err != nil {
		return nil, err
	}
	seen := map[int32]bool{}
	var bad error
	Walk(root, func(n *Node) bool {
		if n.HasRefID {
			if seen[n.RefID] {
				bad = fmt.Errorf("%w: duplicate reference id %d at %s", ErrInvalid, n.RefID, n.Path())
				return false
			}
			seen[n.RefID] = true
		}
		if n.Kind == KindArray && n.ArrayLen != int64(len(n.Children)) {
			bad = fmt.Errorf("%w: array %s declares %d items, has %d", ErrInvalid, n.Path(), n.ArrayLen, len(n.Children))
			return false
		}
		return true
	})
	if bad != nil {
		return nil, bad
	}
	return root, nil
}
