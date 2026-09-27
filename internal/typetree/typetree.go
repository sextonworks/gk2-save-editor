package typetree

import (
	"embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

const alignFlag = 0x4000

var (
	ErrLayout    = errors.New("object layout does not match the schema")
	ErrTruncated = errors.New("object data is truncated")
	ErrSchema    = errors.New("invalid schema")
)

//go:embed schema/*.json
var schemas embed.FS

type Node struct {
	Type     string
	Name     string
	Align    bool
	Children []*Node
}

func (n *Node) arrayElem() (*Node, bool) {
	if len(n.Children) == 0 || n.Children[0].Type != "Array" || len(n.Children[0].Children) != 2 {
		return nil, false
	}
	return n.Children[0].Children[1], true
}

func Load(name string) (*Node, error) {
	raw, err := schemas.ReadFile("schema/" + name + ".json")
	if err != nil {
		return nil, fmt.Errorf("schema %s: %w", name, err)
	}
	var rows [][4]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("schema %s: %w", name, err)
	}
	return build(rows)
}

func build(rows [][4]any) (*Node, error) {
	var root *Node
	type frame struct {
		level int
		node  *Node
	}
	var stack []frame
	for i, r := range rows {
		level, ok1 := r[0].(float64)
		typ, ok2 := r[1].(string)
		name, ok3 := r[2].(string)
		flags, ok4 := r[3].(float64)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return nil, fmt.Errorf("%w: row %d", ErrSchema, i)
		}
		n := &Node{Type: typ, Name: name, Align: int(flags)&alignFlag != 0}
		for len(stack) > 0 && stack[len(stack)-1].level >= int(level) {
			stack = stack[:len(stack)-1]
		}
		switch {
		case len(stack) > 0:
			parent := stack[len(stack)-1].node
			parent.Children = append(parent.Children, n)
		case root == nil:
			root = n
		default:
			return nil, fmt.Errorf("%w: second root at row %d", ErrSchema, i)
		}
		stack = append(stack, frame{int(level), n})
	}
	if root == nil {
		return nil, fmt.Errorf("%w: empty", ErrSchema)
	}
	return root, nil
}

type reader struct {
	b   []byte
	pos int
}

func Read(root *Node, data []byte) (map[string]any, error) {
	r := &reader{b: data}
	v, err := r.value(root)
	if err != nil {
		return nil, err
	}
	if r.pos != len(data) {
		return nil, fmt.Errorf("%w: read %d of %d bytes", ErrLayout, r.pos, len(data))
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: root is not a structure", ErrLayout)
	}
	return m, nil
}

func ReadPrefix(root *Node, data []byte, last string) (map[string]any, error) {
	r := &reader{b: data}
	m := map[string]any{}
	for _, c := range root.Children {
		v, err := r.value(c)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", root.Name, err)
		}
		m[c.Name] = v
		if c.Name == last {
			return m, nil
		}
	}
	return nil, fmt.Errorf("%w: no field %s", ErrSchema, last)
}

func (r *reader) need(n int) error {
	if n < 0 || len(r.b)-r.pos < n {
		return ErrTruncated
	}
	return nil
}

func (r *reader) align() {
	r.pos += (4 - r.pos%4) % 4
	r.pos = min(r.pos, len(r.b))
}

func (r *reader) count() (int, error) {
	if err := r.need(4); err != nil {
		return 0, err
	}
	n := int(int32(binary.LittleEndian.Uint32(r.b[r.pos:])))
	r.pos += 4
	if n < 0 || n > len(r.b)-r.pos {
		return 0, fmt.Errorf("%w: length %d at %d", ErrLayout, n, r.pos-4)
	}
	return n, nil
}

func (r *reader) value(n *Node) (any, error) {
	v, err := r.raw(n)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", n.Name, err)
	}
	if n.Align {
		r.align()
	}
	return v, nil
}

func (r *reader) raw(n *Node) (any, error) {
	elem, isArray := n.arrayElem()
	if n.Type == "string" && (!isArray || elem.Type == "char") {
		return r.str()
	}
	if isArray {
		return r.array(n, elem)
	}
	if n.Type == "TypelessData" {
		size, err := r.count()
		if err != nil {
			return nil, err
		}
		b := r.b[r.pos : r.pos+size]
		r.pos += size
		return b, nil
	}
	if size, ok := primSize[n.Type]; ok {
		if err := r.need(size); err != nil {
			return nil, err
		}
		v := prim(n.Type, r.b[r.pos:r.pos+size])
		r.pos += size
		return v, nil
	}
	if len(n.Children) == 0 {
		return nil, fmt.Errorf("%w: unknown type %s", ErrSchema, n.Type)
	}
	m := make(map[string]any, len(n.Children))
	for _, c := range n.Children {
		v, err := r.value(c)
		if err != nil {
			return nil, err
		}
		m[c.Name] = v
	}
	return m, nil
}

func (r *reader) str() (string, error) {
	n, err := r.count()
	if err != nil {
		return "", err
	}
	s := string(r.b[r.pos : r.pos+n])
	r.pos += n
	r.align()
	return s, nil
}

func (r *reader) array(n, elem *Node) (any, error) {
	count, err := r.count()
	if err != nil {
		return nil, err
	}
	var out any
	if elem.Type == "UInt8" || elem.Type == "char" || elem.Type == "SInt8" {
		out = r.b[r.pos : r.pos+count]
		r.pos += count
	} else {
		items := make([]any, 0, min(count, 1<<16))
		for range count {
			v, err := r.value(elem)
			if err != nil {
				return nil, err
			}
			items = append(items, v)
		}
		out = items
	}
	if n.Children[0].Align {
		r.align()
	}
	return out, nil
}

var primSize = map[string]int{
	"bool": 1, "char": 1, "SInt8": 1, "UInt8": 1,
	"SInt16": 2, "UInt16": 2, "short": 2, "unsigned short": 2,
	"int": 4, "SInt32": 4, "UInt32": 4, "unsigned int": 4, "float": 4, "Type*": 4,
	"SInt64": 8, "UInt64": 8, "long long": 8, "unsigned long long": 8, "FileSize": 8, "double": 8,
}

func prim(typ string, b []byte) any {
	switch typ {
	case "bool":
		return b[0] != 0
	case "char", "SInt8":
		return int64(int8(b[0]))
	case "UInt8":
		return int64(b[0])
	case "SInt16", "short":
		return int64(int16(binary.LittleEndian.Uint16(b)))
	case "UInt16", "unsigned short":
		return int64(binary.LittleEndian.Uint16(b))
	case "int", "SInt32", "Type*":
		return int64(int32(binary.LittleEndian.Uint32(b)))
	case "UInt32", "unsigned int":
		return int64(binary.LittleEndian.Uint32(b))
	case "float":
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))
	case "double":
		return math.Float64frombits(binary.LittleEndian.Uint64(b))
	default:
		return int64(binary.LittleEndian.Uint64(b))
	}
}
