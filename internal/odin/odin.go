package odin

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf16"
)

const (
	entryNamedRefStart      = 0x01
	entryUnnamedRefStart    = 0x02
	entryNamedStructStart   = 0x03
	entryUnnamedStructStart = 0x04
	entryEndOfNode          = 0x05
	entryStartOfArray       = 0x06
	entryEndOfArray         = 0x07
	entryPrimitiveArray     = 0x08
	entryNamedInternalRef   = 0x09
	entryUnnamedInternalRef = 0x0a
	entryNamedExtRefIndex   = 0x0b
	entryUnnamedExtRefIndex = 0x0c
	entryNamedExtRefGUID    = 0x0d
	entryUnnamedExtRefGUID  = 0x0e
	entryNamedSByte         = 0x0f
	entryLastPrimitive      = 0x22
	entryNamedDecimal       = 0x23
	entryUnnamedDecimal     = 0x24
	entryNamedChar          = 0x25
	entryUnnamedChar        = 0x26
	entryNamedString        = 0x27
	entryUnnamedString      = 0x28
	entryNamedGUID          = 0x29
	entryUnnamedGUID        = 0x2a
	entryNamedBool          = 0x2b
	entryUnnamedBool        = 0x2c
	entryNamedNull          = 0x2d
	entryUnnamedNull        = 0x2e
	entryTypeName           = 0x2f
	entryTypeID             = 0x30
	entryEndOfStream        = 0x31
	entryNamedExtRefString  = 0x32
	entryUnnamedExtRefStr   = 0x33
)

var (
	ErrTruncated = errors.New("odin: unexpected end of data")
	ErrMalformed = errors.New("odin: malformed data")
)

type Kind uint8

const (
	KindRoot Kind = iota
	KindRef
	KindStruct
	KindArray
	KindPrimitiveArray
	KindValue
)

type Wire uint8

const (
	WireNone Wire = iota
	WireInt8
	WireUint8
	WireInt16
	WireUint16
	WireInt32
	WireUint32
	WireInt64
	WireUint64
	WireFloat32
	WireFloat64
	WireDecimal
	WireChar
	WireString
	WireGUID
	WireBool
	WireNull
	WireInternalRef
	WireExternalRef
)

var primitiveWire = map[byte]Wire{
	0x0f: WireInt8, 0x11: WireUint8, 0x13: WireInt16, 0x15: WireUint16, 0x17: WireInt32,
	0x19: WireUint32, 0x1b: WireInt64, 0x1d: WireUint64, 0x1f: WireFloat32, 0x21: WireFloat64,
}

var wireSize = map[Wire]int{
	WireInt8: 1, WireUint8: 1, WireInt16: 2, WireUint16: 2, WireInt32: 4,
	WireUint32: 4, WireInt64: 8, WireUint64: 8, WireFloat32: 4, WireFloat64: 8,
}

type Node struct {
	Name     string
	Named    bool
	Kind     Kind
	Wire     Wire
	TypeName string
	Str      string
	Int      int64
	Uint     uint64
	Float    float64
	Bool     bool
	Raw      []byte
	ArrayLen int64
	RefID    int32
	HasRefID bool
	Children []*Node
	Parent   *Node

	Off      int
	ValueOff int
	RefIDOff int
	End      int
}

func (n *Node) Path() string {
	parts := make([]string, 0, 8)
	for c := n; c != nil && c.Parent != nil; c = c.Parent {
		if c.Named {
			parts = append(parts, c.Name)
		} else {
			parts = append(parts, "#")
		}
	}
	out := make([]byte, 0, 64)
	for i := len(parts) - 1; i >= 0; i-- {
		out = append(out, parts[i]...)
		if i > 0 {
			out = append(out, '/')
		}
	}
	return string(out)
}
func (n *Node) Integer() (int64, bool) {
	switch n.Wire {
	case WireInt8, WireInt16, WireInt32, WireInt64:
		return n.Int, true
	case WireUint8, WireUint16, WireUint32, WireUint64:
		if n.Uint > math.MaxInt64 {
			return 0, false
		}
		return int64(n.Uint), true
	}
	return 0, false
}

func (n *Node) Child(name string) (*Node, bool) {
	for _, c := range n.Children {
		if c.Named && c.Name == name {
			return c, true
		}
	}
	return nil, false
}
func (n *Node) Array() (*Node, bool) {
	for _, c := range n.Children {
		if c.Kind == KindArray {
			return c, true
		}
	}
	return nil, false
}
func Walk(n *Node, fn func(*Node) bool) bool {
	if !fn(n) {
		return false
	}
	for _, c := range n.Children {
		if !Walk(c, fn) {
			return false
		}
	}
	return true
}

type reader struct {
	data  []byte
	pos   int
	types map[int32]string
}

func Parse(data []byte) (*Node, error) {
	r := &reader{data: data, types: map[int32]string{}}
	root := &Node{Kind: KindRoot, End: len(data)}
	stack := []*Node{root}
	for r.pos < len(data) {
		off := r.pos
		entry := data[r.pos]
		r.pos++
		if entry == entryEndOfStream {
			break
		}
		cur := stack[len(stack)-1]
		switch entry {
		case entryEndOfNode, entryEndOfArray:
			if len(stack) == 1 {
				return nil, fmt.Errorf("%w: unbalanced end at %d", ErrMalformed, off)
			}
			want := KindArray
			if entry == entryEndOfNode {
				want = KindRef
			}
			top := stack[len(stack)-1]
			if (want == KindArray) != (top.Kind == KindArray) {
				return nil, fmt.Errorf("%w: mismatched end at %d", ErrMalformed, off)
			}
			top.End = r.pos
			stack = stack[:len(stack)-1]
			continue
		}
		n := &Node{Off: off, Parent: cur}
		if isNamed(entry) {
			name, err := r.str()
			if err != nil {
				return nil, fmt.Errorf("name at %d: %w", off, err)
			}
			n.Name, n.Named = name, true
		}
		if err := r.body(entry, n); err != nil {
			return nil, fmt.Errorf("entry 0x%02x at %d: %w", entry, off, err)
		}
		cur.Children = append(cur.Children, n)
		if n.Kind == KindRef || n.Kind == KindStruct || n.Kind == KindArray {
			stack = append(stack, n)
		} else {
			n.End = r.pos
		}
	}
	if len(stack) != 1 {
		return nil, fmt.Errorf("%w: %d unclosed nodes", ErrTruncated, len(stack)-1)
	}
	return root, nil
}

func isNamed(entry byte) bool {
	switch entry {
	case entryNamedRefStart, entryNamedStructStart, entryNamedInternalRef, entryNamedExtRefIndex,
		entryNamedExtRefGUID, entryNamedChar, entryNamedString, entryNamedGUID, entryNamedBool,
		entryNamedNull, entryNamedExtRefString:
		return true
	}
	return entry >= entryNamedSByte && entry <= entryNamedDecimal && entry%2 == 1
}

func (r *reader) body(entry byte, n *Node) error {
	switch entry {
	case entryNamedRefStart, entryUnnamedRefStart, entryNamedStructStart, entryUnnamedStructStart:
		n.Kind = KindRef
		if entry == entryNamedStructStart || entry == entryUnnamedStructStart {
			n.Kind = KindStruct
		}
		t, err := r.typeEntry()
		if err != nil {
			return err
		}
		n.TypeName = t
		if n.Kind == KindRef {
			n.RefIDOff = r.pos
			id, err := r.i32()
			if err != nil {
				return err
			}
			n.RefID, n.HasRefID = id, true
		}
		return nil
	case entryStartOfArray:
		n.Kind = KindArray
		n.ValueOff = r.pos
		v, err := r.i64()
		if err != nil {
			return err
		}
		if v < 0 {
			return fmt.Errorf("%w: negative array length", ErrMalformed)
		}
		n.ArrayLen = v
		return nil
	case entryPrimitiveArray:
		n.Kind = KindPrimitiveArray
		count, err := r.i32()
		if err != nil {
			return err
		}
		size, err := r.i32()
		if err != nil {
			return err
		}
		if count < 0 || size < 0 || int64(count)*int64(size) > int64(len(r.data)-r.pos) {
			return ErrTruncated
		}
		n.ValueOff = r.pos
		n.ArrayLen = int64(count)
		n.Raw = r.data[r.pos : r.pos+int(count)*int(size)]
		r.pos += int(count) * int(size)
		return nil
	}
	n.Kind = KindValue
	n.ValueOff = r.pos
	return r.value(entry, n)
}

func (r *reader) value(entry byte, n *Node) error {
	if entry >= entryNamedSByte && entry <= entryLastPrimitive {
		base := entry
		if base%2 == 0 {
			base--
		}
		n.Wire = primitiveWire[base]
		return r.primitive(n)
	}
	switch entry {
	case entryNamedInternalRef, entryUnnamedInternalRef, entryNamedExtRefIndex, entryUnnamedExtRefIndex:
		n.Wire = WireInternalRef
		if entry >= entryNamedExtRefIndex {
			n.Wire = WireExternalRef
		}
		v, err := r.i32()
		n.Int = int64(v)
		return err
	case entryNamedExtRefGUID, entryUnnamedExtRefGUID, entryNamedGUID, entryUnnamedGUID, entryNamedDecimal,
		entryUnnamedDecimal:
		n.Wire = WireGUID
		if entry == entryNamedDecimal || entry == entryUnnamedDecimal {
			n.Wire = WireDecimal
		}
		b, err := r.take(16)
		n.Raw = b
		return err
	case entryNamedChar, entryUnnamedChar:
		n.Wire = WireChar
		b, err := r.take(2)
		if err == nil {
			n.Str = string(utf16.Decode([]uint16{binary.LittleEndian.Uint16(b)}))
		}
		return err
	case entryNamedString, entryUnnamedString, entryNamedExtRefString, entryUnnamedExtRefStr:
		n.Wire = WireString
		s, err := r.str()
		n.Str = s
		return err
	case entryNamedBool, entryUnnamedBool:
		n.Wire = WireBool
		b, err := r.take(1)
		if err == nil {
			n.Bool = b[0] != 0
		}
		return err
	case entryNamedNull, entryUnnamedNull:
		n.Wire = WireNull
		return nil
	}
	return fmt.Errorf("%w: unknown entry", ErrMalformed)
}

func (r *reader) primitive(n *Node) error {
	size, ok := wireSize[n.Wire]
	if !ok {
		return fmt.Errorf("%w: unknown primitive", ErrMalformed)
	}
	b, err := r.take(size)
	if err != nil {
		return err
	}
	switch n.Wire {
	case WireInt8:
		n.Int = int64(int8(b[0]))
	case WireUint8:
		n.Uint = uint64(b[0])
	case WireInt16:
		n.Int = int64(int16(binary.LittleEndian.Uint16(b)))
	case WireUint16:
		n.Uint = uint64(binary.LittleEndian.Uint16(b))
	case WireInt32:
		n.Int = int64(int32(binary.LittleEndian.Uint32(b)))
	case WireUint32:
		n.Uint = uint64(binary.LittleEndian.Uint32(b))
	case WireInt64:
		n.Int = int64(binary.LittleEndian.Uint64(b))
	case WireUint64:
		n.Uint = binary.LittleEndian.Uint64(b)
	case WireFloat32:
		n.Float = float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))
	case WireFloat64:
		n.Float = math.Float64frombits(binary.LittleEndian.Uint64(b))
	}
	return nil
}

func (r *reader) take(n int) ([]byte, error) {
	if n < 0 || n > len(r.data)-r.pos {
		return nil, ErrTruncated
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

func (r *reader) i32() (int32, error) {
	b, err := r.take(4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(b)), nil
}

func (r *reader) i64() (int64, error) {
	b, err := r.take(8)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b)), nil
}

func (r *reader) str() (string, error) {
	flag, err := r.take(1)
	if err != nil {
		return "", err
	}
	n, err := r.i32()
	if err != nil {
		return "", err
	}
	if n < 0 {
		return "", fmt.Errorf("%w: negative string length", ErrMalformed)
	}
	switch flag[0] {
	case 0:
		b, err := r.take(int(n))
		if err != nil {
			return "", err
		}
		runes := make([]rune, len(b))
		for i, c := range b {
			runes[i] = rune(c)
		}
		return string(runes), nil
	case 1:
		if int64(n)*2 > int64(len(r.data)-r.pos) {
			return "", ErrTruncated
		}
		b, _ := r.take(int(n) * 2)
		u := make([]uint16, n)
		for i := range u {
			u[i] = binary.LittleEndian.Uint16(b[i*2:])
		}
		return string(utf16.Decode(u)), nil
	}
	return "", fmt.Errorf("%w: string flag %d", ErrMalformed, flag[0])
}

func (r *reader) typeEntry() (string, error) {
	b, err := r.take(1)
	if err != nil {
		return "", err
	}
	switch b[0] {
	case entryTypeName:
		id, err := r.i32()
		if err != nil {
			return "", err
		}
		s, err := r.str()
		if err != nil {
			return "", err
		}
		r.types[id] = s
		return s, nil
	case entryTypeID:
		id, err := r.i32()
		if err != nil {
			return "", err
		}
		return r.types[id], nil
	case entryUnnamedNull:
		return "", nil
	}
	return "", fmt.Errorf("%w: type entry 0x%02x", ErrMalformed, b[0])
}
