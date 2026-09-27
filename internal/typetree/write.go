package typetree

import (
	"encoding/binary"
	"fmt"
	"math"
)

type writer struct {
	b []byte
}

func Write(root *Node, value map[string]any) ([]byte, error) {
	w := &writer{}
	if err := w.value(root, value); err != nil {
		return nil, err
	}
	return w.b, nil
}

func (w *writer) align() {
	for len(w.b)%4 != 0 {
		w.b = append(w.b, 0)
	}
}

func (w *writer) count(n int) {
	w.b = binary.LittleEndian.AppendUint32(w.b, uint32(int32(n)))
}

func (w *writer) value(n *Node, v any) error {
	if err := w.raw(n, v); err != nil {
		return fmt.Errorf("%s: %w", n.Name, err)
	}
	if n.Align {
		w.align()
	}
	return nil
}

func (w *writer) raw(n *Node, v any) error {
	elem, isArray := n.arrayElem()
	switch {
	case n.Type == "string" && (!isArray || elem.Type == "char"):
		s, _ := v.(string)
		w.count(len(s))
		w.b = append(w.b, s...)
		w.align()
		return nil
	case isArray:
		return w.array(n, elem, v)
	case n.Type == "TypelessData":
		b, _ := v.([]byte)
		w.count(len(b))
		w.b = append(w.b, b...)
		return nil
	}
	if size, ok := primSize[n.Type]; ok {
		w.b = append(w.b, encodePrim(n.Type, size, v)...)
		return nil
	}
	if len(n.Children) == 0 {
		return fmt.Errorf("%w: unknown type %s", ErrSchema, n.Type)
	}
	m, _ := v.(map[string]any)
	for _, c := range n.Children {
		if err := w.value(c, m[c.Name]); err != nil {
			return err
		}
	}
	return nil
}

func (w *writer) array(n, elem *Node, v any) error {
	switch items := v.(type) {
	case []byte:
		w.count(len(items))
		w.b = append(w.b, items...)
	case []any:
		w.count(len(items))
		for _, it := range items {
			if err := w.value(elem, it); err != nil {
				return err
			}
		}
	case []map[string]any:
		w.count(len(items))
		for _, it := range items {
			if err := w.value(elem, it); err != nil {
				return err
			}
		}
	case []string:
		w.count(len(items))
		for _, it := range items {
			if err := w.value(elem, it); err != nil {
				return err
			}
		}
	default:
		w.count(0)
	}
	if n.Children[0].Align {
		w.align()
	}
	return nil
}

func encodePrim(typ string, size int, v any) []byte {
	out := make([]byte, size)
	var i int64
	var f float64
	switch x := v.(type) {
	case int:
		i, f = int64(x), float64(x)
	case int64:
		i, f = x, float64(x)
	case float64:
		i, f = int64(x), x
	case bool:
		if x {
			i, f = 1, 1
		}
	}
	switch typ {
	case "float":
		binary.LittleEndian.PutUint32(out, math.Float32bits(float32(f)))
	case "double":
		binary.LittleEndian.PutUint64(out, math.Float64bits(f))
	default:
		switch size {
		case 1:
			out[0] = byte(i)
		case 2:
			binary.LittleEndian.PutUint16(out, uint16(i))
		case 4:
			binary.LittleEndian.PutUint32(out, uint32(i))
		default:
			binary.LittleEndian.PutUint64(out, uint64(i))
		}
	}
	return out
}
